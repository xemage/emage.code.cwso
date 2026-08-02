//! Async Parquet trajectory store (rollout-architecture-v1.md §5, T134).

use std::fs::{self, File};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use anyhow::{Context, Result};
use arrow::array::{
    Array, ArrayRef, Float64Array, Float64Builder, ListArray, ListBuilder, StringArray,
    UInt32Array, UInt32Builder, UInt64Array,
};
use arrow::datatypes::{DataType, Field, Schema};
use arrow::record_batch::RecordBatch;
use chrono::{DateTime, NaiveDate, Utc};
use crossbeam_channel::{Receiver, Sender, TrySendError};
use parquet::arrow::arrow_reader::ParquetRecordBatchReaderBuilder;
use parquet::arrow::ArrowWriter;
use parquet::basic::Compression;
use parquet::file::properties::WriterProperties;

use crate::record::CompletionRecord;

const STORE_QUEUE_DEFAULT: usize = 8192;
const FLUSH_BATCH_DEFAULT: usize = 64;
const FLUSH_INTERVAL_MS_DEFAULT: u64 = 1_000;

#[derive(Debug, Clone)]
pub struct StoreConfig {
    pub store_path: PathBuf,
    pub session_id: String,
    pub retention_days: u32,
    pub queue_capacity: usize,
    pub batch_size: usize,
    pub flush_interval: Duration,
}

impl StoreConfig {
    pub fn from_env() -> Result<Option<Self>> {
        if !env_bool("CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED", false) {
            return Ok(None);
        }

        // T170 fix for root-cause-analysis-cwso-rollout-v1.md Issue 2 (confirmed): the source
        // and its own architecture doc (rollout-architecture-v1.md:194) treat
        // CWSO_ROLLOUT_STORE_PATH as canonical, but deploy/Dockerfile.rollout (and the known
        // external emage.code consumer) set CWSO_ROLLOUT_TRAJECTORY_STORE_PATH instead, which
        // this crate never read, so the writer silently fell back to "./rollout_store" and
        // failed to create it. Per T169's recommended option (b), check the
        // _TRAJECTORY_-prefixed name first for backward compatibility with the known external
        // consumer, then fall back to the canonical name, then the historical default — this
        // fixes the wiring without requiring any change to deploy/Dockerfile.rollout.
        let store_path = std::env::var("CWSO_ROLLOUT_TRAJECTORY_STORE_PATH")
            .or_else(|_| std::env::var("CWSO_ROLLOUT_STORE_PATH"))
            .unwrap_or_else(|_| "./rollout_store".to_string());
        let session_id = std::env::var("CWSO_ROLLOUT_DEFAULT_SESSION_ID")
            .unwrap_or_else(|_| "default".to_string());
        let retention_days = std::env::var("CWSO_ROLLOUT_STORE_RETENTION_DAYS")
            .ok()
            .and_then(|value| value.parse::<u32>().ok())
            .filter(|value| *value > 0)
            .unwrap_or(7);

        Ok(Some(Self {
            store_path: PathBuf::from(store_path),
            session_id,
            retention_days,
            queue_capacity: env_usize("CWSO_ROLLOUT_STORE_QUEUE_CAPACITY", STORE_QUEUE_DEFAULT),
            batch_size: env_usize("CWSO_ROLLOUT_STORE_BATCH_SIZE", FLUSH_BATCH_DEFAULT),
            flush_interval: Duration::from_millis(env_u64(
                "CWSO_ROLLOUT_STORE_FLUSH_INTERVAL_MS",
                FLUSH_INTERVAL_MS_DEFAULT,
            )),
        }))
    }
}

#[derive(Debug)]
pub struct StoreHandle {
    pub sender: Sender<CompletionRecord>,
    pub dropped: Arc<AtomicU64>,
    pub written: Arc<AtomicU64>,
}

pub fn spawn_store(config: StoreConfig) -> Result<(StoreHandle, JoinHandle<()>)> {
    let (sender, receiver) = crossbeam_channel::bounded(config.queue_capacity);
    let dropped = Arc::new(AtomicU64::new(0));
    let written = Arc::new(AtomicU64::new(0));
    let handle = StoreHandle {
        sender,
        dropped: Arc::clone(&dropped),
        written: Arc::clone(&written),
    };

    let writer_config = config.clone();
    let writer_dropped = Arc::clone(&dropped);
    let writer_written = Arc::clone(&written);
    let join = thread::Builder::new()
        .name("cwso-rollout-store".into())
        .spawn(move || {
            if let Err(error) = writer_loop(writer_config, receiver, writer_dropped, writer_written)
            {
                tracing::error!(error = %error, "trajectory store writer exited");
            }
        })
        .context("spawn trajectory store writer")?;

    Ok((handle, join))
}

fn writer_loop(
    config: StoreConfig,
    receiver: Receiver<CompletionRecord>,
    dropped: Arc<AtomicU64>,
    written: Arc<AtomicU64>,
) -> Result<()> {
    fs::create_dir_all(&config.store_path)
        .with_context(|| format!("create rollout store {:?}", config.store_path))?;
    purge_stale_partitions(&config)?;

    let mut batch = Vec::with_capacity(config.batch_size);
    loop {
        match receiver.recv_timeout(config.flush_interval) {
            Ok(record) => {
                batch.push(record);
                if batch.len() >= config.batch_size {
                    flush_batch(&config, &mut batch, &written)?;
                }
            }
            Err(crossbeam_channel::RecvTimeoutError::Timeout) => {
                if !batch.is_empty() {
                    flush_batch(&config, &mut batch, &written)?;
                }
            }
            Err(crossbeam_channel::RecvTimeoutError::Disconnected) => {
                if !batch.is_empty() {
                    flush_batch(&config, &mut batch, &written)?;
                }
                break;
            }
        }
    }

    let _ = dropped;
    Ok(())
}

fn flush_batch(
    config: &StoreConfig,
    batch: &mut Vec<CompletionRecord>,
    written: &AtomicU64,
) -> Result<()> {
    if batch.is_empty() {
        return Ok(());
    }

    let partition_date = partition_date_for_batch(batch);
    let mut records = read_existing_records(config, partition_date)?;
    records.append(batch);
    write_records(config, partition_date, &records)?;
    written.fetch_add(batch.len() as u64, Ordering::Relaxed);
    batch.clear();
    Ok(())
}

fn partition_date_for_batch(batch: &[CompletionRecord]) -> NaiveDate {
    let timestamp_ns = batch
        .first()
        .map(|record| record.timestamp_ns)
        .unwrap_or_else(|| Utc::now().timestamp_nanos_opt().unwrap_or(0) as u64);
    timestamp_to_date(timestamp_ns)
}

fn timestamp_to_date(timestamp_ns: u64) -> NaiveDate {
    let secs = (timestamp_ns / 1_000_000_000) as i64;
    let nanos = (timestamp_ns % 1_000_000_000) as u32;
    DateTime::<Utc>::from_timestamp(secs, nanos)
        .map(|value| value.date_naive())
        .unwrap_or_else(|| Utc::now().date_naive())
}

fn partition_dir(config: &StoreConfig, date: NaiveDate) -> PathBuf {
    config.store_path.join(date.format("%Y-%m-%d").to_string())
}

fn partition_file(config: &StoreConfig, date: NaiveDate) -> PathBuf {
    partition_dir(config, date).join(format!("{}.parquet.lz4", config.session_id))
}

fn read_existing_records(config: &StoreConfig, date: NaiveDate) -> Result<Vec<CompletionRecord>> {
    let path = partition_file(config, date);
    if !path.exists() {
        return Ok(Vec::new());
    }

    let file = File::open(&path).with_context(|| format!("open parquet store {path:?}"))?;
    let builder = ParquetRecordBatchReaderBuilder::try_new(file)?;
    let mut records = Vec::new();
    for batch in builder.build()? {
        records.extend(decode_batch(&batch?)?);
    }
    Ok(records)
}

fn write_records(
    config: &StoreConfig,
    date: NaiveDate,
    records: &[CompletionRecord],
) -> Result<()> {
    let dir = partition_dir(config, date);
    fs::create_dir_all(&dir).with_context(|| format!("create partition {dir:?}"))?;
    let path = partition_file(config, date);
    let temp_path = path.with_extension("parquet.lz4.tmp");

    let schema = completion_schema();
    let batch = encode_batch(&schema, records, &config.session_id)?;
    let file =
        File::create(&temp_path).with_context(|| format!("create temp store {temp_path:?}"))?;
    let props = WriterProperties::builder()
        .set_compression(Compression::LZ4)
        .build();
    let mut writer = ArrowWriter::try_new(file, Arc::new(schema), Some(props))?;
    writer.write(&batch)?;
    writer.close()?;
    fs::rename(&temp_path, &path).with_context(|| format!("commit store file {path:?}"))?;
    Ok(())
}

fn completion_schema() -> Schema {
    Schema::new(vec![
        Field::new("request_id", DataType::Utf8, false),
        Field::new("provider", DataType::Utf8, false),
        Field::new(
            "prompt_token_ids",
            DataType::List(Arc::new(Field::new("item", DataType::UInt32, true))),
            false,
        ),
        Field::new(
            "sampled_token_ids",
            DataType::List(Arc::new(Field::new("item", DataType::UInt32, true))),
            false,
        ),
        Field::new(
            "logprobs",
            DataType::List(Arc::new(Field::new("item", DataType::Float64, true))),
            false,
        ),
        Field::new("finish_reason", DataType::Utf8, true),
        Field::new("timestamp_ns", DataType::UInt64, false),
        Field::new("session_id", DataType::Utf8, false),
    ])
}

fn encode_batch(
    schema: &Schema,
    records: &[CompletionRecord],
    session_id: &str,
) -> Result<RecordBatch> {
    let request_ids: Vec<&str> = records
        .iter()
        .map(|record| record.request_id.as_str())
        .collect();
    let providers: Vec<&str> = records
        .iter()
        .map(|record| record.provider.as_str())
        .collect();

    let mut prompt_builder = ListBuilder::new(UInt32Builder::new());
    let mut sampled_builder = ListBuilder::new(UInt32Builder::new());
    let mut logprob_builder = ListBuilder::new(Float64Builder::new());
    for record in records {
        for token in &record.prompt_token_ids {
            prompt_builder.values().append_value(*token);
        }
        prompt_builder.append(true);

        for token in &record.sampled_token_ids {
            sampled_builder.values().append_value(*token);
        }
        sampled_builder.append(true);

        for logprob in &record.logprobs {
            logprob_builder.values().append_value(*logprob);
        }
        logprob_builder.append(true);
    }

    let finish_reasons: Vec<Option<&str>> = records
        .iter()
        .map(|record| record.finish_reason.as_deref())
        .collect();
    let timestamps: Vec<u64> = records.iter().map(|record| record.timestamp_ns).collect();
    let session_ids: Vec<&str> = std::iter::repeat(session_id).take(records.len()).collect();

    RecordBatch::try_new(
        Arc::new(schema.clone()),
        vec![
            Arc::new(StringArray::from(request_ids)) as ArrayRef,
            Arc::new(StringArray::from(providers)) as ArrayRef,
            Arc::new(prompt_builder.finish()) as ArrayRef,
            Arc::new(sampled_builder.finish()) as ArrayRef,
            Arc::new(logprob_builder.finish()) as ArrayRef,
            Arc::new(StringArray::from(finish_reasons)) as ArrayRef,
            Arc::new(UInt64Array::from(timestamps)) as ArrayRef,
            Arc::new(StringArray::from(session_ids)) as ArrayRef,
        ],
    )
    .context("build parquet record batch")
}

fn decode_batch(batch: &RecordBatch) -> Result<Vec<CompletionRecord>> {
    use crate::provider::Provider;

    let request_ids = batch
        .column(0)
        .as_any()
        .downcast_ref::<StringArray>()
        .context("request_id column")?;
    let providers = batch
        .column(1)
        .as_any()
        .downcast_ref::<StringArray>()
        .context("provider column")?;
    let prompt_lists = batch
        .column(2)
        .as_any()
        .downcast_ref::<ListArray>()
        .context("prompt_token_ids column")?;
    let sampled_lists = batch
        .column(3)
        .as_any()
        .downcast_ref::<ListArray>()
        .context("sampled_token_ids column")?;
    let logprob_lists = batch
        .column(4)
        .as_any()
        .downcast_ref::<ListArray>()
        .context("logprobs column")?;
    let finish_reasons = batch
        .column(5)
        .as_any()
        .downcast_ref::<StringArray>()
        .context("finish_reason column")?;
    let timestamps = batch
        .column(6)
        .as_any()
        .downcast_ref::<UInt64Array>()
        .context("timestamp_ns column")?;

    let mut records = Vec::with_capacity(batch.num_rows());
    for row in 0..batch.num_rows() {
        let provider = Provider::from_wire_str(providers.value(row));
        records.push(CompletionRecord {
            request_id: request_ids.value(row).to_string(),
            provider,
            prompt_token_ids: list_u32_values(prompt_lists, row),
            sampled_token_ids: list_u32_values(sampled_lists, row),
            logprobs: list_f64_values(logprob_lists, row),
            finish_reason: if finish_reasons.is_null(row) {
                None
            } else {
                Some(finish_reasons.value(row).to_string())
            },
            timestamp_ns: timestamps.value(row),
        });
    }
    Ok(records)
}

fn list_u32_values(list: &ListArray, row: usize) -> Vec<u32> {
    let values = list.value(row);
    values
        .as_any()
        .downcast_ref::<UInt32Array>()
        .map(|array| array.values().to_vec())
        .unwrap_or_default()
}

fn list_f64_values(list: &ListArray, row: usize) -> Vec<f64> {
    let values = list.value(row);
    values
        .as_any()
        .downcast_ref::<Float64Array>()
        .map(|array| array.values().to_vec())
        .unwrap_or_default()
}

fn purge_stale_partitions(config: &StoreConfig) -> Result<()> {
    let cutoff = Utc::now().date_naive() - chrono::Duration::days(config.retention_days as i64);
    if !config.store_path.exists() {
        return Ok(());
    }
    for entry in fs::read_dir(&config.store_path)? {
        let entry = entry?;
        let file_type = entry.file_type()?;
        if !file_type.is_dir() {
            continue;
        }
        let name = entry.file_name();
        let Some(name) = name.to_str() else {
            continue;
        };
        let Ok(date) = NaiveDate::parse_from_str(name, "%Y-%m-%d") else {
            continue;
        };
        if date < cutoff {
            fs::remove_dir_all(entry.path())
                .with_context(|| format!("purge stale rollout partition {name}"))?;
        }
    }
    Ok(())
}

pub fn try_fanout_enqueue(store: &StoreHandle, record: &CompletionRecord) -> bool {
    match store.sender.try_send(record.clone()) {
        Ok(()) => true,
        Err(TrySendError::Full(_)) => {
            store.dropped.fetch_add(1, Ordering::Relaxed);
            false
        }
        Err(TrySendError::Disconnected(_)) => false,
    }
}

fn env_bool(key: &str, default: bool) -> bool {
    std::env::var(key)
        .ok()
        .map(|value| {
            matches!(
                value.trim().to_ascii_lowercase().as_str(),
                "1" | "true" | "yes" | "on"
            )
        })
        .unwrap_or(default)
}

fn env_u64(key: &str, default: u64) -> u64 {
    std::env::var(key)
        .ok()
        .and_then(|value| value.parse::<u64>().ok())
        .filter(|value| *value > 0)
        .unwrap_or(default)
}

fn env_usize(key: &str, default: usize) -> usize {
    std::env::var(key)
        .ok()
        .and_then(|value| value.parse::<usize>().ok())
        .filter(|value| *value > 0)
        .unwrap_or(default)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::provider::Provider;
    use std::time::Instant;

    fn sample_record(id: &str, timestamp_ns: u64) -> CompletionRecord {
        CompletionRecord {
            request_id: id.to_string(),
            provider: Provider::OpenAiChat,
            prompt_token_ids: vec![1, 2],
            sampled_token_ids: vec![3, 4],
            logprobs: vec![-0.1, -0.2],
            finish_reason: Some("stop".to_string()),
            timestamp_ns,
        }
    }

    #[test]
    fn parquet_round_trip_preserves_records() {
        let temp = tempfile::tempdir().expect("tempdir");
        let config = StoreConfig {
            store_path: temp.path().to_path_buf(),
            session_id: "sess-a".to_string(),
            retention_days: 7,
            queue_capacity: 8,
            batch_size: 4,
            flush_interval: Duration::from_millis(50),
        };
        let date = timestamp_to_date(1_700_000_000_000_000_000);
        let records = vec![
            sample_record("r1", 1_700_000_000_000_000_000),
            sample_record("r2", 1_700_000_000_100_000_000),
        ];
        write_records(&config, date, &records).expect("write");

        let decoded = read_existing_records(&config, date).expect("read");
        assert_eq!(decoded, records);
    }

    #[test]
    fn writer_thread_flushes_batches_without_blocking_proxy() {
        let temp = tempfile::tempdir().expect("tempdir");
        let config = StoreConfig {
            store_path: temp.path().to_path_buf(),
            session_id: "sess-b".to_string(),
            retention_days: 7,
            queue_capacity: 16,
            batch_size: 2,
            flush_interval: Duration::from_millis(100),
        };
        let (handle, join) = spawn_store(config.clone()).expect("spawn");
        for idx in 0..4 {
            assert!(try_fanout_enqueue(
                &handle,
                &sample_record(&format!("req-{idx}"), 1_700_000_000_200_000_000)
            ));
        }
        drop(handle.sender);
        join.join().expect("join");

        let date = timestamp_to_date(1_700_000_000_200_000_000);
        let decoded = read_existing_records(&config, date).expect("read");
        assert_eq!(decoded.len(), 4);
    }

    #[test]
    fn saturated_store_queue_increments_drop_counter() {
        let temp = tempfile::tempdir().expect("tempdir");
        let config = StoreConfig {
            store_path: temp.path().to_path_buf(),
            session_id: "sess-c".to_string(),
            retention_days: 7,
            queue_capacity: 1,
            batch_size: 64,
            flush_interval: Duration::from_secs(60),
        };
        let (handle, join) = spawn_store(config).expect("spawn");
        let record = sample_record("drop-me", 1_700_000_000_300_000_000);
        assert!(try_fanout_enqueue(&handle, &record));
        assert!(!try_fanout_enqueue(&handle, &record));
        assert_eq!(handle.dropped.load(Ordering::Relaxed), 1);
        drop(handle.sender);
        join.join().expect("join");
    }

    #[test]
    fn retention_purges_stale_partitions() {
        let temp = tempfile::tempdir().expect("tempdir");
        let stale = temp.path().join("2000-01-01");
        fs::create_dir_all(&stale).expect("mkdir");
        fs::write(stale.join("old.parquet.lz4"), b"stale").expect("write");

        let config = StoreConfig {
            store_path: temp.path().to_path_buf(),
            session_id: "sess-d".to_string(),
            retention_days: 7,
            queue_capacity: 8,
            batch_size: 4,
            flush_interval: Duration::from_millis(50),
        };
        purge_stale_partitions(&config).expect("purge");
        assert!(!stale.exists());
    }

    #[test]
    fn fanout_enqueue_is_fast() {
        let temp = tempfile::tempdir().expect("tempdir");
        let config = StoreConfig {
            store_path: temp.path().to_path_buf(),
            session_id: "sess-e".to_string(),
            retention_days: 7,
            queue_capacity: 1024,
            batch_size: 256,
            flush_interval: Duration::from_secs(30),
        };
        let (handle, join) = spawn_store(config).expect("spawn");
        let record = sample_record("fast", 1_700_000_000_400_000_000);
        let start = Instant::now();
        for _ in 0..512 {
            let _ = try_fanout_enqueue(&handle, &record);
        }
        assert!(start.elapsed() < Duration::from_millis(50));
        drop(handle.sender);
        join.join().expect("join");
    }

    /// T170 regression test for root-cause-analysis-cwso-rollout-v1.md Issue 2: verifies the
    /// env-var precedence chosen for `StoreConfig::from_env` — `CWSO_ROLLOUT_TRAJECTORY_STORE_PATH`
    /// (the name set by deploy/Dockerfile.rollout and the known external emage.code consumer)
    /// takes priority, falling back to the canonical `CWSO_ROLLOUT_STORE_PATH`, then the
    /// historical `./rollout_store` default.
    #[test]
    fn from_env_prefers_trajectory_alias_then_canonical_then_default() {
        // Serialize against other tests/processes touching these env vars; std::env is
        // process-global, and this crate's other tests never set these keys.
        static ENV_LOCK: std::sync::Mutex<()> = std::sync::Mutex::new(());
        let _guard = ENV_LOCK.lock().unwrap_or_else(|poison| poison.into_inner());

        let keys = [
            "CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED",
            "CWSO_ROLLOUT_TRAJECTORY_STORE_PATH",
            "CWSO_ROLLOUT_STORE_PATH",
        ];
        let saved: Vec<(&str, Option<String>)> = keys
            .iter()
            .map(|key| (*key, std::env::var(key).ok()))
            .collect();

        unsafe {
            std::env::set_var("CWSO_ROLLOUT_TRAJECTORY_STORE_ENABLED", "true");
            std::env::remove_var("CWSO_ROLLOUT_TRAJECTORY_STORE_PATH");
            std::env::remove_var("CWSO_ROLLOUT_STORE_PATH");
        }
        let default_config = StoreConfig::from_env()
            .expect("from_env default")
            .expect("store enabled");
        assert_eq!(default_config.store_path, PathBuf::from("./rollout_store"));

        unsafe {
            std::env::set_var("CWSO_ROLLOUT_STORE_PATH", "/tmp/canonical-only");
        }
        let canonical_only = StoreConfig::from_env()
            .expect("from_env canonical")
            .expect("store enabled");
        assert_eq!(
            canonical_only.store_path,
            PathBuf::from("/tmp/canonical-only")
        );

        unsafe {
            std::env::set_var("CWSO_ROLLOUT_TRAJECTORY_STORE_PATH", "/data/parquet-store");
        }
        let both_set = StoreConfig::from_env()
            .expect("from_env both set")
            .expect("store enabled");
        assert_eq!(
            both_set.store_path,
            PathBuf::from("/data/parquet-store"),
            "the _TRAJECTORY_-suffixed alias must win when both env vars are set"
        );

        unsafe {
            for (key, value) in saved {
                match value {
                    Some(value) => std::env::set_var(key, value),
                    None => std::env::remove_var(key),
                }
            }
        }
    }
}

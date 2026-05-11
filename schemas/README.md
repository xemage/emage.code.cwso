# Shared MCP Tool Schemas

JSON Schema definitions for CWSO MCP tools. Schemas are duplicated inline in
the Go and Rust code for runtime validation; this directory is the canonical
human-authored source of truth used during reviews.

| File | Tool | Phase |
|------|------|-------|
| `read_file_sync.json` | `read_file_sync` | 1 |
| `write_file_sync.json` | `write_file_sync` | 1 |
| `list_dir.json` | `list_dir` | 1 |
| `query_ast.json` | `query_ast` | 2 |
| `create_shadow_workspace.json` | `create_shadow_workspace` | 2 |
| `dispatch_concurrent_jobs.json` | `dispatch_concurrent_jobs` | 3 |
| `merge_concurrent_results.json` | `merge_concurrent_results` | 4 |

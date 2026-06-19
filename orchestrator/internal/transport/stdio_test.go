package transport

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestReadLineBasic(t *testing.T) {
	r := bufio.NewReader(bytes.NewBufferString("first line\n"))
	line, err := readLine(r)
	if err != nil {
		t.Fatalf("readLine basic error: %v", err)
	}
	if string(line) != "first line" {
		t.Fatalf("unexpected line %q", string(line))
	}
}

func TestReadLineHandlesLongSplitInput(t *testing.T) {
	long := bytes.Repeat([]byte("a"), 100000)
	buf := bytes.NewBuffer(nil)
	buf.Write(long)
	buf.WriteByte('\n')

	r := bufio.NewReaderSize(buf, 64)
	line, err := readLine(r)
	if err != nil {
		t.Fatalf("readLine long input error: %v", err)
	}
	if len(line) != len(long) {
		t.Fatalf("expected %d bytes, got %d", len(long), len(line))
	}
}

func TestReadLineRejectsOverLimit(t *testing.T) {
	tooLong := bytes.Repeat([]byte("z"), (8<<20)+1)
	buf := bytes.NewBuffer(nil)
	buf.Write(tooLong)
	buf.WriteByte('\n')

	r := bufio.NewReaderSize(buf, 1024)
	_, err := readLine(r)
	if err == nil {
		t.Fatal("expected limit error")
	}
	if err.Error() != "input line exceeds 8MiB limit" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadLineEOF(t *testing.T) {
	r := bufio.NewReader(bytes.NewBuffer(nil))
	_, err := readLine(r)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF for empty reader, got %v", err)
	}
}

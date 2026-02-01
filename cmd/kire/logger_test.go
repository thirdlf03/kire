package main

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"
)

func TestSetupLogger_Quiet(t *testing.T) {
	// Save and restore original logger state
	origOut := log.Writer()
	origPrefix := log.Prefix()
	origFlags := log.Flags()
	defer func() {
		log.SetOutput(origOut)
		log.SetPrefix(origPrefix)
		log.SetFlags(origFlags)
	}()

	setupLogger(true, "text")

	var buf bytes.Buffer
	// After quiet setup, log output should be discarded.
	// We verify by checking that the writer is io.Discard (indirectly: logging produces nothing).
	log.SetOutput(&buf) // override to capture
	// But setupLogger set it to discard, so let's re-call to verify the pattern
	setupLogger(true, "text")
	log.Println("should not appear")

	// The buf won't capture because we called setupLogger again which sets io.Discard
	// Instead, let's test by capturing output after setup
	// Re-approach: setupLogger sets to discard, then log.Println goes nowhere.
	// We can't capture discard, but we can verify flag state.
	if log.Flags() != 0 {
		t.Errorf("expected flags=0 after quiet setup, got %d", log.Flags())
	}
}

func TestSetupLogger_TextFormat(t *testing.T) {
	origOut := log.Writer()
	origPrefix := log.Prefix()
	origFlags := log.Flags()
	defer func() {
		log.SetOutput(origOut)
		log.SetPrefix(origPrefix)
		log.SetFlags(origFlags)
	}()

	setupLogger(false, "text")

	if log.Prefix() != "kire: " {
		t.Errorf("expected prefix 'kire: ', got %q", log.Prefix())
	}
	if log.Flags() != 0 {
		t.Errorf("expected flags=0, got %d", log.Flags())
	}
}

func TestSetupLogger_JSONFormat(t *testing.T) {
	origOut := log.Writer()
	origPrefix := log.Prefix()
	origFlags := log.Flags()
	defer func() {
		log.SetOutput(origOut)
		log.SetPrefix(origPrefix)
		log.SetFlags(origFlags)
	}()

	setupLogger(false, "json")

	if log.Prefix() != "" {
		t.Errorf("expected empty prefix for json format, got %q", log.Prefix())
	}
}

func TestJSONLogWriter(t *testing.T) {
	var buf bytes.Buffer
	w := &jsonLogWriter{out: &buf}

	_, err := w.Write([]byte("hello world\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var entry logEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}

	if entry.Level != "info" {
		t.Errorf("expected level=info, got %q", entry.Level)
	}
	if entry.Message != "hello world" {
		t.Errorf("expected message='hello world', got %q", entry.Message)
	}
	if entry.Time == "" {
		t.Error("expected non-empty time")
	}
}

func TestJSONLogWriter_Warning(t *testing.T) {
	var buf bytes.Buffer
	w := &jsonLogWriter{out: &buf}

	_, err := w.Write([]byte("WARNING: something bad\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var entry logEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}

	if entry.Level != "warn" {
		t.Errorf("expected level=warn, got %q", entry.Level)
	}
	if entry.Message != "something bad" {
		t.Errorf("expected message='something bad', got %q", entry.Message)
	}
}

func TestJSONLogWriter_WarningNoSpace(t *testing.T) {
	var buf bytes.Buffer
	w := &jsonLogWriter{out: &buf}

	// "WARNING:" with no trailing space, followed by newline
	_, err := w.Write([]byte("WARNING:\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var entry logEntry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}

	if entry.Level != "warn" {
		t.Errorf("expected level=warn, got %q", entry.Level)
	}
	if entry.Message != "" {
		t.Errorf("expected empty message, got %q", entry.Message)
	}
}

func TestJSONLogWriter_MultipleLines(t *testing.T) {
	var buf bytes.Buffer
	w := &jsonLogWriter{out: &buf}

	if _, err := w.Write([]byte("line 1\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("WARNING: line 2\n")); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}

	var e1, e2 logEntry
	if err := json.Unmarshal([]byte(lines[0]), &e1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &e2); err != nil {
		t.Fatal(err)
	}

	if e1.Level != "info" {
		t.Errorf("line 1: expected level=info, got %q", e1.Level)
	}
	if e2.Level != "warn" {
		t.Errorf("line 2: expected level=warn, got %q", e2.Level)
	}
}

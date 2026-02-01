package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// setupLogger configures the global logger based on --quiet and --log-format flags.
func setupLogger(quiet bool, logFormat string) {
	if quiet {
		log.SetOutput(io.Discard)
		log.SetFlags(0)
		return
	}

	switch logFormat {
	case "json":
		log.SetOutput(&jsonLogWriter{out: os.Stderr})
		log.SetPrefix("")
		log.SetFlags(0)
	default: // "text"
		log.SetPrefix("kire: ")
		log.SetFlags(0)
	}
}

// jsonLogWriter wraps an io.Writer to emit each log line as a JSON object.
type jsonLogWriter struct {
	out io.Writer
}

type logEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

func (w *jsonLogWriter) Write(p []byte) (int, error) {
	msg := string(p)
	// Trim trailing newline (log package appends one)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	level := "info"
	// Detect WARNING prefix
	if strings.HasPrefix(msg, "WARNING:") {
		level = "warn"
		msg = strings.TrimSpace(msg[8:])
	}

	entry := logEntry{
		Time:    time.Now().UTC().Format(time.RFC3339),
		Level:   level,
		Message: msg,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return 0, err
	}
	data = append(data, '\n')
	return w.out.Write(data)
}

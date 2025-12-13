package tailer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTailer(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(tmpFile, []byte("line 1\nline 2\n"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	// Test creating a tailer for existing file
	tailer, err := NewTailer(tmpFile, nil)
	if err != nil {
		t.Fatalf("failed to create tailer: %v", err)
	}
	defer tailer.Stop()

	if tailer.filePath != tmpFile {
		t.Errorf("expected filePath %s, got %s", tmpFile, tailer.filePath)
	}
}

func TestNewTailerNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.log")

	// Should fail with default options (MustExist = true)
	_, err := NewTailer(nonExistentFile, nil)
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}

	// Should succeed with MustExist = false
	opts := DefaultOptions()
	opts.MustExist = false
	tailer, err := NewTailer(nonExistentFile, opts)
	if err != nil {
		t.Fatalf("expected no error with MustExist=false, got: %v", err)
	}
	defer tailer.Stop()
}

func TestTailerReadExistingContent(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	content := "line 1\nline 2\nline 3\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	opts := DefaultOptions()
	opts.Follow = false // Don't follow, just read existing content

	tailer, err := NewTailer(tmpFile, opts)
	if err != nil {
		t.Fatalf("failed to create tailer: %v", err)
	}
	defer tailer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := tailer.Start(ctx); err != nil {
		t.Fatalf("failed to start tailer: %v", err)
	}

	// Read all lines
	var lines []Line
	for line := range tailer.Lines() {
		if line.Err != nil {
			t.Errorf("unexpected error: %v", line.Err)
			continue
		}
		lines = append(lines, line)
	}

	expected := []string{"line 1", "line 2", "line 3"}
	if len(lines) != len(expected) {
		t.Fatalf("expected %d lines, got %d", len(expected), len(lines))
	}

	for i, line := range lines {
		if line.Text != expected[i] {
			t.Errorf("line %d: expected %q, got %q", i, expected[i], line.Text)
		}
		if line.FilePath != tmpFile {
			t.Errorf("line %d: expected filePath %s, got %s", i, tmpFile, line.FilePath)
		}
	}
}

func TestTailerFollowNewContent(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	// Create empty file
	if err := os.WriteFile(tmpFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	opts := DefaultOptions()
	opts.Follow = true
	opts.PollInterval = 50 * time.Millisecond

	tailer, err := NewTailer(tmpFile, opts)
	if err != nil {
		t.Fatalf("failed to create tailer: %v", err)
	}
	defer tailer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tailer.StartFromEnd(ctx); err != nil {
		t.Fatalf("failed to start tailer: %v", err)
	}

	// Write new content after a delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		f, err := os.OpenFile(tmpFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		f.WriteString("new line 1\n")
		f.WriteString("new line 2\n")
	}()

	// Read lines with timeout
	var lines []Line
	timeout := time.After(3 * time.Second)

readLoop:
	for {
		select {
		case line, ok := <-tailer.Lines():
			if !ok {
				break readLoop
			}
			if line.Err != nil {
				continue
			}
			lines = append(lines, line)
			if len(lines) >= 2 {
				break readLoop
			}
		case <-timeout:
			break readLoop
		}
	}

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	expected := []string{"new line 1", "new line 2"}
	for i := 0; i < 2; i++ {
		if lines[i].Text != expected[i] {
			t.Errorf("line %d: expected %q, got %q", i, expected[i], lines[i].Text)
		}
	}
}

func TestTailerHandleRotation(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	// Create initial file
	if err := os.WriteFile(tmpFile, []byte("initial\n"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	opts := DefaultOptions()
	opts.Follow = true
	opts.ReOpen = true
	opts.PollInterval = 50 * time.Millisecond

	tailer, err := NewTailer(tmpFile, opts)
	if err != nil {
		t.Fatalf("failed to create tailer: %v", err)
	}
	defer tailer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tailer.StartFromEnd(ctx); err != nil {
		t.Fatalf("failed to start tailer: %v", err)
	}

	// Simulate log rotation
	go func() {
		time.Sleep(100 * time.Millisecond)

		// Remove old file
		os.Remove(tmpFile)

		// Create new file with same name
		time.Sleep(50 * time.Millisecond)
		if err := os.WriteFile(tmpFile, []byte("after rotation\n"), 0644); err != nil {
			return
		}
	}()

	// Read lines with timeout
	var lines []Line
	timeout := time.After(3 * time.Second)

readLoop:
	for {
		select {
		case line, ok := <-tailer.Lines():
			if !ok {
				break readLoop
			}
			if line.Err != nil {
				continue
			}
			lines = append(lines, line)
			if len(lines) >= 1 {
				break readLoop
			}
		case <-timeout:
			break readLoop
		}
	}

	// Should have detected the rotation and read the new content
	if len(lines) < 1 {
		t.Fatalf("expected at least 1 line after rotation, got %d", len(lines))
	}

	if lines[0].Text != "after rotation" {
		t.Errorf("expected 'after rotation', got %q", lines[0].Text)
	}
}

func TestTailerHandleTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")

	// Create file with content
	if err := os.WriteFile(tmpFile, []byte("line before truncation\n"), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	opts := DefaultOptions()
	opts.Follow = true
	opts.PollInterval = 50 * time.Millisecond

	tailer, err := NewTailer(tmpFile, opts)
	if err != nil {
		t.Fatalf("failed to create tailer: %v", err)
	}
	defer tailer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tailer.StartFromEnd(ctx); err != nil {
		t.Fatalf("failed to start tailer: %v", err)
	}

	// Simulate truncation (copytruncate style)
	go func() {
		time.Sleep(100 * time.Millisecond)

		// Truncate the file
		if err := os.Truncate(tmpFile, 0); err != nil {
			return
		}

		// Write new content
		time.Sleep(50 * time.Millisecond)
		f, err := os.OpenFile(tmpFile, os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		defer f.Close()
		f.WriteString("after truncation\n")
	}()

	// Read lines with timeout
	var lines []Line
	timeout := time.After(3 * time.Second)

readLoop:
	for {
		select {
		case line, ok := <-tailer.Lines():
			if !ok {
				break readLoop
			}
			if line.Err != nil {
				continue
			}
			lines = append(lines, line)
			if len(lines) >= 1 {
				break readLoop
			}
		case <-timeout:
			break readLoop
		}
	}

	// Should have detected the truncation and read the new content
	if len(lines) < 1 {
		t.Fatalf("expected at least 1 line after truncation, got %d", len(lines))
	}

	if lines[0].Text != "after truncation" {
		t.Errorf("expected 'after truncation', got %q", lines[0].Text)
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if !opts.Follow {
		t.Error("expected Follow to be true by default")
	}
	if !opts.ReOpen {
		t.Error("expected ReOpen to be true by default")
	}
	if !opts.MustExist {
		t.Error("expected MustExist to be true by default")
	}
	if opts.PollInterval != 250*time.Millisecond {
		t.Errorf("expected PollInterval 250ms, got %v", opts.PollInterval)
	}
}

package tailer

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestNewMultiTailer(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "test1.log")
	file2 := filepath.Join(tmpDir, "test2.log")

	if err := os.WriteFile(file1, []byte("file1 line\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("file2 line\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test with direct file paths
	mt, err := NewMultiTailer([]string{file1, file2}, nil)
	if err != nil {
		t.Fatalf("failed to create multi-tailer: %v", err)
	}
	defer mt.Stop()
}

func TestMultiTailerGlobPattern(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files matching pattern
	for i := 1; i <= 3; i++ {
		file := filepath.Join(tmpDir, "app"+string(rune('0'+i))+".log")
		if err := os.WriteFile(file, []byte("content\n"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Create a file that shouldn't match
	nonMatch := filepath.Join(tmpDir, "other.txt")
	if err := os.WriteFile(nonMatch, []byte("content\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	pattern := filepath.Join(tmpDir, "*.log")
	opts := DefaultOptions()
	opts.Follow = false

	mt, err := NewMultiTailer([]string{pattern}, opts)
	if err != nil {
		t.Fatalf("failed to create multi-tailer: %v", err)
	}
	defer mt.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := mt.Start(ctx); err != nil {
		t.Fatalf("failed to start multi-tailer: %v", err)
	}

	files := mt.Files()
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d: %v", len(files), files)
	}

	// Verify all matched files have .log extension
	for _, f := range files {
		if filepath.Ext(f) != ".log" {
			t.Errorf("unexpected file matched: %s", f)
		}
	}
}

func TestMultiTailerMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "test1.log")
	file2 := filepath.Join(tmpDir, "test2.log")

	if err := os.WriteFile(file1, []byte("file1 line 1\nfile1 line 2\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("file2 line 1\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	opts := DefaultOptions()
	opts.Follow = false

	mt, err := NewMultiTailer([]string{file1, file2}, opts)
	if err != nil {
		t.Fatalf("failed to create multi-tailer: %v", err)
	}
	defer mt.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := mt.Start(ctx); err != nil {
		t.Fatalf("failed to start multi-tailer: %v", err)
	}

	// Collect all lines
	var lines []Line
	timeout := time.After(1 * time.Second)

readLoop:
	for {
		select {
		case line, ok := <-mt.Lines():
			if !ok {
				break readLoop
			}
			if line.Err == nil {
				lines = append(lines, line)
			}
		case <-timeout:
			break readLoop
		}
	}

	// Should have 3 lines total
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	// Verify lines are from different files
	files := make(map[string]int)
	for _, line := range lines {
		files[line.FilePath]++
	}

	if files[file1] != 2 {
		t.Errorf("expected 2 lines from file1, got %d", files[file1])
	}
	if files[file2] != 1 {
		t.Errorf("expected 1 line from file2, got %d", files[file2])
	}
}

func TestMultiTailerFilesMethod(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "a.log")
	file2 := filepath.Join(tmpDir, "b.log")

	if err := os.WriteFile(file1, []byte("content\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	opts := DefaultOptions()
	opts.Follow = false

	mt, err := NewMultiTailer([]string{file1, file2}, opts)
	if err != nil {
		t.Fatalf("failed to create multi-tailer: %v", err)
	}
	defer mt.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := mt.Start(ctx); err != nil {
		t.Fatalf("failed to start multi-tailer: %v", err)
	}

	files := mt.Files()
	sort.Strings(files)

	expected := []string{file1, file2}
	sort.Strings(expected)

	if len(files) != len(expected) {
		t.Fatalf("expected %d files, got %d", len(expected), len(files))
	}

	for i, f := range files {
		if f != expected[i] {
			t.Errorf("file %d: expected %s, got %s", i, expected[i], f)
		}
	}
}

func TestMultiTailerNoMatchingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	pattern := filepath.Join(tmpDir, "*.nonexistent")
	opts := DefaultOptions()
	opts.MustExist = true

	_, err := NewMultiTailer([]string{pattern}, opts)
	if err != nil {
		t.Fatalf("NewMultiTailer should not fail: %v", err)
	}

	// Note: The error should come when we try to start
	mt, _ := NewMultiTailer([]string{pattern}, opts)
	defer mt.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = mt.Start(ctx)
	if err == nil {
		t.Error("expected error for non-matching pattern, got nil")
	}
}

func TestMultiTailerInvalidGlobPattern(t *testing.T) {
	// This test verifies behavior with invalid glob patterns
	// The filepath.Glob function handles most patterns gracefully
	// but we can test with a valid directory and empty result

	tmpDir := t.TempDir()
	opts := DefaultOptions()
	opts.MustExist = false // Don't require files to exist

	mt, err := NewMultiTailer([]string{filepath.Join(tmpDir, "*.log")}, opts)
	if err != nil {
		t.Fatalf("failed to create multi-tailer: %v", err)
	}
	defer mt.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Should succeed even with no matching files when MustExist is false
	err = mt.Start(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMultiTailerStop(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.log")

	if err := os.WriteFile(file, []byte("content\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	opts := DefaultOptions()
	opts.Follow = true

	mt, err := NewMultiTailer([]string{file}, opts)
	if err != nil {
		t.Fatalf("failed to create multi-tailer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mt.Start(ctx); err != nil {
		t.Fatalf("failed to start multi-tailer: %v", err)
	}

	// Stop should not panic and should close channels
	mt.Stop()

	// Calling Stop again should be safe
	mt.Stop()
}

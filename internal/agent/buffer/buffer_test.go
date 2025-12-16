package buffer

import (
	"os"
	"path/filepath"
	"testing"

	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createTestEntry(msg string) *blazelogv1.LogEntry {
	return &blazelogv1.LogEntry{
		Timestamp: timestamppb.Now(),
		Level:     blazelogv1.LogLevel_LOG_LEVEL_INFO,
		Message:   msg,
		Source:    "test",
	}
}

func TestDiskBuffer_WriteRead(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:       dir,
		MaxSize:   10 * 1024 * 1024,
		SyncEvery: 1,
	}

	buf, err := NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer: %v", err)
	}
	defer buf.Close()

	// Write entries
	entries := []*blazelogv1.LogEntry{
		createTestEntry("entry 1"),
		createTestEntry("entry 2"),
		createTestEntry("entry 3"),
	}

	if err := buf.Write(entries); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if buf.Len() != 3 {
		t.Errorf("expected Len 3, got %d", buf.Len())
	}

	// Read entries
	read, err := buf.Read(2)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(read) != 2 {
		t.Errorf("expected 2 entries, got %d", len(read))
	}
	if read[0].Message != "entry 1" {
		t.Errorf("expected 'entry 1', got %s", read[0].Message)
	}
	if read[1].Message != "entry 2" {
		t.Errorf("expected 'entry 2', got %s", read[1].Message)
	}

	// Remaining should be 1
	if buf.Len() != 1 {
		t.Errorf("expected Len 1 after read, got %d", buf.Len())
	}

	// Read remaining
	read, err = buf.Read(10)
	if err != nil {
		t.Fatalf("Read remaining: %v", err)
	}

	if len(read) != 1 {
		t.Errorf("expected 1 entry, got %d", len(read))
	}
	if read[0].Message != "entry 3" {
		t.Errorf("expected 'entry 3', got %s", read[0].Message)
	}

	// Should be empty now
	if buf.Len() != 0 {
		t.Errorf("expected Len 0, got %d", buf.Len())
	}
}

func TestDiskBuffer_Persistence(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:       dir,
		MaxSize:   10 * 1024 * 1024,
		SyncEvery: 1,
	}

	// Write to buffer
	buf1, err := NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer: %v", err)
	}

	entries := []*blazelogv1.LogEntry{
		createTestEntry("persistent 1"),
		createTestEntry("persistent 2"),
	}
	if err := buf1.Write(entries); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buf1.Close()

	// Reopen and verify
	buf2, err := NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer reopen: %v", err)
	}
	defer buf2.Close()

	if buf2.Len() != 2 {
		t.Errorf("expected Len 2 after reopen, got %d", buf2.Len())
	}

	read, err := buf2.Read(2)
	if err != nil {
		t.Fatalf("Read after reopen: %v", err)
	}

	if len(read) != 2 {
		t.Errorf("expected 2 entries, got %d", len(read))
	}
	if read[0].Message != "persistent 1" {
		t.Errorf("expected 'persistent 1', got %s", read[0].Message)
	}
}

func TestDiskBuffer_MaxSize(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:       dir,
		MaxSize:   500, // Very small for testing
		SyncEvery: 1,
	}

	buf, err := NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer: %v", err)
	}
	defer buf.Close()

	// Write many entries - should drop oldest
	for i := 0; i < 20; i++ {
		entry := createTestEntry("overflow test entry")
		if err := buf.Write([]*blazelogv1.LogEntry{entry}); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	// Size should be under limit
	if buf.Size() > cfg.MaxSize {
		t.Errorf("size %d exceeds max %d", buf.Size(), cfg.MaxSize)
	}

	// Should have some entries (not all 20 due to overflow)
	if buf.Len() == 0 {
		t.Error("expected some entries after overflow handling")
	}
	if buf.Len() == 20 {
		t.Error("expected fewer than 20 entries due to size limit")
	}
}

func TestDiskBuffer_Clear(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:       dir,
		MaxSize:   10 * 1024 * 1024,
		SyncEvery: 1,
	}

	buf, err := NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer: %v", err)
	}
	defer buf.Close()

	// Write entries
	entries := []*blazelogv1.LogEntry{
		createTestEntry("clear 1"),
		createTestEntry("clear 2"),
	}
	if err := buf.Write(entries); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if buf.Len() != 2 {
		t.Errorf("expected Len 2, got %d", buf.Len())
	}

	// Clear
	if err := buf.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected Len 0 after clear, got %d", buf.Len())
	}
	if buf.Size() != 0 {
		t.Errorf("expected Size 0 after clear, got %d", buf.Size())
	}
}

func TestDiskBuffer_EmptyRead(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:       dir,
		MaxSize:   10 * 1024 * 1024,
		SyncEvery: 1,
	}

	buf, err := NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer: %v", err)
	}
	defer buf.Close()

	// Read from empty buffer
	read, err := buf.Read(10)
	if err != nil {
		t.Fatalf("Read empty: %v", err)
	}

	if len(read) != 0 {
		t.Errorf("expected nil or empty, got %d entries", len(read))
	}
}

func TestDiskBuffer_ClosedOperations(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Dir:       dir,
		MaxSize:   10 * 1024 * 1024,
		SyncEvery: 1,
	}

	buf, err := NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer: %v", err)
	}

	buf.Close()

	// Operations on closed buffer should fail
	err = buf.Write([]*blazelogv1.LogEntry{createTestEntry("test")})
	if err != ErrBufferClosed {
		t.Errorf("Write on closed: expected ErrBufferClosed, got %v", err)
	}

	_, err = buf.Read(1)
	if err != ErrBufferClosed {
		t.Errorf("Read on closed: expected ErrBufferClosed, got %v", err)
	}
}

func TestDiskBuffer_DirectoryCreation(t *testing.T) {
	// Non-existent nested directory
	dir := filepath.Join(t.TempDir(), "nested", "buffer", "dir")
	cfg := Config{
		Dir:       dir,
		MaxSize:   10 * 1024 * 1024,
		SyncEvery: 1,
	}

	buf, err := NewDiskBuffer(cfg)
	if err != nil {
		t.Fatalf("NewDiskBuffer with nested dir: %v", err)
	}
	defer buf.Close()

	// Verify directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("directory was not created")
	}
}

package buffer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	blazelogv1 "github.com/good-yellow-bee/blazelog/internal/proto/blazelog/v1"
	"google.golang.org/protobuf/proto"
)

var (
	ErrBufferFull   = errors.New("buffer is full")
	ErrBufferClosed = errors.New("buffer is closed")
)

// lenBufPool pools 4-byte length buffers to reduce allocations
// Uses *[]byte to satisfy SA6002 (sync.Pool argument should be pointer-like)
var lenBufPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 4)
		return &buf
	},
}

// Buffer interface for pluggable buffer implementations.
type Buffer interface {
	// Write appends entries to the buffer.
	Write(entries []*blazelogv1.LogEntry) error
	// Read returns up to n entries from the buffer, removing them.
	Read(n int) ([]*blazelogv1.LogEntry, error)
	// Len returns the number of buffered entries.
	Len() int
	// Size returns the current buffer size in bytes.
	Size() int64
	// Close flushes and closes the buffer.
	Close() error
}

// Config configures the disk buffer.
type Config struct {
	Dir              string  // Buffer directory
	MaxSize          int64   // Maximum buffer size in bytes (default: 100MB)
	SyncEvery        int     // Sync to disk after N writes (default: 100)
	CompactThreshold float64 // Compact when consumed ratio exceeds this (default: 0.5)
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		Dir:              filepath.Join(homeDir, ".blazelog", "buffer"),
		MaxSize:          100 * 1024 * 1024, // 100MB
		SyncEvery:        100,
		CompactThreshold: 0.5, // Compact when >50% consumed
	}
}

// DiskBuffer implements Buffer with file-based persistence.
// Format: [4 bytes length][protobuf data][4 bytes length][protobuf data]...
// Uses offset-based tracking to avoid O(n) compaction on every read.
type DiskBuffer struct {
	config     Config
	file       *os.File
	size       int64 // Total bytes in file
	readOffset int64 // Bytes consumed (logical start of unread data)
	count      int   // Number of unread entries
	writes     int   // Counter for sync

	mu     sync.Mutex
	closed bool
}

// NewDiskBuffer creates a new disk-backed buffer.
func NewDiskBuffer(cfg Config) (*DiskBuffer, error) {
	if cfg.Dir == "" {
		cfg = DefaultConfig()
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 100 * 1024 * 1024
	}
	if cfg.SyncEvery == 0 {
		cfg.SyncEvery = 100
	}
	if cfg.CompactThreshold == 0 {
		cfg.CompactThreshold = 0.5
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("create buffer dir: %w", err)
	}

	// Open or create buffer file
	bufferPath := filepath.Join(cfg.Dir, "buffer.dat")
	file, err := os.OpenFile(bufferPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open buffer file: %w", err)
	}

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat buffer file: %w", err)
	}

	b := &DiskBuffer{
		config: cfg,
		file:   file,
		size:   info.Size(),
	}

	// Count existing entries
	if b.size > 0 {
		count, err := b.countEntries()
		if err != nil {
			// Corrupted buffer, truncate
			file.Truncate(0)
			file.Seek(0, io.SeekStart)
			b.size = 0
		} else {
			b.count = count
		}
	}

	// Seek to end for appending
	file.Seek(0, io.SeekEnd)

	return b, nil
}

// Write appends entries to the buffer.
func (b *DiskBuffer) Write(entries []*blazelogv1.LogEntry) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrBufferClosed
	}

	// Get pooled length buffer
	lenBufPtr := lenBufPool.Get().(*[]byte)
	lenBuf := *lenBufPtr
	defer lenBufPool.Put(lenBufPtr)

	for _, entry := range entries {
		data, err := proto.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal entry: %w", err)
		}

		entrySize := int64(4 + len(data))

		// Check if buffer would exceed max size (accounting for consumed space)
		activeSize := b.size - b.readOffset
		if activeSize+entrySize > b.config.MaxSize {
			// Drop oldest entries to make room
			if err := b.dropOldest(entrySize); err != nil {
				return fmt.Errorf("drop oldest: %w", err)
			}
		}

		// Write length prefix
		binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
		if _, err := b.file.Write(lenBuf); err != nil {
			return fmt.Errorf("write length: %w", err)
		}

		// Write data
		if _, err := b.file.Write(data); err != nil {
			return fmt.Errorf("write data: %w", err)
		}

		b.size += entrySize
		b.count++
		b.writes++
	}

	// Sync periodically
	if b.writes >= b.config.SyncEvery {
		b.file.Sync()
		b.writes = 0
	}

	return nil
}

// Read returns up to n entries from the buffer, removing them.
// Uses offset-based tracking to avoid O(n) compaction on every read.
func (b *DiskBuffer) Read(n int) ([]*blazelogv1.LogEntry, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrBufferClosed
	}

	if b.count == 0 {
		return nil, nil
	}

	// Seek to read offset (start of unread data)
	if _, err := b.file.Seek(b.readOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	// Get pooled length buffer
	lenBufPtr := lenBufPool.Get().(*[]byte)
	lenBuf := *lenBufPtr
	defer lenBufPool.Put(lenBufPtr)

	// Read entries
	entries := make([]*blazelogv1.LogEntry, 0, n)
	bytesRead := int64(0)

	for i := 0; i < n && i < b.count; i++ {
		// Read length prefix
		if _, err := io.ReadFull(b.file, lenBuf); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read length: %w", err)
		}
		dataLen := binary.BigEndian.Uint32(lenBuf)
		bytesRead += 4

		// Read data
		data := make([]byte, dataLen)
		if _, err := io.ReadFull(b.file, data); err != nil {
			return nil, fmt.Errorf("read data: %w", err)
		}
		bytesRead += int64(dataLen)

		// Unmarshal
		entry := &blazelogv1.LogEntry{}
		if err := proto.Unmarshal(data, entry); err != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}

		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Update offset instead of compacting (O(1) instead of O(n))
	b.readOffset += bytesRead
	b.count -= len(entries)

	// Compact only when consumed ratio exceeds threshold
	if b.size > 0 && float64(b.readOffset)/float64(b.size) > b.config.CompactThreshold {
		if err := b.compactNow(); err != nil {
			return nil, fmt.Errorf("compact: %w", err)
		}
	}

	return entries, nil
}

// Len returns the number of buffered entries.
func (b *DiskBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

// Size returns the current buffer size in bytes (active data only).
func (b *DiskBuffer) Size() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size - b.readOffset
}

// Close flushes and closes the buffer.
func (b *DiskBuffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil
	}

	b.closed = true
	if err := b.file.Sync(); err != nil {
		b.file.Close()
		return err
	}
	return b.file.Close()
}

// countEntries counts entries in the buffer file.
func (b *DiskBuffer) countEntries() (int, error) {
	if _, err := b.file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	lenBufPtr := lenBufPool.Get().(*[]byte)
	lenBuf := *lenBufPtr
	defer lenBufPool.Put(lenBufPtr)

	count := 0
	for {
		if _, err := io.ReadFull(b.file, lenBuf); err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
		dataLen := binary.BigEndian.Uint32(lenBuf)

		// Skip data
		if _, err := b.file.Seek(int64(dataLen), io.SeekCurrent); err != nil {
			return 0, err
		}
		count++
	}

	return count, nil
}

// dropOldest removes oldest entries to make room for new data.
// Uses offset-based tracking for efficiency.
func (b *DiskBuffer) dropOldest(needed int64) error {
	// Seek to current read position (start of unread data)
	if _, err := b.file.Seek(b.readOffset, io.SeekStart); err != nil {
		return err
	}

	lenBufPtr := lenBufPool.Get().(*[]byte)
	lenBuf := *lenBufPtr
	defer lenBufPool.Put(lenBufPtr)

	// Find how many entries to drop
	bytesToDrop := int64(0)
	entriesToDrop := 0

	for bytesToDrop < needed && entriesToDrop < b.count {
		if _, err := io.ReadFull(b.file, lenBuf); err != nil {
			break
		}
		dataLen := binary.BigEndian.Uint32(lenBuf)
		bytesToDrop += 4 + int64(dataLen)
		entriesToDrop++

		if _, err := b.file.Seek(int64(dataLen), io.SeekCurrent); err != nil {
			break
		}
	}

	if entriesToDrop == 0 {
		// Seek back to end for appending since we moved the file position
		_, err := b.file.Seek(0, io.SeekEnd)
		return err
	}

	// Update offset instead of rewriting file
	b.readOffset += bytesToDrop
	b.count -= entriesToDrop

	// Compact if threshold exceeded
	if b.size > 0 && float64(b.readOffset)/float64(b.size) > b.config.CompactThreshold {
		return b.compactNow()
	}

	// Seek back to end for appending
	_, err := b.file.Seek(0, io.SeekEnd)
	return err
}

// compactNow removes consumed data from the file using streaming copy.
// Called only when consumed ratio exceeds threshold.
func (b *DiskBuffer) compactNow() error {
	// Calculate remaining data
	remaining := b.size - b.readOffset
	if remaining <= 0 {
		// No remaining data, truncate
		if err := b.file.Truncate(0); err != nil {
			return err
		}
		if _, err := b.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		b.size = 0
		b.readOffset = 0
		return nil
	}

	// Create temp file for streaming copy to avoid large memory allocation
	tempPath := b.file.Name() + ".tmp"
	tempFile, err := os.OpenFile(tempPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	// Seek to start of unread data
	if _, err := b.file.Seek(b.readOffset, io.SeekStart); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return err
	}

	// Stream copy in chunks to limit memory usage (64KB chunks)
	const chunkSize = 64 * 1024
	buf := make([]byte, chunkSize)
	copied := int64(0)
	for copied < remaining {
		toRead := remaining - copied
		if toRead > chunkSize {
			toRead = chunkSize
		}
		n, err := b.file.Read(buf[:toRead])
		if err != nil && err != io.EOF {
			tempFile.Close()
			os.Remove(tempPath)
			return fmt.Errorf("read during compact: %w", err)
		}
		if n == 0 {
			break
		}
		if _, err := tempFile.Write(buf[:n]); err != nil {
			tempFile.Close()
			os.Remove(tempPath)
			return fmt.Errorf("write during compact: %w", err)
		}
		copied += int64(n)
	}

	// Sync temp file
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("sync temp file: %w", err)
	}

	// Close original file
	origPath := b.file.Name()
	b.file.Close()

	// Close temp and rename
	tempFile.Close()
	if err := os.Rename(tempPath, origPath); err != nil {
		// Try to reopen original on failure
		reopenErr := error(nil)
		b.file, reopenErr = os.OpenFile(origPath, os.O_RDWR|os.O_CREATE, 0644)
		if reopenErr != nil {
			return fmt.Errorf("rename temp file: %w, reopen also failed: %w", err, reopenErr)
		}
		return fmt.Errorf("rename temp file: %w", err)
	}

	// Reopen the compacted file
	b.file, err = os.OpenFile(origPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("reopen after compact: %w", err)
	}

	// Seek to end for appending
	if _, err := b.file.Seek(0, io.SeekEnd); err != nil {
		return err
	}

	b.size = copied
	b.readOffset = 0
	return nil
}

// Clear removes all entries from the buffer.
func (b *DiskBuffer) Clear() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrBufferClosed
	}

	if err := b.file.Truncate(0); err != nil {
		return err
	}
	if _, err := b.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	b.size = 0
	b.readOffset = 0
	b.count = 0
	return nil
}

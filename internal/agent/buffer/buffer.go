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
	Dir       string // Buffer directory
	MaxSize   int64  // Maximum buffer size in bytes (default: 100MB)
	SyncEvery int    // Sync to disk after N writes (default: 100)
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		Dir:       filepath.Join(homeDir, ".blazelog", "buffer"),
		MaxSize:   100 * 1024 * 1024, // 100MB
		SyncEvery: 100,
	}
}

// DiskBuffer implements Buffer with file-based persistence.
// Format: [4 bytes length][protobuf data][4 bytes length][protobuf data]...
type DiskBuffer struct {
	config Config
	file   *os.File
	size   int64
	count  int
	writes int // Counter for sync

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

	for _, entry := range entries {
		data, err := proto.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal entry: %w", err)
		}

		entrySize := int64(4 + len(data))

		// Check if buffer would exceed max size
		if b.size+entrySize > b.config.MaxSize {
			// Drop oldest entries to make room
			if err := b.dropOldest(entrySize); err != nil {
				return fmt.Errorf("drop oldest: %w", err)
			}
		}

		// Write length prefix
		lenBuf := make([]byte, 4)
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
func (b *DiskBuffer) Read(n int) ([]*blazelogv1.LogEntry, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrBufferClosed
	}

	if b.count == 0 {
		return nil, nil
	}

	// Seek to start
	if _, err := b.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	// Read entries
	entries := make([]*blazelogv1.LogEntry, 0, n)
	bytesRead := int64(0)

	for i := 0; i < n && i < b.count; i++ {
		// Read length prefix
		lenBuf := make([]byte, 4)
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

	// Remove read entries by rewriting remaining data
	if err := b.compactAfterRead(bytesRead, len(entries)); err != nil {
		return nil, fmt.Errorf("compact: %w", err)
	}

	return entries, nil
}

// Len returns the number of buffered entries.
func (b *DiskBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.count
}

// Size returns the current buffer size in bytes.
func (b *DiskBuffer) Size() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size
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

	count := 0
	for {
		lenBuf := make([]byte, 4)
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
func (b *DiskBuffer) dropOldest(needed int64) error {
	// Read all remaining entries
	if _, err := b.file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Find how many entries to drop
	bytesToDrop := int64(0)
	entriesToDrop := 0

	for bytesToDrop < needed && entriesToDrop < b.count {
		lenBuf := make([]byte, 4)
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
		return nil
	}

	return b.compactAfterRead(bytesToDrop, entriesToDrop)
}

// compactAfterRead removes the first bytesRead bytes from the file.
func (b *DiskBuffer) compactAfterRead(bytesRead int64, entriesRemoved int) error {
	// Read remaining data
	remaining := b.size - bytesRead
	if remaining <= 0 {
		// No remaining data, truncate
		if err := b.file.Truncate(0); err != nil {
			return err
		}
		if _, err := b.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		b.size = 0
		b.count = 0
		return nil
	}

	// Read remaining bytes
	if _, err := b.file.Seek(bytesRead, io.SeekStart); err != nil {
		return err
	}
	remainingData := make([]byte, remaining)
	if _, err := io.ReadFull(b.file, remainingData); err != nil {
		return err
	}

	// Truncate and rewrite
	if err := b.file.Truncate(0); err != nil {
		return err
	}
	if _, err := b.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := b.file.Write(remainingData); err != nil {
		return err
	}

	b.size = remaining
	b.count -= entriesRemoved
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
	b.count = 0
	return nil
}

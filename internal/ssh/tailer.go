package ssh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// Line represents a single line read from a remote file.
type Line struct {
	Text     string    // The line content
	FilePath string    // The source file path
	Host     string    // The remote host
	Time     time.Time // When the line was read
	Err      error     // Any error that occurred
}

// TailerConfig holds configuration for the remote tailer.
type TailerConfig struct {
	// Follow indicates whether to continue watching for new lines.
	Follow bool
	// PollInterval is the interval to check for file changes.
	PollInterval time.Duration
	// ReOpen indicates whether to reopen the file if it's rotated.
	ReOpen bool
	// FromEnd starts tailing from the end of the file.
	FromEnd bool
}

// DefaultTailerConfig returns TailerConfig with sensible defaults.
func DefaultTailerConfig() *TailerConfig {
	return &TailerConfig{
		Follow:       true,
		PollInterval: 500 * time.Millisecond,
		ReOpen:       true,
		FromEnd:      false,
	}
}

// Tailer watches a remote file via SSH and emits new lines.
type Tailer struct {
	client   *Client
	filePath string
	config   *TailerConfig

	lines   chan Line
	done    chan struct{}
	running bool
	mu      sync.Mutex

	// State tracking for rotation detection
	lastInode int64
	lastSize  int64
	offset    int64
}

// NewTailer creates a new remote file tailer.
func NewTailer(client *Client, filePath string, config *TailerConfig) *Tailer {
	if config == nil {
		config = DefaultTailerConfig()
	}

	return &Tailer{
		client:   client,
		filePath: filePath,
		config:   config,
		lines:    make(chan Line, 100),
		done:     make(chan struct{}),
	}
}

// Lines returns a channel that emits lines as they're read.
func (t *Tailer) Lines() <-chan Line {
	return t.lines
}

// Start begins tailing the remote file.
func (t *Tailer) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return fmt.Errorf("tailer already running")
	}
	t.running = true
	t.mu.Unlock()

	// Get initial file info
	info, err := t.client.FileInfo(ctx, t.filePath)
	if err != nil {
		return fmt.Errorf("get file info: %w", err)
	}
	t.lastInode = info.Inode
	t.lastSize = info.Size

	// Determine starting offset
	if t.config.FromEnd {
		t.offset = info.Size
	} else {
		t.offset = 0
	}

	go t.run(ctx)
	return nil
}

// Stop stops the tailer.
func (t *Tailer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return
	}
	t.running = false
	close(t.done)
}

func (t *Tailer) run(ctx context.Context) {
	defer close(t.lines)

	// Read initial content if not starting from end
	if !t.config.FromEnd && t.lastSize > 0 {
		t.readFromOffset(ctx)
	}

	if !t.config.Follow {
		return
	}

	ticker := time.NewTicker(t.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.done:
			return
		case <-ticker.C:
			t.poll(ctx)
		}
	}
}

func (t *Tailer) poll(ctx context.Context) {
	info, err := t.client.FileInfo(ctx, t.filePath)
	if err != nil {
		t.sendLine(&Line{
			FilePath: t.filePath,
			Host:     t.client.config.Host,
			Time:     time.Now(),
			Err:      fmt.Errorf("get file info: %w", err),
		})
		return
	}

	// Check for file rotation (inode change)
	if info.Inode != t.lastInode {
		t.handleRotation(ctx, info)
		return
	}

	// Check for truncation
	if info.Size < t.offset {
		t.handleTruncation(ctx, info)
		return
	}

	// Check for new content
	if info.Size > t.offset {
		t.lastSize = info.Size
		t.readFromOffset(ctx)
	}
}

func (t *Tailer) handleRotation(ctx context.Context, info *RemoteFileInfo) {
	if !t.config.ReOpen {
		return
	}

	t.lastInode = info.Inode
	t.lastSize = info.Size
	t.offset = 0
	t.readFromOffset(ctx)
}

func (t *Tailer) handleTruncation(ctx context.Context, info *RemoteFileInfo) {
	t.lastSize = info.Size
	t.offset = 0
	t.readFromOffset(ctx)
}

func (t *Tailer) readFromOffset(ctx context.Context) {
	// Calculate bytes to read
	bytesToRead := t.lastSize - t.offset
	if bytesToRead <= 0 {
		return
	}

	// Read chunk from file
	data, err := t.client.ReadFileRange(ctx, t.filePath, t.offset, bytesToRead)
	if err != nil {
		t.sendLine(&Line{
			FilePath: t.filePath,
			Host:     t.client.config.Host,
			Time:     time.Now(),
			Err:      fmt.Errorf("read file: %w", err),
		})
		return
	}

	// Parse and emit lines
	t.parseLines(data)
	t.offset += int64(len(data))
}

func (t *Tailer) parseLines(data []byte) {
	scanner := bufio.NewScanner(&bytesReader{data: data})
	for scanner.Scan() {
		t.sendLine(&Line{
			Text:     scanner.Text(),
			FilePath: t.filePath,
			Host:     t.client.config.Host,
			Time:     time.Now(),
		})
	}
}

func (t *Tailer) sendLine(line *Line) {
	select {
	case t.lines <- *line:
	case <-t.done:
	}
}

// bytesReader implements io.Reader for a byte slice.
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// StreamTailer uses SSH streaming for more efficient tailing.
type StreamTailer struct {
	client   *Client
	filePath string
	config   *TailerConfig

	lines   chan Line
	done    chan struct{}
	running bool
	mu      sync.Mutex
	stream  io.ReadCloser
}

// NewStreamTailer creates a tailer that uses SSH streaming.
func NewStreamTailer(client *Client, filePath string, config *TailerConfig) *StreamTailer {
	if config == nil {
		config = DefaultTailerConfig()
	}

	return &StreamTailer{
		client:   client,
		filePath: filePath,
		config:   config,
		lines:    make(chan Line, 100),
		done:     make(chan struct{}),
	}
}

// Lines returns a channel that emits lines as they're read.
func (t *StreamTailer) Lines() <-chan Line {
	return t.lines
}

// Start begins streaming the remote file.
func (t *StreamTailer) Start(ctx context.Context) error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return fmt.Errorf("tailer already running")
	}
	t.running = true
	t.mu.Unlock()

	// Determine starting offset
	var offset int64
	if t.config.FromEnd {
		info, err := t.client.FileInfo(ctx, t.filePath)
		if err != nil {
			return fmt.Errorf("get file info: %w", err)
		}
		offset = info.Size
	}

	// Open stream
	stream, err := t.client.StreamFile(ctx, t.filePath, offset)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	t.stream = stream

	go t.run(ctx)
	return nil
}

// Stop stops the stream tailer.
func (t *StreamTailer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return
	}
	t.running = false
	close(t.done)

	if t.stream != nil {
		t.stream.Close()
	}
}

func (t *StreamTailer) run(ctx context.Context) {
	defer close(t.lines)

	scanner := bufio.NewScanner(t.stream)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		case <-t.done:
			return
		default:
			t.lines <- Line{
				Text:     scanner.Text(),
				FilePath: t.filePath,
				Host:     t.client.config.Host,
				Time:     time.Now(),
			}
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-t.done:
		default:
			t.lines <- Line{
				FilePath: t.filePath,
				Host:     t.client.config.Host,
				Time:     time.Now(),
				Err:      fmt.Errorf("stream error: %w", err),
			}
		}
	}
}

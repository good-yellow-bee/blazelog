package logs

import (
	"fmt"
	"net/http"
)

// SSEWriter provides Server-Sent Events writing capabilities.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter creates a new SSE writer.
func NewSSEWriter(w http.ResponseWriter, flusher http.Flusher) *SSEWriter {
	return &SSEWriter{
		w:       w,
		flusher: flusher,
	}
}

// SendEvent sends an SSE event with the given event type and data.
// Format: event: <type>\ndata: <data>\n\n
func (s *SSEWriter) SendEvent(event, data string) error {
	_, err := fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, data)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// SendData sends data without an event type (uses default "message" event).
// Format: data: <data>\n\n
func (s *SSEWriter) SendData(data string) error {
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// SendComment sends a comment (ignored by clients, useful for keepalive).
// Format: : <comment>\n\n
func (s *SSEWriter) SendComment(comment string) error {
	_, err := fmt.Fprintf(s.w, ": %s\n\n", comment)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// SendRetry tells the client to wait the specified milliseconds before reconnecting.
// Format: retry: <ms>\n\n
func (s *SSEWriter) SendRetry(milliseconds int) error {
	_, err := fmt.Fprintf(s.w, "retry: %d\n\n", milliseconds)
	if err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

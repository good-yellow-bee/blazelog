package notifier

import (
	"bufio"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/alerting"
	"github.com/good-yellow-bee/blazelog/internal/models"
)

func TestEmailConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  EmailConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty config",
			config:  EmailConfig{},
			wantErr: true,
			errMsg:  "SMTP host is required",
		},
		{
			name: "missing port",
			config: EmailConfig{
				Host: "smtp.example.com",
			},
			wantErr: true,
			errMsg:  "SMTP port is required",
		},
		{
			name: "missing from",
			config: EmailConfig{
				Host: "smtp.example.com",
				Port: 587,
			},
			wantErr: true,
			errMsg:  "from address is required",
		},
		{
			name: "missing recipients",
			config: EmailConfig{
				Host: "smtp.example.com",
				Port: 587,
				From: "test@example.com",
			},
			wantErr: true,
			errMsg:  "at least one recipient is required",
		},
		{
			name: "valid config",
			config: EmailConfig{
				Host:       "smtp.example.com",
				Port:       587,
				From:       "test@example.com",
				Recipients: []string{"admin@example.com"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadTemplates(t *testing.T) {
	templates, err := LoadTemplates()
	if err != nil {
		t.Fatalf("failed to load templates: %v", err)
	}

	if templates.html == nil {
		t.Error("HTML template is nil")
	}
	if templates.plain == nil {
		t.Error("plain template is nil")
	}
}

func TestTemplatesRender(t *testing.T) {
	templates, err := LoadTemplates()
	if err != nil {
		t.Fatalf("failed to load templates: %v", err)
	}

	data := TemplateData{
		RuleName:      "Test Rule",
		Description:   "Test description",
		Severity:      "critical",
		SeverityColor: "#d32f2f",
		Message:       "Test message",
		Timestamp:     "2024-01-15 10:30:00 UTC",
		Count:         5,
		Threshold:     3,
		Window:        "5m",
		TriggeringEntry: &LogEntryData{
			Timestamp: "2024-01-15 10:30:00",
			Level:     "error",
			Message:   "Test log message",
			FilePath:  "/var/log/test.log",
		},
		Labels: map[string]string{
			"environment": "production",
		},
	}

	// Test HTML rendering
	html, err := templates.RenderHTML(&data)
	if err != nil {
		t.Fatalf("failed to render HTML: %v", err)
	}
	if !strings.Contains(html, "Test Rule") {
		t.Error("HTML missing rule name")
	}
	if !strings.Contains(html, "critical") {
		t.Error("HTML missing severity")
	}
	if !strings.Contains(html, "Test message") {
		t.Error("HTML missing message")
	}

	// Test plain rendering
	plain, err := templates.RenderPlain(&data)
	if err != nil {
		t.Fatalf("failed to render plain: %v", err)
	}
	if !strings.Contains(plain, "Test Rule") {
		t.Error("plain missing rule name")
	}
	if !strings.Contains(plain, "CRITICAL") {
		t.Error("plain missing severity (should be uppercased)")
	}
}

func TestAlertToTemplateData(t *testing.T) {
	now := time.Now()
	alert := &alerting.Alert{
		RuleName:    "High Error Rate",
		Description: "More than 100 errors in 5 minutes",
		Severity:    alerting.SeverityCritical,
		Message:     "Threshold exceeded: 150 events in 5m (threshold: 100)",
		Timestamp:   now,
		Count:       150,
		Threshold:   100,
		Window:      "5m",
		TriggeringEntry: &models.LogEntry{
			Timestamp: now,
			Level:     models.LevelError,
			Message:   "Database connection failed",
			FilePath:  "/var/log/app/error.log",
		},
		Labels: map[string]string{
			"project": "ecommerce",
		},
	}

	data := AlertToTemplateData(alert)

	if data.RuleName != alert.RuleName {
		t.Errorf("RuleName mismatch: got %q, want %q", data.RuleName, alert.RuleName)
	}
	if data.Severity != "critical" {
		t.Errorf("Severity mismatch: got %q, want %q", data.Severity, "critical")
	}
	if data.SeverityColor != "#d32f2f" {
		t.Errorf("SeverityColor mismatch: got %q, want %q", data.SeverityColor, "#d32f2f")
	}
	if data.Count != 150 {
		t.Errorf("Count mismatch: got %d, want %d", data.Count, 150)
	}
	if data.TriggeringEntry == nil {
		t.Error("TriggeringEntry is nil")
	}
	if data.TriggeringEntry.Level != "error" {
		t.Errorf("TriggeringEntry.Level mismatch: got %q, want %q", data.TriggeringEntry.Level, "error")
	}
}

func TestSeverityColor(t *testing.T) {
	tests := []struct {
		severity alerting.Severity
		want     string
	}{
		{alerting.SeverityCritical, "#d32f2f"},
		{alerting.SeverityHigh, "#f57c00"},
		{alerting.SeverityMedium, "#fbc02d"},
		{alerting.SeverityLow, "#388e3c"},
		{alerting.Severity("unknown"), "#757575"},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := severityColor(tt.severity)
			if got != tt.want {
				t.Errorf("severityColor(%q) = %q, want %q", tt.severity, got, tt.want)
			}
		})
	}
}

func TestEmailNotifierName(t *testing.T) {
	notifier := &EmailNotifier{}
	if got := notifier.Name(); got != "email" {
		t.Errorf("Name() = %q, want %q", got, "email")
	}
}

func TestDispatcher(t *testing.T) {
	dispatcher := NewDispatcher()

	// Create a mock notifier
	mock := &mockNotifier{name: "test", sent: make(chan *alerting.Alert, 1)}
	dispatcher.Register(mock)

	// Verify registration
	n, ok := dispatcher.Get("test")
	if !ok {
		t.Fatal("notifier not found after registration")
	}
	if n.Name() != "test" {
		t.Errorf("notifier name = %q, want %q", n.Name(), "test")
	}

	// Test dispatch
	alert := &alerting.Alert{
		RuleName: "Test",
		Notify:   []string{"test"},
	}

	if err := dispatcher.Dispatch(context.Background(), alert); err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	select {
	case received := <-mock.sent:
		if received.RuleName != "Test" {
			t.Errorf("received alert rule name = %q, want %q", received.RuleName, "Test")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for alert")
	}

	// Test unregister
	dispatcher.Unregister("test")
	if _, ok := dispatcher.Get("test"); ok {
		t.Error("notifier still exists after unregister")
	}

	// Test close
	if err := dispatcher.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestDispatcherDispatchAll(t *testing.T) {
	dispatcher := NewDispatcher()

	mock1 := &mockNotifier{name: "mock1", sent: make(chan *alerting.Alert, 1)}
	mock2 := &mockNotifier{name: "mock2", sent: make(chan *alerting.Alert, 1)}
	dispatcher.Register(mock1)
	dispatcher.Register(mock2)

	alert := &alerting.Alert{
		RuleName: "Test",
		Notify:   []string{}, // empty, should still dispatch to all with DispatchAll
	}

	if err := dispatcher.DispatchAll(context.Background(), alert); err != nil {
		t.Fatalf("DispatchAll failed: %v", err)
	}

	// Both should receive
	for _, mock := range []*mockNotifier{mock1, mock2} {
		select {
		case <-mock.sent:
		case <-time.After(time.Second):
			t.Errorf("timeout waiting for alert on %s", mock.name)
		}
	}
}

func TestBuildMIMEMessage(t *testing.T) {
	notifier := &EmailNotifier{
		config: EmailConfig{
			From:       "BlazeLog <alerts@example.com>",
			Recipients: []string{"admin@example.com", "ops@example.com"},
		},
	}

	msg := notifier.buildMIMEMessage("Test Subject", "Plain body", "<html>HTML body</html>")
	msgStr := string(msg)

	// Check headers
	if !strings.Contains(msgStr, "From: BlazeLog <alerts@example.com>") {
		t.Error("message missing From header")
	}
	if !strings.Contains(msgStr, "To: admin@example.com, ops@example.com") {
		t.Error("message missing To header")
	}
	if !strings.Contains(msgStr, "Subject: Test Subject") {
		t.Error("message missing Subject header")
	}
	if !strings.Contains(msgStr, "MIME-Version: 1.0") {
		t.Error("message missing MIME-Version header")
	}
	if !strings.Contains(msgStr, "multipart/alternative") {
		t.Error("message missing multipart/alternative content type")
	}

	// Check bodies
	if !strings.Contains(msgStr, "Plain body") {
		t.Error("message missing plain text body")
	}
	if !strings.Contains(msgStr, "<html>HTML body</html>") {
		t.Error("message missing HTML body")
	}
}

func TestExtractEmail(t *testing.T) {
	notifier := &EmailNotifier{}

	tests := []struct {
		input string
		want  string
	}{
		{"test@example.com", "test@example.com"},
		{"Test User <test@example.com>", "test@example.com"},
		{"BlazeLog Alerts <alerts@example.com>", "alerts@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := notifier.extractEmail(tt.input)
			if got != tt.want {
				t.Errorf("extractEmail(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// mockNotifier is a test notifier that records sent alerts.
type mockNotifier struct {
	name string
	sent chan *alerting.Alert
}

func (m *mockNotifier) Name() string {
	return m.name
}

func (m *mockNotifier) Send(ctx context.Context, alert *alerting.Alert) error {
	m.sent <- alert
	return nil
}

func (m *mockNotifier) Close() error {
	return nil
}

// mockSMTPServer creates a mock SMTP server for testing.
type mockSMTPServer struct {
	listener net.Listener
	messages [][]byte
	mu       sync.Mutex
	wg       sync.WaitGroup
}

func newMockSMTPServer(t *testing.T) *mockSMTPServer {
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	server := &mockSMTPServer{
		listener: listener,
		messages: make([][]byte, 0),
	}

	server.wg.Add(1)
	go server.serve(t)

	return server
}

func (s *mockSMTPServer) serve(t *testing.T) {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConnection(conn, t)
	}
}

func (s *mockSMTPServer) handleConnection(conn net.Conn, t *testing.T) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Send greeting
	writer.WriteString("220 localhost SMTP Mock Server\r\n")
	writer.Flush()

	var dataMode bool
	var messageData []byte

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)

		if dataMode {
			if line == "." {
				dataMode = false
				s.mu.Lock()
				s.messages = append(s.messages, messageData)
				s.mu.Unlock()
				messageData = nil
				writer.WriteString("250 OK\r\n")
				writer.Flush()
				continue
			}
			messageData = append(messageData, []byte(line+"\n")...)
			continue
		}

		upperLine := strings.ToUpper(line)

		switch {
		case strings.HasPrefix(upperLine, "EHLO"), strings.HasPrefix(upperLine, "HELO"):
			writer.WriteString("250-localhost\r\n")
			writer.WriteString("250 OK\r\n")
			writer.Flush()
		case strings.HasPrefix(upperLine, "MAIL FROM"):
			writer.WriteString("250 OK\r\n")
			writer.Flush()
		case strings.HasPrefix(upperLine, "RCPT TO"):
			writer.WriteString("250 OK\r\n")
			writer.Flush()
		case upperLine == "DATA":
			writer.WriteString("354 Start mail input\r\n")
			writer.Flush()
			dataMode = true
		case upperLine == "QUIT":
			writer.WriteString("221 Bye\r\n")
			writer.Flush()
			return
		default:
			writer.WriteString("500 Unknown command\r\n")
			writer.Flush()
		}
	}
}

func (s *mockSMTPServer) addr() string {
	return s.listener.Addr().String()
}

func (s *mockSMTPServer) close() {
	s.listener.Close()
	s.wg.Wait()
}

func (s *mockSMTPServer) getMessages() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([][]byte, len(s.messages))
	copy(result, s.messages)
	return result
}

func TestEmailNotifierSendWithMockSMTP(t *testing.T) {
	server := newMockSMTPServer(t)
	defer server.close()

	// Parse address
	host, portStr, _ := net.SplitHostPort(server.addr())
	var port int
	// Manual parse since port is numeric
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	config := EmailConfig{
		Host:       host,
		Port:       port,
		From:       "test@example.com",
		Recipients: []string{"admin@example.com"},
	}

	notifier, err := NewEmailNotifier(&config)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	alert := &alerting.Alert{
		RuleName:    "Test Alert",
		Description: "Test description",
		Severity:    alerting.SeverityHigh,
		Message:     "Test message",
		Timestamp:   time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := notifier.Send(ctx, alert); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Wait a bit for message to be processed
	time.Sleep(100 * time.Millisecond)

	messages := server.getMessages()
	if len(messages) == 0 {
		t.Fatal("no messages received by mock server")
	}

	msgStr := string(messages[0])
	if !strings.Contains(msgStr, "Test Alert") {
		t.Error("message doesn't contain alert name")
	}
}

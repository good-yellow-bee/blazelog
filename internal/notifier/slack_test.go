package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/alerting"
	"github.com/good-yellow-bee/blazelog/internal/models"
)

func TestSlackConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  SlackConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty config",
			config:  SlackConfig{},
			wantErr: true,
			errMsg:  "webhook URL is required",
		},
		{
			name: "http URL rejected",
			config: SlackConfig{
				WebhookURL: "http://hooks.slack.com/services/xxx",
			},
			wantErr: true,
			errMsg:  "webhook URL must use HTTPS",
		},
		{
			name: "valid config",
			config: SlackConfig{
				WebhookURL: "https://hooks.slack.com/services/T00/B00/xxx",
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

func TestSlackNotifierName(t *testing.T) {
	notifier := &SlackNotifier{}
	if got := notifier.Name(); got != "slack" {
		t.Errorf("Name() = %q, want %q", got, "slack")
	}
}

func TestSlackNotifierClose(t *testing.T) {
	notifier := &SlackNotifier{}
	if err := notifier.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestSlackNotifierSend(t *testing.T) {
	var receivedPayload slackMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}

		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Errorf("failed to unmarshal payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	// Use test server URL (allow non-HTTPS for testing)
	notifier := &SlackNotifier{
		config: SlackConfig{
			WebhookURL: server.URL,
		},
		httpClient: server.Client(),
	}

	alert := &alerting.Alert{
		RuleName:    "Test Alert",
		Description: "Test description",
		Severity:    alerting.SeverityCritical,
		Message:     "Test message",
		Timestamp:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Count:       5,
		Threshold:   3,
		Window:      "5m",
	}

	ctx := context.Background()
	if err := notifier.Send(ctx, alert); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify payload structure
	if len(receivedPayload.Blocks) == 0 {
		t.Fatal("expected blocks in payload")
	}

	// Check header block
	header := receivedPayload.Blocks[0]
	if header.Type != "header" {
		t.Errorf("first block type = %q, want %q", header.Type, "header")
	}
	if header.Text == nil {
		t.Fatal("header text is nil")
	}
	if !strings.Contains(header.Text.Text, "Test Alert") {
		t.Errorf("header missing rule name, got %q", header.Text.Text)
	}
}

func TestSlackNotifierSendWithTriggeringEntry(t *testing.T) {
	var receivedPayload slackMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := &SlackNotifier{
		config:     SlackConfig{WebhookURL: server.URL},
		httpClient: server.Client(),
	}

	alert := &alerting.Alert{
		RuleName:    "Pattern Alert",
		Description: "Detected pattern match",
		Severity:    alerting.SeverityHigh,
		Message:     "FATAL error detected",
		Timestamp:   time.Now(),
		TriggeringEntry: &models.LogEntry{
			Timestamp: time.Now(),
			Level:     models.LevelError,
			Message:   "Database connection failed",
			FilePath:  "/var/log/app/error.log",
		},
		Labels: map[string]string{
			"environment": "production",
			"project":     "ecommerce",
		},
	}

	ctx := context.Background()
	if err := notifier.Send(ctx, alert); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Should have header, severity/time, message, triggering entry, description context, labels context
	if len(receivedPayload.Blocks) < 4 {
		t.Errorf("expected at least 4 blocks, got %d", len(receivedPayload.Blocks))
	}

	// Look for triggering entry info
	found := false
	for _, block := range receivedPayload.Blocks {
		if block.Text != nil && strings.Contains(block.Text.Text, "Database connection failed") {
			found = true
			break
		}
	}
	if !found {
		t.Error("triggering entry message not found in payload")
	}

	// Look for labels
	foundLabels := false
	for _, block := range receivedPayload.Blocks {
		if block.Type == "context" && len(block.Elements) > 0 {
			for _, elem := range block.Elements {
				if strings.Contains(elem.Text, "environment=production") {
					foundLabels = true
					break
				}
			}
		}
	}
	if !foundLabels {
		t.Error("labels not found in payload")
	}
}

func TestSlackNotifierHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	notifier := &SlackNotifier{
		config:     SlackConfig{WebhookURL: server.URL},
		httpClient: server.Client(),
	}

	alert := &alerting.Alert{
		RuleName:  "Test",
		Severity:  alerting.SeverityLow,
		Timestamp: time.Now(),
	}

	err := notifier.Send(context.Background(), alert)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("error should contain status code, got %q", err.Error())
	}
}

func TestSlackNotifierContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := &SlackNotifier{
		config:     SlackConfig{WebhookURL: server.URL},
		httpClient: server.Client(),
	}

	alert := &alerting.Alert{
		RuleName:  "Test",
		Severity:  alerting.SeverityLow,
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := notifier.Send(ctx, alert)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestSeverityEmoji(t *testing.T) {
	tests := []struct {
		severity alerting.Severity
		want     string
	}{
		{alerting.SeverityCritical, "\U0001F534"}, // red
		{alerting.SeverityHigh, "\U0001F7E0"},     // orange
		{alerting.SeverityMedium, "\U0001F7E1"},   // yellow
		{alerting.SeverityLow, "\U0001F7E2"},      // green
		{alerting.Severity("unknown"), "\u26AA"},  // white
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			got := severityEmoji(tt.severity)
			if got != tt.want {
				t.Errorf("severityEmoji(%q) = %q, want %q", tt.severity, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestSlackBlockFormatting(t *testing.T) {
	notifier := &SlackNotifier{}

	alert := &alerting.Alert{
		RuleName:    "High Error Rate",
		Description: "More than 100 errors in 5 minutes",
		Severity:    alerting.SeverityCritical,
		Message:     "Threshold exceeded: 150 events in 5m (threshold: 100)",
		Timestamp:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Count:       150,
		Threshold:   100,
		Window:      "5m",
	}

	payload := notifier.buildPayload(alert)

	// Should have: header, severity/time, message, count/threshold, description
	if len(payload.Blocks) < 4 {
		t.Errorf("expected at least 4 blocks, got %d", len(payload.Blocks))
	}

	// Verify block types
	blockTypes := make([]string, len(payload.Blocks))
	for i, b := range payload.Blocks {
		blockTypes[i] = b.Type
	}

	if blockTypes[0] != "header" {
		t.Errorf("first block should be header, got %s", blockTypes[0])
	}

	// Check that severity emoji is in header
	if !strings.Contains(payload.Blocks[0].Text.Text, "\U0001F534") {
		t.Error("critical alert should have red circle emoji")
	}

	// Verify JSON serialization works
	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	if !strings.Contains(string(jsonData), "High Error Rate") {
		t.Error("JSON missing rule name")
	}
	if !strings.Contains(string(jsonData), "CRITICAL") {
		t.Error("JSON missing severity")
	}
}

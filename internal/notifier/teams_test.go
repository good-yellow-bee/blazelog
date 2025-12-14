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

func TestTeamsConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  TeamsConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty config",
			config:  TeamsConfig{},
			wantErr: true,
			errMsg:  "webhook URL is required",
		},
		{
			name: "http URL rejected",
			config: TeamsConfig{
				WebhookURL: "http://outlook.office.com/webhook/xxx",
			},
			wantErr: true,
			errMsg:  "webhook URL must use HTTPS",
		},
		{
			name: "valid config",
			config: TeamsConfig{
				WebhookURL: "https://outlook.office.com/webhook/xxx",
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

func TestTeamsNotifierName(t *testing.T) {
	notifier := &TeamsNotifier{}
	if got := notifier.Name(); got != "teams" {
		t.Errorf("Name() = %q, want %q", got, "teams")
	}
}

func TestTeamsNotifierClose(t *testing.T) {
	notifier := &TeamsNotifier{}
	if err := notifier.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

func TestTeamsNotifierSend(t *testing.T) {
	var receivedPayload teamsMessage

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
		w.Write([]byte("1"))
	}))
	defer server.Close()

	// Use test server URL (allow non-HTTPS for testing)
	notifier := &TeamsNotifier{
		config: TeamsConfig{
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
	if receivedPayload.Type != "message" {
		t.Errorf("payload type = %q, want %q", receivedPayload.Type, "message")
	}

	if len(receivedPayload.Attachments) == 0 {
		t.Fatal("expected attachments in payload")
	}

	attachment := receivedPayload.Attachments[0]
	if attachment.ContentType != "application/vnd.microsoft.card.adaptive" {
		t.Errorf("attachment contentType = %q, want adaptive card", attachment.ContentType)
	}

	if attachment.Content.Version != "1.4" {
		t.Errorf("card version = %q, want %q", attachment.Content.Version, "1.4")
	}

	if len(attachment.Content.Body) == 0 {
		t.Fatal("expected body elements in card")
	}
}

func TestTeamsNotifierSendWithTriggeringEntry(t *testing.T) {
	var receivedPayload teamsMessage

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := &TeamsNotifier{
		config:     TeamsConfig{WebhookURL: server.URL},
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

	// Verify adaptive card structure
	if len(receivedPayload.Attachments) == 0 {
		t.Fatal("expected attachments")
	}

	// Check body contains expected elements
	bodyJSON, _ := json.Marshal(receivedPayload.Attachments[0].Content.Body)
	bodyStr := string(bodyJSON)

	if !strings.Contains(bodyStr, "Database connection failed") {
		t.Error("triggering entry message not found in payload")
	}

	if !strings.Contains(bodyStr, "environment=production") {
		t.Error("labels not found in payload")
	}

	if !strings.Contains(bodyStr, "/var/log/app/error.log") {
		t.Error("file path not found in payload")
	}
}

func TestTeamsNotifierHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	notifier := &TeamsNotifier{
		config:     TeamsConfig{WebhookURL: server.URL},
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

func TestTeamsNotifierContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := &TeamsNotifier{
		config:     TeamsConfig{WebhookURL: server.URL},
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
		t.Fatal("expected error for cancelled context")
	}
}

func TestSeverityColor(t *testing.T) {
	tests := []struct {
		severity alerting.Severity
		want     string
	}{
		{alerting.SeverityCritical, "attention"},
		{alerting.SeverityHigh, "warning"},
		{alerting.SeverityMedium, "accent"},
		{alerting.SeverityLow, "good"},
		{alerting.Severity("unknown"), "default"},
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

func TestTeamsAdaptiveCardFormatting(t *testing.T) {
	notifier := &TeamsNotifier{}

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

	// Verify message structure
	if payload.Type != "message" {
		t.Errorf("payload type = %q, want %q", payload.Type, "message")
	}

	if len(payload.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(payload.Attachments))
	}

	attachment := payload.Attachments[0]
	if attachment.ContentType != "application/vnd.microsoft.card.adaptive" {
		t.Errorf("wrong content type: %s", attachment.ContentType)
	}

	card := attachment.Content
	if card.Type != "AdaptiveCard" {
		t.Errorf("card type = %q, want %q", card.Type, "AdaptiveCard")
	}

	if card.Version != "1.4" {
		t.Errorf("card version = %q, want %q", card.Version, "1.4")
	}

	// Verify JSON serialization works
	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	jsonStr := string(jsonData)
	if !strings.Contains(jsonStr, "High Error Rate") {
		t.Error("JSON missing rule name")
	}
	if !strings.Contains(jsonStr, "CRITICAL") {
		t.Error("JSON missing severity")
	}
	if !strings.Contains(jsonStr, "attention") {
		t.Error("JSON missing critical color style")
	}
}

func TestNewTeamsNotifierValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  TeamsConfig
		wantErr bool
	}{
		{
			name:    "empty URL",
			config:  TeamsConfig{},
			wantErr: true,
		},
		{
			name: "http URL",
			config: TeamsConfig{
				WebhookURL: "http://example.com/webhook",
			},
			wantErr: true,
		},
		{
			name: "valid URL",
			config: TeamsConfig{
				WebhookURL: "https://outlook.office.com/webhook/xxx",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTeamsNotifier(tt.config)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

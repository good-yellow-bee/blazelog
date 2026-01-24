package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/alerting"
)

// TeamsConfig holds Microsoft Teams webhook configuration.
type TeamsConfig struct {
	WebhookURL string // Teams incoming webhook URL
}

// Validate validates the Teams configuration.
func (c *TeamsConfig) Validate() error {
	if c.WebhookURL == "" {
		return fmt.Errorf("webhook URL is required")
	}
	if !strings.HasPrefix(c.WebhookURL, "https://") {
		return fmt.Errorf("webhook URL must use HTTPS")
	}
	return nil
}

// TeamsNotifier sends alerts to Microsoft Teams via webhook.
type TeamsNotifier struct {
	config     TeamsConfig
	httpClient *http.Client
}

// NewTeamsNotifier creates a new Teams notifier.
func NewTeamsNotifier(config TeamsConfig) (*TeamsNotifier, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid teams config: %w", err)
	}

	return &TeamsNotifier{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Name returns "teams".
func (t *TeamsNotifier) Name() string {
	return "teams"
}

// Send sends an alert to Microsoft Teams.
func (t *TeamsNotifier) Send(ctx context.Context, alert *alerting.Alert) error {
	payload := t.buildPayload(alert)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.config.WebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("teams API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close is a no-op for Teams notifier.
func (t *TeamsNotifier) Close() error {
	return nil
}

// teamsMessage represents the Teams webhook payload with Adaptive Card.
type teamsMessage struct {
	Type        string            `json:"type"`
	Attachments []teamsAttachment `json:"attachments"`
}

// teamsAttachment represents an attachment in the Teams message.
type teamsAttachment struct {
	ContentType string       `json:"contentType"`
	ContentURL  *string      `json:"contentUrl"`
	Content     adaptiveCard `json:"content"`
}

// adaptiveCard represents a Microsoft Adaptive Card.
type adaptiveCard struct {
	Schema  string        `json:"$schema"`
	Type    string        `json:"type"`
	Version string        `json:"version"`
	Body    []interface{} `json:"body"`
}

// Adaptive Card element types
type textBlock struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Size   string `json:"size,omitempty"`
	Weight string `json:"weight,omitempty"`
	Color  string `json:"color,omitempty"`
	Wrap   bool   `json:"wrap,omitempty"`
}

type factSet struct {
	Type  string `json:"type"`
	Facts []fact `json:"facts"`
}

type fact struct {
	Title string `json:"title"`
	Value string `json:"value"`
}

type container struct {
	Type  string        `json:"type"`
	Style string        `json:"style,omitempty"`
	Items []interface{} `json:"items"`
}

// buildPayload builds the Teams Adaptive Card message payload.
func (t *TeamsNotifier) buildPayload(alert *alerting.Alert) teamsMessage {
	timestamp := alert.Timestamp.Format("2006-01-02 15:04:05 MST")
	emoji := severityEmoji(alert.Severity)
	color := teamsSeverityStyle(alert.Severity)

	body := []interface{}{}

	// Header container with severity color
	headerContainer := container{
		Type:  "Container",
		Style: color,
		Items: []interface{}{
			textBlock{
				Type:   "TextBlock",
				Text:   fmt.Sprintf("%s BlazeLog Alert: %s", emoji, alert.RuleName),
				Size:   "Large",
				Weight: "Bolder",
				Wrap:   true,
			},
		},
	}
	body = append(body, headerContainer)

	// Alert details fact set
	facts := []fact{
		{Title: "Severity", Value: fmt.Sprintf("%s %s", emoji, strings.ToUpper(string(alert.Severity)))},
		{Title: "Time", Value: timestamp},
	}

	body = append(body,
		factSet{
			Type:  "FactSet",
			Facts: facts,
		},
		textBlock{
			Type: "TextBlock",
			Text: fmt.Sprintf("**Message:** %s", alert.Message),
			Wrap: true,
		},
	)

	// Threshold info if applicable
	if alert.Threshold > 0 {
		body = append(body, factSet{
			Type: "FactSet",
			Facts: []fact{
				{Title: "Count", Value: fmt.Sprintf("%d", alert.Count)},
				{Title: "Threshold", Value: fmt.Sprintf("%d in %s", alert.Threshold, alert.Window)},
			},
		})
	}

	// Triggering entry if available
	if alert.TriggeringEntry != nil {
		entry := alert.TriggeringEntry
		entryText := fmt.Sprintf("`%s [%s] %s`",
			entry.Timestamp.Format("15:04:05"),
			strings.ToUpper(string(entry.Level)),
			truncate(entry.Message, 200))

		if entry.FilePath != "" {
			entryText = fmt.Sprintf("**File:** `%s`\n\n%s", entry.FilePath, entryText)
		}

		body = append(body, textBlock{
			Type: "TextBlock",
			Text: entryText,
			Wrap: true,
		})
	}

	// Description
	if alert.Description != "" {
		body = append(body, textBlock{
			Type:  "TextBlock",
			Text:  fmt.Sprintf("_Rule: %s_", alert.Description),
			Wrap:  true,
			Color: "light",
		})
	}

	// Labels if present
	if len(alert.Labels) > 0 {
		labelParts := make([]string, 0, len(alert.Labels))
		for k, v := range alert.Labels {
			labelParts = append(labelParts, fmt.Sprintf("`%s=%s`", k, v))
		}
		body = append(body, textBlock{
			Type:  "TextBlock",
			Text:  fmt.Sprintf("**Labels:** %s", strings.Join(labelParts, " ")),
			Wrap:  true,
			Color: "light",
		})
	}

	return teamsMessage{
		Type: "message",
		Attachments: []teamsAttachment{
			{
				ContentType: "application/vnd.microsoft.card.adaptive",
				ContentURL:  nil,
				Content: adaptiveCard{
					Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
					Type:    "AdaptiveCard",
					Version: "1.4",
					Body:    body,
				},
			},
		},
	}
}

// teamsSeverityStyle returns an Adaptive Card container style for the severity level.
func teamsSeverityStyle(severity alerting.Severity) string {
	switch severity {
	case alerting.SeverityCritical:
		return "attention" // red
	case alerting.SeverityHigh:
		return "warning" // orange/yellow
	case alerting.SeverityMedium:
		return "accent" // blue
	case alerting.SeverityLow:
		return "good" // green
	default:
		return "default"
	}
}

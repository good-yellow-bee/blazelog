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

// SlackConfig holds Slack webhook configuration.
type SlackConfig struct {
	WebhookURL string // Slack incoming webhook URL
}

// Validate validates the Slack configuration.
func (c *SlackConfig) Validate() error {
	if c.WebhookURL == "" {
		return fmt.Errorf("webhook URL is required")
	}
	if !strings.HasPrefix(c.WebhookURL, "https://") {
		return fmt.Errorf("webhook URL must use HTTPS")
	}
	return nil
}

// SlackNotifier sends alerts to Slack via webhook.
type SlackNotifier struct {
	config     SlackConfig
	httpClient *http.Client
}

// NewSlackNotifier creates a new Slack notifier.
func NewSlackNotifier(config SlackConfig) (*SlackNotifier, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid slack config: %w", err)
	}

	return &SlackNotifier{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Name returns "slack".
func (s *SlackNotifier) Name() string {
	return "slack"
}

// Send sends an alert to Slack.
func (s *SlackNotifier) Send(ctx context.Context, alert *alerting.Alert) error {
	payload := s.buildPayload(alert)

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.config.WebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("slack API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Close is a no-op for Slack notifier.
func (s *SlackNotifier) Close() error {
	return nil
}

// slackMessage represents the Slack webhook payload.
type slackMessage struct {
	Blocks []slackBlock `json:"blocks"`
}

// slackBlock represents a Slack Block Kit block.
type slackBlock struct {
	Type     string           `json:"type"`
	Text     *slackText       `json:"text,omitempty"`
	Fields   []slackText      `json:"fields,omitempty"`
	Elements []slackText      `json:"elements,omitempty"`
}

// slackText represents text in Slack Block Kit.
type slackText struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// buildPayload builds the Slack Block Kit message payload.
func (s *SlackNotifier) buildPayload(alert *alerting.Alert) slackMessage {
	emoji := severityEmoji(alert.Severity)
	timestamp := alert.Timestamp.Format("2006-01-02 15:04:05 MST")

	blocks := []slackBlock{
		// Header
		{
			Type: "header",
			Text: &slackText{
				Type:  "plain_text",
				Text:  fmt.Sprintf("%s BlazeLog Alert: %s", emoji, alert.RuleName),
				Emoji: true,
			},
		},
		// Severity and Time fields
		{
			Type: "section",
			Fields: []slackText{
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Severity:*\n%s %s", emoji, strings.ToUpper(string(alert.Severity))),
				},
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Time:*\n%s", timestamp),
				},
			},
		},
		// Message
		{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*Message:*\n%s", alert.Message),
			},
		},
	}

	// Add threshold info if applicable
	if alert.Threshold > 0 {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Fields: []slackText{
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Count:*\n%d", alert.Count),
				},
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*Threshold:*\n%d in %s", alert.Threshold, alert.Window),
				},
			},
		})
	}

	// Add triggering entry if available
	if alert.TriggeringEntry != nil {
		entry := alert.TriggeringEntry
		entryText := fmt.Sprintf("```%s [%s] %s```",
			entry.Timestamp.Format("15:04:05"),
			strings.ToUpper(string(entry.Level)),
			truncate(entry.Message, 200))

		if entry.FilePath != "" {
			entryText = fmt.Sprintf("*File:* `%s`\n%s", entry.FilePath, entryText)
		}

		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: entryText,
			},
		})
	}

	// Add description as context
	if alert.Description != "" {
		blocks = append(blocks, slackBlock{
			Type: "context",
			Elements: []slackText{
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("Rule: %s", alert.Description),
				},
			},
		})
	}

	// Add labels if present
	if len(alert.Labels) > 0 {
		labelParts := make([]string, 0, len(alert.Labels))
		for k, v := range alert.Labels {
			labelParts = append(labelParts, fmt.Sprintf("`%s=%s`", k, v))
		}
		blocks = append(blocks, slackBlock{
			Type: "context",
			Elements: []slackText{
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("Labels: %s", strings.Join(labelParts, " ")),
				},
			},
		})
	}

	return slackMessage{Blocks: blocks}
}

// severityEmoji returns an emoji for the severity level.
func severityEmoji(severity alerting.Severity) string {
	switch severity {
	case alerting.SeverityCritical:
		return "\U0001F534" // red circle
	case alerting.SeverityHigh:
		return "\U0001F7E0" // orange circle
	case alerting.SeverityMedium:
		return "\U0001F7E1" // yellow circle
	case alerting.SeverityLow:
		return "\U0001F7E2" // green circle
	default:
		return "\u26AA" // white circle
	}
}

// truncate truncates a string to max length with ellipsis.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

package notifier

import (
	"bytes"
	"embed"
	"strings"
	"text/template"

	"github.com/good-yellow-bee/blazelog/internal/alerting"
)

//go:embed templates/*
var templateFS embed.FS

// Templates holds parsed email templates.
type Templates struct {
	html  *template.Template
	plain *template.Template
}

// TemplateData contains data for template rendering.
type TemplateData struct {
	RuleName        string
	Description     string
	Severity        string
	SeverityColor   string
	Message         string
	Timestamp       string
	Count           int
	Threshold       int
	Window          string
	TriggeringEntry *LogEntryData
	Labels          map[string]string
}

// LogEntryData contains log entry data for templates.
type LogEntryData struct {
	Timestamp string
	Level     string
	Message   string
	Source    string
	FilePath  string
}

// LoadTemplates loads embedded email templates.
func LoadTemplates() (*Templates, error) {
	funcs := template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
	}

	htmlTmpl, err := template.New("alert.html").Funcs(funcs).ParseFS(templateFS, "templates/alert.html")
	if err != nil {
		return nil, err
	}

	plainTmpl, err := template.New("alert.txt").Funcs(funcs).ParseFS(templateFS, "templates/alert.txt")
	if err != nil {
		return nil, err
	}

	return &Templates{
		html:  htmlTmpl,
		plain: plainTmpl,
	}, nil
}

// RenderHTML renders the HTML email body.
func (t *Templates) RenderHTML(data *TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := t.html.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// RenderPlain renders the plain text email body.
func (t *Templates) RenderPlain(data *TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := t.plain.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// severityColor returns the color for a severity level.
func severityColor(severity alerting.Severity) string {
	switch severity {
	case alerting.SeverityCritical:
		return "#d32f2f" // red
	case alerting.SeverityHigh:
		return "#f57c00" // orange
	case alerting.SeverityMedium:
		return "#fbc02d" // yellow
	case alerting.SeverityLow:
		return "#388e3c" // green
	default:
		return "#757575" // gray
	}
}

// AlertToTemplateData converts an alert to template data.
func AlertToTemplateData(alert *alerting.Alert) TemplateData {
	data := TemplateData{
		RuleName:      alert.RuleName,
		Description:   alert.Description,
		Severity:      string(alert.Severity),
		SeverityColor: severityColor(alert.Severity),
		Message:       alert.Message,
		Timestamp:     alert.Timestamp.Format("2006-01-02 15:04:05 MST"),
		Count:         alert.Count,
		Threshold:     alert.Threshold,
		Window:        alert.Window,
		Labels:        alert.Labels,
	}

	if alert.TriggeringEntry != nil {
		entry := alert.TriggeringEntry
		data.TriggeringEntry = &LogEntryData{
			Timestamp: entry.Timestamp.Format("2006-01-02 15:04:05"),
			Level:     string(entry.Level),
			Message:   entry.Message,
			Source:    entry.Source,
			FilePath:  entry.FilePath,
		}
	}

	return data
}

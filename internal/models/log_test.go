package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewLogEntry(t *testing.T) {
	entry := NewLogEntry()

	if entry == nil {
		t.Fatal("NewLogEntry returned nil")
	}

	if entry.Fields == nil {
		t.Error("Fields map should be initialized")
	}

	if entry.Labels == nil {
		t.Error("Labels map should be initialized")
	}

	if entry.Level != LevelUnknown {
		t.Errorf("Expected level %v, got %v", LevelUnknown, entry.Level)
	}

	if entry.Type != LogTypeUnknown {
		t.Errorf("Expected type %v, got %v", LogTypeUnknown, entry.Type)
	}
}

func TestLogEntry_SetGetField(t *testing.T) {
	entry := NewLogEntry()

	// Test setting and getting field
	entry.SetField("status", 200)
	val, ok := entry.GetField("status")
	if !ok {
		t.Error("GetField should return true for existing field")
	}
	if val != 200 {
		t.Errorf("Expected value 200, got %v", val)
	}

	// Test getting non-existent field
	_, ok = entry.GetField("nonexistent")
	if ok {
		t.Error("GetField should return false for non-existent field")
	}
}

func TestLogEntry_GetFieldString(t *testing.T) {
	entry := NewLogEntry()
	entry.SetField("method", "GET")
	entry.SetField("status", 200)

	// Test string field
	method := entry.GetFieldString("method")
	if method != "GET" {
		t.Errorf("Expected 'GET', got '%s'", method)
	}

	// Test non-string field returns empty
	status := entry.GetFieldString("status")
	if status != "" {
		t.Errorf("Expected empty string for non-string field, got '%s'", status)
	}

	// Test non-existent field
	empty := entry.GetFieldString("nonexistent")
	if empty != "" {
		t.Errorf("Expected empty string for non-existent field, got '%s'", empty)
	}
}

func TestLogEntry_GetFieldInt(t *testing.T) {
	entry := NewLogEntry()
	entry.SetField("int_val", 200)
	entry.SetField("int64_val", int64(300))
	entry.SetField("float64_val", float64(400.5))
	entry.SetField("string_val", "not a number")

	tests := []struct {
		field    string
		expected int
	}{
		{"int_val", 200},
		{"int64_val", 300},
		{"float64_val", 400},
		{"string_val", 0},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		got := entry.GetFieldInt(tt.field)
		if got != tt.expected {
			t.Errorf("GetFieldInt(%s): expected %d, got %d", tt.field, tt.expected, got)
		}
	}
}

func TestLogEntry_SetGetLabel(t *testing.T) {
	entry := NewLogEntry()

	entry.SetLabel("env", "production")
	label := entry.GetLabel("env")
	if label != "production" {
		t.Errorf("Expected 'production', got '%s'", label)
	}

	// Test non-existent label
	empty := entry.GetLabel("nonexistent")
	if empty != "" {
		t.Errorf("Expected empty string for non-existent label, got '%s'", empty)
	}
}

func TestLogEntry_JSON(t *testing.T) {
	entry := NewLogEntry()
	entry.Timestamp = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	entry.Level = LevelError
	entry.Message = "Test error message"
	entry.SetField("status", 500)

	data, err := entry.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if parsed["level"] != "error" {
		t.Errorf("Expected level 'error', got '%v'", parsed["level"])
	}

	if parsed["message"] != "Test error message" {
		t.Errorf("Expected message 'Test error message', got '%v'", parsed["message"])
	}
}

func TestLogEntry_String(t *testing.T) {
	entry := NewLogEntry()
	entry.Timestamp = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	entry.Level = LevelInfo
	entry.Message = "Test message"

	str := entry.String()
	expected := "2024-01-15T10:30:00Z [info] Test message"
	if str != expected {
		t.Errorf("Expected '%s', got '%s'", expected, str)
	}
}

func TestLogEntry_IsError(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected bool
	}{
		{LevelDebug, false},
		{LevelInfo, false},
		{LevelWarning, false},
		{LevelError, true},
		{LevelFatal, true},
		{LevelUnknown, false},
	}

	for _, tt := range tests {
		entry := NewLogEntry()
		entry.Level = tt.level
		if entry.IsError() != tt.expected {
			t.Errorf("IsError() for level %v: expected %v, got %v", tt.level, tt.expected, entry.IsError())
		}
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"Debug", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"notice", LevelInfo},
		{"warning", LevelWarning},
		{"WARN", LevelWarning},
		{"error", LevelError},
		{"ERROR", LevelError},
		{"err", LevelError},
		{"fatal", LevelFatal},
		{"FATAL", LevelFatal},
		{"critical", LevelFatal},
		{"CRITICAL", LevelFatal},
		{"emergency", LevelFatal},
		{"unknown_level", LevelUnknown},
		{"", LevelUnknown},
	}

	for _, tt := range tests {
		got := ParseLogLevel(tt.input)
		if got != tt.expected {
			t.Errorf("ParseLogLevel(%q): expected %v, got %v", tt.input, tt.expected, got)
		}
	}
}

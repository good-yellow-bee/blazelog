package parser

import (
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// TestMagentoParser_Parse tests the Magento log parser.
func TestMagentoParser_Parse(t *testing.T) {
	parser := NewMagentoParser(nil)

	tests := []struct {
		name          string
		line          string
		expectError   bool
		expectedLevel models.LogLevel
		expectedMsg   string
		expectedChan  string
	}{
		{
			name:          "DEBUG level",
			line:          `[2024-01-15 10:23:45] main.DEBUG: Debug message here {"is_exception":false} []`,
			expectError:   false,
			expectedLevel: models.LevelDebug,
			expectedMsg:   "Debug message here",
			expectedChan:  "main",
		},
		{
			name:          "INFO level",
			line:          `[2024-01-15 10:23:45] main.INFO: Information message [] []`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "Information message",
			expectedChan:  "main",
		},
		{
			name:          "NOTICE level (maps to INFO)",
			line:          `[2024-01-15 10:23:45] main.NOTICE: Notice message [] []`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "Notice message",
			expectedChan:  "main",
		},
		{
			name:          "WARNING level",
			line:          `[2024-01-15 10:23:45] main.WARNING: Warning message {} []`,
			expectError:   false,
			expectedLevel: models.LevelWarning,
			expectedMsg:   "Warning message",
			expectedChan:  "main",
		},
		{
			name:          "ERROR level",
			line:          `[2024-01-15 10:23:45] main.ERROR: Error occurred in the system {} []`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "Error occurred in the system",
			expectedChan:  "main",
		},
		{
			name:          "CRITICAL level (maps to FATAL)",
			line:          `[2024-01-15 10:23:45] main.CRITICAL: Critical system failure {} []`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "Critical system failure",
			expectedChan:  "main",
		},
		{
			name:          "ALERT level (maps to FATAL)",
			line:          `[2024-01-15 10:23:45] main.ALERT: Alert! Immediate attention required {} []`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "Alert! Immediate attention required",
			expectedChan:  "main",
		},
		{
			name:          "EMERGENCY level (maps to FATAL)",
			line:          `[2024-01-15 10:23:45] main.EMERGENCY: System is down! {} []`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "System is down!",
			expectedChan:  "main",
		},
		{
			name:          "Different channel (report)",
			line:          `[2024-01-15 10:23:45] report.CRITICAL: Report error occurred {} []`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "Report error occurred",
			expectedChan:  "report",
		},
		{
			name:          "Different channel (exception)",
			line:          `[2024-01-15 10:23:45] exception.ERROR: Exception caught in module {} []`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "Exception caught in module",
			expectedChan:  "exception",
		},
		{
			name:          "Message with special characters",
			line:          `[2024-01-15 10:23:45] main.ERROR: Error in /var/www/magento/app/code/Module/File.php:123 {} []`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "Error in /var/www/magento/app/code/Module/File.php:123",
			expectedChan:  "main",
		},
		{
			name:          "Message with quotes",
			line:          `[2024-01-15 10:23:45] main.INFO: Processing order "12345" for customer {} []`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   `Processing order "12345" for customer`,
			expectedChan:  "main",
		},
		{
			name:        "Invalid format - no brackets",
			line:        "this is not a valid log line",
			expectError: true,
		},
		{
			name:        "Empty line",
			line:        "",
			expectError: true,
		},
		{
			name:        "Invalid format - missing timestamp",
			line:        `[] main.INFO: Test message [] []`,
			expectError: true,
		},
		{
			name:        "Invalid format - wrong date format",
			line:        `[15-01-2024 10:23:45] main.INFO: Test message [] []`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)

			if tt.expectError {
				if err == nil {
					t.Errorf("Parse(%q): expected error, got nil", tt.line)
				}
				return
			}

			if err != nil {
				t.Errorf("Parse(%q): unexpected error: %v", tt.line, err)
				return
			}

			if entry.Level != tt.expectedLevel {
				t.Errorf("Parse(%q): level = %v, want %v", tt.line, entry.Level, tt.expectedLevel)
			}

			if entry.Message != tt.expectedMsg {
				t.Errorf("Parse(%q): message = %q, want %q", tt.line, entry.Message, tt.expectedMsg)
			}

			if entry.GetFieldString("channel") != tt.expectedChan {
				t.Errorf("Parse(%q): channel = %q, want %q", tt.line, entry.GetFieldString("channel"), tt.expectedChan)
			}

			if entry.Type != models.LogTypeMagento {
				t.Errorf("Parse(%q): type = %v, want %v", tt.line, entry.Type, models.LogTypeMagento)
			}
		})
	}
}

// TestMagentoParser_ParseTimestamp tests timestamp parsing.
func TestMagentoParser_ParseTimestamp(t *testing.T) {
	parser := NewMagentoParser(nil)

	tests := []struct {
		name            string
		line            string
		expectedYear    int
		expectedMonth   time.Month
		expectedDay     int
		expectedHour    int
		expectedMinute  int
		expectedSecond  int
	}{
		{
			name:            "standard timestamp",
			line:            `[2024-01-15 10:23:45] main.INFO: Test message [] []`,
			expectedYear:    2024,
			expectedMonth:   time.January,
			expectedDay:     15,
			expectedHour:    10,
			expectedMinute:  23,
			expectedSecond:  45,
		},
		{
			name:            "midnight timestamp",
			line:            `[2024-12-31 00:00:00] main.INFO: New Year's Eve [] []`,
			expectedYear:    2024,
			expectedMonth:   time.December,
			expectedDay:     31,
			expectedHour:    0,
			expectedMinute:  0,
			expectedSecond:  0,
		},
		{
			name:            "end of day timestamp",
			line:            `[2024-06-15 23:59:59] main.INFO: End of day [] []`,
			expectedYear:    2024,
			expectedMonth:   time.June,
			expectedDay:     15,
			expectedHour:    23,
			expectedMinute:  59,
			expectedSecond:  59,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", tt.line, err)
			}

			if entry.Timestamp.Year() != tt.expectedYear {
				t.Errorf("year = %d, want %d", entry.Timestamp.Year(), tt.expectedYear)
			}
			if entry.Timestamp.Month() != tt.expectedMonth {
				t.Errorf("month = %v, want %v", entry.Timestamp.Month(), tt.expectedMonth)
			}
			if entry.Timestamp.Day() != tt.expectedDay {
				t.Errorf("day = %d, want %d", entry.Timestamp.Day(), tt.expectedDay)
			}
			if entry.Timestamp.Hour() != tt.expectedHour {
				t.Errorf("hour = %d, want %d", entry.Timestamp.Hour(), tt.expectedHour)
			}
			if entry.Timestamp.Minute() != tt.expectedMinute {
				t.Errorf("minute = %d, want %d", entry.Timestamp.Minute(), tt.expectedMinute)
			}
			if entry.Timestamp.Second() != tt.expectedSecond {
				t.Errorf("second = %d, want %d", entry.Timestamp.Second(), tt.expectedSecond)
			}
		})
	}
}

// TestMagentoParser_ParseContext tests parsing of context JSON.
func TestMagentoParser_ParseContext(t *testing.T) {
	parser := NewMagentoParser(nil)

	tests := []struct {
		name            string
		line            string
		expectContext   bool
		expectedIsExc   bool
		expectedClass   string
	}{
		{
			name:            "with is_exception false",
			line:            `[2024-01-15 10:23:45] main.DEBUG: Debug message {"is_exception":false} []`,
			expectContext:   true,
			expectedIsExc:   false,
		},
		{
			name:            "with is_exception true",
			line:            `[2024-01-15 10:23:45] main.ERROR: Error occurred {"is_exception":true} []`,
			expectContext:   true,
			expectedIsExc:   true,
		},
		{
			name:            "with exception class",
			line:            `[2024-01-15 10:23:45] main.ERROR: Exception {"class":"Magento\\Framework\\Exception\\LocalizedException"} []`,
			expectContext:   true,
			expectedClass:   "Magento\\Framework\\Exception\\LocalizedException",
		},
		{
			name:            "empty context",
			line:            `[2024-01-15 10:23:45] main.INFO: Simple message {} []`,
			expectContext:   false,
		},
		{
			name:            "no context (empty braces)",
			line:            `[2024-01-15 10:23:45] main.INFO: Message [] []`,
			expectContext:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", tt.line, err)
			}

			context, hasContext := entry.GetField("context")

			if tt.expectContext && !hasContext {
				t.Errorf("expected context, but none found")
				return
			}

			if tt.expectContext && hasContext {
				contextMap, ok := context.(map[string]interface{})
				if !ok {
					t.Errorf("context is not a map")
					return
				}

				if tt.expectedClass != "" {
					if class, ok := entry.GetField("exception_class"); ok {
						if class != tt.expectedClass {
							t.Errorf("exception_class = %v, want %v", class, tt.expectedClass)
						}
					}
				}

				if isExc, ok := contextMap["is_exception"]; ok {
					if isExc != tt.expectedIsExc {
						t.Errorf("is_exception = %v, want %v", isExc, tt.expectedIsExc)
					}
				}
			}
		})
	}
}

// TestMagentoParser_ParseMultiLine tests multiline parsing for stack traces.
func TestMagentoParser_ParseMultiLine(t *testing.T) {
	parser := NewMagentoParser(nil)

	tests := []struct {
		name               string
		lines              []string
		expectError        bool
		expectedLevel      models.LogLevel
		expectedMsg        string
		expectedFrameCount int
		expectedMultiline  bool
	}{
		{
			name: "single line - no stack trace",
			lines: []string{
				`[2024-01-15 10:23:45] main.INFO: Simple message [] []`,
			},
			expectError:       false,
			expectedLevel:     models.LevelInfo,
			expectedMsg:       "Simple message",
			expectedMultiline: false,
		},
		{
			name: "exception with stack trace",
			lines: []string{
				`[2024-01-15 10:23:45] main.CRITICAL: Exception message in /var/www/magento/app/code/Module/File.php:123 {"is_exception":true} []`,
				`#0 /var/www/magento/vendor/magento/framework/App.php(456): Module\File->method()`,
				`#1 /var/www/magento/vendor/magento/framework/Bootstrap.php(789): Magento\Framework\App->run()`,
				`#2 /var/www/magento/pub/index.php(12): Magento\Framework\Bootstrap->run()`,
				`#3 {main}`,
			},
			expectError:        false,
			expectedLevel:      models.LevelFatal,
			expectedMsg:        "Exception message in /var/www/magento/app/code/Module/File.php:123",
			expectedFrameCount: 4,
			expectedMultiline:  true,
		},
		{
			name: "error with short stack trace",
			lines: []string{
				`[2024-01-15 10:23:45] main.ERROR: Error occurred {} []`,
				`Stack trace:`,
				`#0 /path/to/file.php(10): function()`,
				`#1 {main}`,
			},
			expectError:        false,
			expectedLevel:      models.LevelError,
			expectedMsg:        "Error occurred",
			expectedFrameCount: 2,
			expectedMultiline:  true,
		},
		{
			name: "exception with nested exception",
			lines: []string{
				`[2024-01-15 10:23:45] main.CRITICAL: Outer exception {} []`,
				`#0 /path/file.php(10): outer()`,
				`#1 {main}`,
				``,
				`Caused by: Inner exception`,
				`#0 /path/inner.php(20): inner()`,
				`#1 {main}`,
			},
			expectError:        false,
			expectedLevel:      models.LevelFatal,
			expectedMsg:        "Outer exception",
			expectedFrameCount: 4, // All frames counted
			expectedMultiline:  true,
		},
		{
			name:        "empty lines",
			lines:       []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.ParseMultiLine(tt.lines)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseMultiLine: expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseMultiLine: unexpected error: %v", err)
			}

			if entry.Level != tt.expectedLevel {
				t.Errorf("level = %v, want %v", entry.Level, tt.expectedLevel)
			}

			if entry.Message != tt.expectedMsg {
				t.Errorf("message = %q, want %q", entry.Message, tt.expectedMsg)
			}

			if tt.expectedMultiline {
				multiline, _ := entry.GetField("multiline")
				if multiline != true {
					t.Errorf("multiline = %v, want true", multiline)
				}

				stackTrace, hasStackTrace := entry.GetField("stack_trace")
				if !hasStackTrace {
					t.Errorf("expected stack_trace field")
				} else if stackTrace == "" {
					t.Errorf("stack_trace is empty")
				}

				frameCount := entry.GetFieldInt("stack_frame_count")
				if frameCount != tt.expectedFrameCount {
					t.Errorf("stack_frame_count = %d, want %d", frameCount, tt.expectedFrameCount)
				}
			} else {
				_, hasMultiline := entry.GetField("multiline")
				if hasMultiline {
					t.Errorf("unexpected multiline field for single-line entry")
				}
			}
		})
	}
}

// TestMagentoParser_IsStartOfEntry tests detection of log entry start.
func TestMagentoParser_IsStartOfEntry(t *testing.T) {
	parser := NewMagentoParser(nil)

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "valid start - INFO",
			line:     `[2024-01-15 10:23:45] main.INFO: Message [] []`,
			expected: true,
		},
		{
			name:     "valid start - ERROR",
			line:     `[2024-01-15 10:23:45] main.ERROR: Error [] []`,
			expected: true,
		},
		{
			name:     "valid start - different timestamp",
			line:     `[2000-12-31 23:59:59] report.DEBUG: Debug [] []`,
			expected: true,
		},
		{
			name:     "stack trace line",
			line:     `#0 /var/www/magento/app/code/Module/File.php(123): method()`,
			expected: false,
		},
		{
			name:     "continuation line",
			line:     `    at Magento\Framework\App\Http->launch()`,
			expected: false,
		},
		{
			name:     "empty line",
			line:     "",
			expected: false,
		},
		{
			name:     "regular text",
			line:     "This is just regular text",
			expected: false,
		},
		{
			name:     "malformed timestamp",
			line:     `[15-01-2024 10:23:45] main.INFO: Message [] []`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.IsStartOfEntry(tt.line)
			if result != tt.expected {
				t.Errorf("IsStartOfEntry(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

// TestMagentoParser_CanParse tests auto-detection capability.
func TestMagentoParser_CanParse(t *testing.T) {
	parser := NewMagentoParser(nil)

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "valid Magento log",
			line:     `[2024-01-15 10:23:45] main.INFO: Message [] []`,
			expected: true,
		},
		{
			name:     "valid Magento error log",
			line:     `[2024-01-15 10:23:45] main.ERROR: Error occurred {"is_exception":true} []`,
			expected: true,
		},
		{
			name:     "Nginx access log",
			line:     `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326`,
			expected: false,
		},
		{
			name:     "Apache error log",
			line:     `[Sat Oct 10 14:32:52.123456 2020] [core:error] [pid 12345:tid 123456789] [client 192.168.1.1:56789] AH00124: Request exceeded`,
			expected: false,
		},
		{
			name:     "random text",
			line:     "This is not a log line",
			expected: false,
		},
		{
			name:     "empty line",
			line:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.CanParse(tt.line)
			if result != tt.expected {
				t.Errorf("CanParse(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

// TestMagentoParser_Name tests the parser name.
func TestMagentoParser_Name(t *testing.T) {
	parser := NewMagentoParser(nil)
	expected := "magento"
	if parser.Name() != expected {
		t.Errorf("Name() = %q, want %q", parser.Name(), expected)
	}
}

// TestMagentoParser_Type tests the parser type.
func TestMagentoParser_Type(t *testing.T) {
	parser := NewMagentoParser(nil)
	expected := models.LogTypeMagento
	if parser.Type() != expected {
		t.Errorf("Type() = %v, want %v", parser.Type(), expected)
	}
}

// TestMagentoParser_Options tests parser options application.
func TestMagentoParser_Options(t *testing.T) {
	t.Run("include raw line", func(t *testing.T) {
		opts := &Options{
			IncludeRaw: true,
		}
		parser := NewMagentoParser(opts)
		line := `[2024-01-15 10:23:45] main.INFO: Test message [] []`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Raw != line {
			t.Errorf("Raw = %q, want %q", entry.Raw, line)
		}
	})

	t.Run("exclude raw line", func(t *testing.T) {
		opts := &Options{
			IncludeRaw: false,
		}
		parser := NewMagentoParser(opts)
		line := `[2024-01-15 10:23:45] main.INFO: Test message [] []`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Raw != "" {
			t.Errorf("Raw = %q, want empty", entry.Raw)
		}
	})

	t.Run("with source", func(t *testing.T) {
		opts := &Options{
			Source: "magento-server-1",
		}
		parser := NewMagentoParser(opts)
		line := `[2024-01-15 10:23:45] main.INFO: Test message [] []`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Source != "magento-server-1" {
			t.Errorf("Source = %q, want %q", entry.Source, "magento-server-1")
		}
	})

	t.Run("with labels", func(t *testing.T) {
		opts := &Options{
			Labels: map[string]string{
				"environment": "production",
				"app":         "magento",
			},
		}
		parser := NewMagentoParser(opts)
		line := `[2024-01-15 10:23:45] main.INFO: Test message [] []`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.GetLabel("environment") != "production" {
			t.Errorf("Label[environment] = %q, want %q", entry.GetLabel("environment"), "production")
		}
		if entry.GetLabel("app") != "magento" {
			t.Errorf("Label[app] = %q, want %q", entry.GetLabel("app"), "magento")
		}
	})
}

// TestMagentoParser_RealWorldLogs tests with realistic Magento log examples.
func TestMagentoParser_RealWorldLogs(t *testing.T) {
	parser := NewMagentoParser(nil)

	tests := []struct {
		name          string
		line          string
		expectedLevel models.LogLevel
		expectedChan  string
	}{
		{
			name:          "cache flush",
			line:          `[2024-01-15 10:23:45] main.INFO: Cache types config flushed successfully [] []`,
			expectedLevel: models.LevelInfo,
			expectedChan:  "main",
		},
		{
			name:          "indexer running",
			line:          `[2024-01-15 10:23:45] main.INFO: Indexer: catalogsearch_fulltext is started [] []`,
			expectedLevel: models.LevelInfo,
			expectedChan:  "main",
		},
		{
			name:          "cron job execution",
			line:          `[2024-01-15 10:23:45] main.DEBUG: Cron group: default, job: catalog_product_frontend_actions_flush is executed [] []`,
			expectedLevel: models.LevelDebug,
			expectedChan:  "main",
		},
		{
			name:          "payment error",
			line:          `[2024-01-15 10:23:45] main.ERROR: Payment capturing error [] []`,
			expectedLevel: models.LevelError,
			expectedChan:  "main",
		},
		{
			name:          "database exception",
			line:          `[2024-01-15 10:23:45] main.CRITICAL: SQLSTATE[HY000]: General error: 1205 Lock wait timeout exceeded {"is_exception":true} []`,
			expectedLevel: models.LevelFatal,
			expectedChan:  "main",
		},
		{
			name:          "API request",
			line:          `[2024-01-15 10:23:45] main.DEBUG: REST API request: GET /V1/products {"request_id":"abc123"} []`,
			expectedLevel: models.LevelDebug,
			expectedChan:  "main",
		},
		{
			name:          "session warning",
			line:          `[2024-01-15 10:23:45] main.WARNING: Session size of 256000 exceeded allowed session max size of 256000 [] []`,
			expectedLevel: models.LevelWarning,
			expectedChan:  "main",
		},
		{
			name:          "report channel",
			line:          `[2024-01-15 10:23:45] report.CRITICAL: Report ID: abc123; Message: Something went wrong [] []`,
			expectedLevel: models.LevelFatal,
			expectedChan:  "report",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", tt.line, err)
			}

			if entry.Level != tt.expectedLevel {
				t.Errorf("level = %v, want %v", entry.Level, tt.expectedLevel)
			}

			if entry.GetFieldString("channel") != tt.expectedChan {
				t.Errorf("channel = %q, want %q", entry.GetFieldString("channel"), tt.expectedChan)
			}
		})
	}
}

// TestMagentoLevelToLogLevel tests level conversion.
func TestMagentoLevelToLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected models.LogLevel
	}{
		{"DEBUG", models.LevelDebug},
		{"INFO", models.LevelInfo},
		{"NOTICE", models.LevelInfo},
		{"WARNING", models.LevelWarning},
		{"ERROR", models.LevelError},
		{"CRITICAL", models.LevelFatal},
		{"ALERT", models.LevelFatal},
		{"EMERGENCY", models.LevelFatal},
		{"UNKNOWN", models.LevelUnknown},
		{"invalid", models.LevelUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := magentoLevelToLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("magentoLevelToLogLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

package parser

import (
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// TestPrestaShopParser_Parse tests the PrestaShop log parser.
func TestPrestaShopParser_Parse(t *testing.T) {
	parser := NewPrestaShopParser(nil)

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
			line:          `[2024-01-15 10:23:45] app.DEBUG: Debug message here {"is_exception":false} []`,
			expectError:   false,
			expectedLevel: models.LevelDebug,
			expectedMsg:   "Debug message here",
			expectedChan:  "app",
		},
		{
			name:          "INFO level",
			line:          `[2024-01-15 10:23:45] request.INFO: Information message [] []`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "Information message",
			expectedChan:  "request",
		},
		{
			name:          "NOTICE level (maps to INFO)",
			line:          `[2024-01-15 10:23:45] security.NOTICE: Notice message [] []`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "Notice message",
			expectedChan:  "security",
		},
		{
			name:          "WARNING level",
			line:          `[2024-01-15 10:23:45] php.WARNING: Warning message {} []`,
			expectError:   false,
			expectedLevel: models.LevelWarning,
			expectedMsg:   "Warning message",
			expectedChan:  "php",
		},
		{
			name:          "ERROR level",
			line:          `[2024-01-15 10:23:45] request.ERROR: Error occurred in the system {} []`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "Error occurred in the system",
			expectedChan:  "request",
		},
		{
			name:          "CRITICAL level (maps to FATAL)",
			line:          `[2024-01-15 10:23:45] request.CRITICAL: Critical system failure {} []`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "Critical system failure",
			expectedChan:  "request",
		},
		{
			name:          "ALERT level (maps to FATAL)",
			line:          `[2024-01-15 10:23:45] app.ALERT: Alert! Immediate attention required {} []`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "Alert! Immediate attention required",
			expectedChan:  "app",
		},
		{
			name:          "EMERGENCY level (maps to FATAL)",
			line:          `[2024-01-15 10:23:45] app.EMERGENCY: System is down! {} []`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "System is down!",
			expectedChan:  "app",
		},
		{
			name:          "Different channel (console)",
			line:          `[2024-01-15 10:23:45] console.CRITICAL: Console error occurred {} []`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "Console error occurred",
			expectedChan:  "console",
		},
		{
			name:          "Different channel (doctrine)",
			line:          `[2024-01-15 10:23:45] doctrine.ERROR: Database query failed {} []`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "Database query failed",
			expectedChan:  "doctrine",
		},
		{
			name:          "Message with special characters",
			line:          `[2024-01-15 10:23:45] request.ERROR: Error in /var/www/prestashop/src/Controller/File.php:123 {} []`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "Error in /var/www/prestashop/src/Controller/File.php:123",
			expectedChan:  "request",
		},
		{
			name:          "Message with quotes",
			line:          `[2024-01-15 10:23:45] app.INFO: Processing order "12345" for customer {} []`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   `Processing order "12345" for customer`,
			expectedChan:  "app",
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
			line:        `[] request.INFO: Test message [] []`,
			expectError: true,
		},
		{
			name:        "Invalid format - wrong date format",
			line:        `[15-01-2024 10:23:45] request.INFO: Test message [] []`,
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

			if entry.Type != models.LogTypePrestaShop {
				t.Errorf("Parse(%q): type = %v, want %v", tt.line, entry.Type, models.LogTypePrestaShop)
			}
		})
	}
}

// TestPrestaShopParser_ParseTimestamp tests timestamp parsing.
func TestPrestaShopParser_ParseTimestamp(t *testing.T) {
	parser := NewPrestaShopParser(nil)

	tests := []struct {
		name           string
		line           string
		expectedYear   int
		expectedMonth  time.Month
		expectedDay    int
		expectedHour   int
		expectedMinute int
		expectedSecond int
	}{
		{
			name:           "standard timestamp",
			line:           `[2024-01-15 10:23:45] request.INFO: Test message [] []`,
			expectedYear:   2024,
			expectedMonth:  time.January,
			expectedDay:    15,
			expectedHour:   10,
			expectedMinute: 23,
			expectedSecond: 45,
		},
		{
			name:           "midnight timestamp",
			line:           `[2024-12-31 00:00:00] request.INFO: New Year's Eve [] []`,
			expectedYear:   2024,
			expectedMonth:  time.December,
			expectedDay:    31,
			expectedHour:   0,
			expectedMinute: 0,
			expectedSecond: 0,
		},
		{
			name:           "end of day timestamp",
			line:           `[2024-06-15 23:59:59] request.INFO: End of day [] []`,
			expectedYear:   2024,
			expectedMonth:  time.June,
			expectedDay:    15,
			expectedHour:   23,
			expectedMinute: 59,
			expectedSecond: 59,
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

// TestPrestaShopParser_ParseContext tests parsing of context JSON.
func TestPrestaShopParser_ParseContext(t *testing.T) {
	parser := NewPrestaShopParser(nil)

	tests := []struct {
		name          string
		line          string
		expectContext bool
		expectURI     string
		expectMethod  string
	}{
		{
			name:          "with URI and method",
			line:          `[2024-01-15 10:23:45] request.INFO: Matched route {"uri":"/admin/products","method":"GET"} []`,
			expectContext: true,
			expectURI:     "/admin/products",
			expectMethod:  "GET",
		},
		{
			name:          "with only URI",
			line:          `[2024-01-15 10:23:45] request.ERROR: Request failed {"uri":"/checkout"} []`,
			expectContext: true,
			expectURI:     "/checkout",
		},
		{
			name:          "empty context",
			line:          `[2024-01-15 10:23:45] app.INFO: Simple message {} []`,
			expectContext: false,
		},
		{
			name:          "no context (empty braces)",
			line:          `[2024-01-15 10:23:45] app.INFO: Message [] []`,
			expectContext: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", tt.line, err)
			}

			_, hasContext := entry.GetField("context")

			if tt.expectContext && !hasContext {
				t.Errorf("expected context, but none found")
				return
			}

			if tt.expectURI != "" {
				uri := entry.GetFieldString("uri")
				if uri != tt.expectURI {
					t.Errorf("uri = %q, want %q", uri, tt.expectURI)
				}
			}

			if tt.expectMethod != "" {
				method := entry.GetFieldString("method")
				if method != tt.expectMethod {
					t.Errorf("method = %q, want %q", method, tt.expectMethod)
				}
			}
		})
	}
}

// TestPrestaShopParser_ParseMultiLine tests multiline parsing for stack traces.
func TestPrestaShopParser_ParseMultiLine(t *testing.T) {
	parser := NewPrestaShopParser(nil)

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
				`[2024-01-15 10:23:45] request.INFO: Simple message [] []`,
			},
			expectError:       false,
			expectedLevel:     models.LevelInfo,
			expectedMsg:       "Simple message",
			expectedMultiline: false,
		},
		{
			name: "exception with stack trace",
			lines: []string{
				`[2024-01-15 10:23:45] request.CRITICAL: Uncaught PHP Exception Doctrine\DBAL\Exception {} []`,
				`#0 /var/www/prestashop/vendor/doctrine/dbal/lib/Doctrine/DBAL/Connection.php(456): connect()`,
				`#1 /var/www/prestashop/src/Adapter/Database.php(789): Doctrine\DBAL\Connection->query()`,
				`#2 /var/www/prestashop/src/Controller/AdminController.php(12): Adapter\Database->fetchAll()`,
				`#3 {main}`,
			},
			expectError:        false,
			expectedLevel:      models.LevelFatal,
			expectedMsg:        "Uncaught PHP Exception Doctrine\\DBAL\\Exception",
			expectedFrameCount: 4,
			expectedMultiline:  true,
		},
		{
			name: "error with short stack trace",
			lines: []string{
				`[2024-01-15 10:23:45] request.ERROR: Error occurred {} []`,
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
				`[2024-01-15 10:23:45] request.CRITICAL: Outer exception {} []`,
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

// TestPrestaShopParser_IsStartOfEntry tests detection of log entry start.
func TestPrestaShopParser_IsStartOfEntry(t *testing.T) {
	parser := NewPrestaShopParser(nil)

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "valid start - INFO",
			line:     `[2024-01-15 10:23:45] request.INFO: Message [] []`,
			expected: true,
		},
		{
			name:     "valid start - ERROR",
			line:     `[2024-01-15 10:23:45] request.ERROR: Error [] []`,
			expected: true,
		},
		{
			name:     "valid start - different timestamp",
			line:     `[2000-12-31 23:59:59] console.DEBUG: Debug [] []`,
			expected: true,
		},
		{
			name:     "stack trace line",
			line:     `#0 /var/www/prestashop/src/Controller/File.php(123): method()`,
			expected: false,
		},
		{
			name:     "continuation line",
			line:     `    at PrestaShop\Framework\App\Http->launch()`,
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
			line:     `[15-01-2024 10:23:45] request.INFO: Message [] []`,
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

// TestPrestaShopParser_CanParse tests auto-detection capability.
func TestPrestaShopParser_CanParse(t *testing.T) {
	parser := NewPrestaShopParser(nil)

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "valid PrestaShop log",
			line:     `[2024-01-15 10:23:45] request.INFO: Message [] []`,
			expected: true,
		},
		{
			name:     "valid PrestaShop error log",
			line:     `[2024-01-15 10:23:45] request.ERROR: Error occurred {"exception":"test"} []`,
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

// TestPrestaShopParser_Name tests the parser name.
func TestPrestaShopParser_Name(t *testing.T) {
	parser := NewPrestaShopParser(nil)
	expected := "prestashop"
	if parser.Name() != expected {
		t.Errorf("Name() = %q, want %q", parser.Name(), expected)
	}
}

// TestPrestaShopParser_Type tests the parser type.
func TestPrestaShopParser_Type(t *testing.T) {
	parser := NewPrestaShopParser(nil)
	expected := models.LogTypePrestaShop
	if parser.Type() != expected {
		t.Errorf("Type() = %v, want %v", parser.Type(), expected)
	}
}

// TestPrestaShopParser_Options tests parser options application.
func TestPrestaShopParser_Options(t *testing.T) {
	t.Run("include raw line", func(t *testing.T) {
		opts := &Options{
			IncludeRaw: true,
		}
		parser := NewPrestaShopParser(opts)
		line := `[2024-01-15 10:23:45] request.INFO: Test message [] []`
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
		parser := NewPrestaShopParser(opts)
		line := `[2024-01-15 10:23:45] request.INFO: Test message [] []`
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
			Source: "prestashop-server-1",
		}
		parser := NewPrestaShopParser(opts)
		line := `[2024-01-15 10:23:45] request.INFO: Test message [] []`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Source != "prestashop-server-1" {
			t.Errorf("Source = %q, want %q", entry.Source, "prestashop-server-1")
		}
	})

	t.Run("with labels", func(t *testing.T) {
		opts := &Options{
			Labels: map[string]string{
				"environment": "production",
				"app":         "prestashop",
			},
		}
		parser := NewPrestaShopParser(opts)
		line := `[2024-01-15 10:23:45] request.INFO: Test message [] []`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.GetLabel("environment") != "production" {
			t.Errorf("Label[environment] = %q, want %q", entry.GetLabel("environment"), "production")
		}
		if entry.GetLabel("app") != "prestashop" {
			t.Errorf("Label[app] = %q, want %q", entry.GetLabel("app"), "prestashop")
		}
	})
}

// TestPrestaShopParser_RealWorldLogs tests with realistic PrestaShop log examples.
func TestPrestaShopParser_RealWorldLogs(t *testing.T) {
	parser := NewPrestaShopParser(nil)

	tests := []struct {
		name          string
		line          string
		expectedLevel models.LogLevel
		expectedChan  string
	}{
		{
			name:          "database connection error",
			line:          `[2024-01-15 10:23:45] request.CRITICAL: Uncaught PHP Exception Doctrine\DBAL\Exception: "An exception occurred while establishing a connection" {} []`,
			expectedLevel: models.LevelFatal,
			expectedChan:  "request",
		},
		{
			name:          "request matched route",
			line:          `[2024-01-15 10:23:45] request.INFO: Matched route "admin_products_index" [] []`,
			expectedLevel: models.LevelInfo,
			expectedChan:  "request",
		},
		{
			name:          "security authentication",
			line:          `[2024-01-15 10:23:45] security.INFO: User "admin@example.com" has been authenticated successfully [] []`,
			expectedLevel: models.LevelInfo,
			expectedChan:  "security",
		},
		{
			name:          "console command",
			line:          `[2024-01-15 10:23:45] console.INFO: Command "prestashop:update:database" finished successfully [] []`,
			expectedLevel: models.LevelInfo,
			expectedChan:  "console",
		},
		{
			name:          "php warning",
			line:          `[2024-01-15 10:23:45] php.WARNING: Undefined array key "product_id" {} []`,
			expectedLevel: models.LevelWarning,
			expectedChan:  "php",
		},
		{
			name:          "doctrine query",
			line:          `[2024-01-15 10:23:45] doctrine.DEBUG: SELECT * FROM ps_product WHERE id_product = 1 [] []`,
			expectedLevel: models.LevelDebug,
			expectedChan:  "doctrine",
		},
		{
			name:          "cache clear",
			line:          `[2024-01-15 10:23:45] cache.INFO: Cache cleared successfully [] []`,
			expectedLevel: models.LevelInfo,
			expectedChan:  "cache",
		},
		{
			name:          "translation warning",
			line:          `[2024-01-15 10:23:45] translation.WARNING: Translation not found for key "admin.dashboard.title" in locale "fr" [] []`,
			expectedLevel: models.LevelWarning,
			expectedChan:  "translation",
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

// TestPrestaShopLevelToLogLevel tests level conversion.
func TestPrestaShopLevelToLogLevel(t *testing.T) {
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
			result := prestashopLevelToLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("prestashopLevelToLogLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestPrestaShopParser_SymfonyChannels tests various Symfony channel names.
func TestPrestaShopParser_SymfonyChannels(t *testing.T) {
	parser := NewPrestaShopParser(nil)

	channels := []string{
		"request",
		"security",
		"console",
		"doctrine",
		"php",
		"cache",
		"translation",
		"deprecation",
		"event",
		"router",
		"profiler",
		"app",
	}

	for _, channel := range channels {
		t.Run(channel, func(t *testing.T) {
			line := `[2024-01-15 10:23:45] ` + channel + `.INFO: Test message [] []`
			entry, err := parser.Parse(line)
			if err != nil {
				t.Fatalf("Parse: unexpected error: %v", err)
			}

			if entry.GetFieldString("channel") != channel {
				t.Errorf("channel = %q, want %q", entry.GetFieldString("channel"), channel)
			}
		})
	}
}

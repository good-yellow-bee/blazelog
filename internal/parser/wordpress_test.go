package parser

import (
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// TestWordPressParser_Parse tests the WordPress log parser.
func TestWordPressParser_Parse(t *testing.T) {
	parser := NewWordPressParser(nil)

	tests := []struct {
		name          string
		line          string
		expectError   bool
		expectedLevel models.LogLevel
		expectedMsg   string
		expectedType  string
	}{
		{
			name:          "PHP Notice",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Undefined variable: foo in /var/www/html/wp-content/plugins/test/test.php on line 123`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "Undefined variable: foo in /var/www/html/wp-content/plugins/test/test.php on line 123",
			expectedType:  "php",
		},
		{
			name:          "PHP Warning",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Warning:  array_merge(): Expected parameter 1 to be an array in /var/www/html/wp-includes/functions.php on line 456`,
			expectError:   false,
			expectedLevel: models.LevelWarning,
			expectedMsg:   "array_merge(): Expected parameter 1 to be an array in /var/www/html/wp-includes/functions.php on line 456",
			expectedType:  "php",
		},
		{
			name:          "PHP Error",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Error:  Call to undefined function custom_function() in /var/www/html/wp-content/themes/theme/functions.php on line 78`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "Call to undefined function custom_function() in /var/www/html/wp-content/themes/theme/functions.php on line 78",
			expectedType:  "php",
		},
		{
			name:          "PHP Fatal error",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Uncaught Error: Class 'WP_Widget' not found in /var/www/html/wp-content/plugins/test/widget.php on line 10`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "Uncaught Error: Class 'WP_Widget' not found in /var/www/html/wp-content/plugins/test/widget.php on line 10",
			expectedType:  "php",
		},
		{
			name:          "PHP Parse error",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Parse error:  syntax error, unexpected '}' in /var/www/html/wp-content/plugins/broken.php on line 50`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "syntax error, unexpected '}' in /var/www/html/wp-content/plugins/broken.php on line 50",
			expectedType:  "php",
		},
		{
			name:          "PHP Deprecated",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Deprecated:  Function create_function() is deprecated in /var/www/html/wp-content/plugins/old-plugin.php on line 200`,
			expectError:   false,
			expectedLevel: models.LevelWarning,
			expectedMsg:   "Function create_function() is deprecated in /var/www/html/wp-content/plugins/old-plugin.php on line 200",
			expectedType:  "php",
		},
		{
			name:          "PHP Strict Standards",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Strict Standards:  Declaration of Child::method() should be compatible with Parent::method() in /var/www/html/wp-content/plugins/plugin.php on line 30`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "Declaration of Child::method() should be compatible with Parent::method() in /var/www/html/wp-content/plugins/plugin.php on line 30",
			expectedType:  "php",
		},
		{
			name:          "PHP Catchable fatal error",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Catchable fatal error:  Argument 1 passed to test() must be an array, string given in /var/www/html/test.php on line 5`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "Argument 1 passed to test() must be an array, string given in /var/www/html/test.php on line 5",
			expectedType:  "php",
		},
		{
			name:          "WordPress database error",
			line:          `[15-Jan-2024 10:23:45 UTC] WordPress database error Table 'wp_posts' doesn't exist for query SELECT * FROM wp_posts`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "WordPress database error Table 'wp_posts' doesn't exist for query SELECT * FROM wp_posts",
			expectedType:  "wordpress_database",
		},
		{
			name:          "WordPress generic message",
			line:          `[15-Jan-2024 10:23:45 UTC] WordPress cron job executed successfully`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "WordPress cron job executed successfully",
			expectedType:  "wordpress",
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
			line:        `[] PHP Notice: Test message`,
			expectError: true,
		},
		{
			name:        "Invalid format - wrong date format",
			line:        `[2024-01-15 10:23:45] PHP Notice: Test message`,
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

			if entry.GetFieldString("source_type") != tt.expectedType {
				t.Errorf("Parse(%q): source_type = %q, want %q", tt.line, entry.GetFieldString("source_type"), tt.expectedType)
			}

			if entry.Type != models.LogTypeWordPress {
				t.Errorf("Parse(%q): type = %v, want %v", tt.line, entry.Type, models.LogTypeWordPress)
			}
		})
	}
}

// TestWordPressParser_ParseTimestamp tests timestamp parsing.
func TestWordPressParser_ParseTimestamp(t *testing.T) {
	parser := NewWordPressParser(nil)

	tests := []struct {
		name           string
		line           string
		expectedYear   int
		expectedMonth  time.Month
		expectedDay    int
		expectedHour   int
		expectedMinute int
		expectedSecond int
		expectedTZ     string
	}{
		{
			name:           "standard timestamp UTC",
			line:           `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Test message`,
			expectedYear:   2024,
			expectedMonth:  time.January,
			expectedDay:    15,
			expectedHour:   10,
			expectedMinute: 23,
			expectedSecond: 45,
			expectedTZ:     "UTC",
		},
		{
			name:           "midnight timestamp",
			line:           `[31-Dec-2024 00:00:00 UTC] PHP Warning:  New Year's Eve warning`,
			expectedYear:   2024,
			expectedMonth:  time.December,
			expectedDay:    31,
			expectedHour:   0,
			expectedMinute: 0,
			expectedSecond: 0,
			expectedTZ:     "UTC",
		},
		{
			name:           "end of day timestamp",
			line:           `[15-Jun-2024 23:59:59 PST] PHP Error:  End of day error`,
			expectedYear:   2024,
			expectedMonth:  time.June,
			expectedDay:    15,
			expectedHour:   23,
			expectedMinute: 59,
			expectedSecond: 59,
			expectedTZ:     "PST",
		},
		{
			name:           "different month Feb",
			line:           `[28-Feb-2024 12:30:00 EST] PHP Notice:  February notice`,
			expectedYear:   2024,
			expectedMonth:  time.February,
			expectedDay:    28,
			expectedHour:   12,
			expectedMinute: 30,
			expectedSecond: 0,
			expectedTZ:     "EST",
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
			if entry.GetFieldString("timezone") != tt.expectedTZ {
				t.Errorf("timezone = %q, want %q", entry.GetFieldString("timezone"), tt.expectedTZ)
			}
		})
	}
}

// TestWordPressParser_ParsePHPLocation tests extraction of file and line info.
func TestWordPressParser_ParsePHPLocation(t *testing.T) {
	parser := NewWordPressParser(nil)

	tests := []struct {
		name         string
		line         string
		expectedFile string
		expectedLine string
	}{
		{
			name:         "standard location format",
			line:         `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Undefined variable in /var/www/html/wp-content/plugins/test.php on line 123`,
			expectedFile: "/var/www/html/wp-content/plugins/test.php",
			expectedLine: "123",
		},
		{
			name:         "location with colon format",
			line:         `[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Error in /var/www/test.php:456`,
			expectedFile: "/var/www/test.php",
			expectedLine: "456",
		},
		{
			name:         "windows-style path",
			line:         `[15-Jan-2024 10:23:45 UTC] PHP Warning:  Warning in C:/xampp/htdocs/wordpress/wp-content/themes/theme/functions.php on line 78`,
			expectedFile: "C:/xampp/htdocs/wordpress/wp-content/themes/theme/functions.php",
			expectedLine: "78",
		},
		{
			name:         "no location info",
			line:         `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Some general notice without location`,
			expectedFile: "",
			expectedLine: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if err != nil {
				t.Fatalf("Parse(%q): unexpected error: %v", tt.line, err)
			}

			phpFile := entry.GetFieldString("php_file")
			if phpFile != tt.expectedFile {
				t.Errorf("php_file = %q, want %q", phpFile, tt.expectedFile)
			}

			phpLine := entry.GetFieldString("php_line")
			if phpLine != tt.expectedLine {
				t.Errorf("php_line = %q, want %q", phpLine, tt.expectedLine)
			}
		})
	}
}

// TestWordPressParser_ParseMultiLine tests multiline parsing for stack traces.
func TestWordPressParser_ParseMultiLine(t *testing.T) {
	parser := NewWordPressParser(nil)

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
				`[15-Jan-2024 10:23:45 UTC] PHP Notice:  Simple notice message`,
			},
			expectError:       false,
			expectedLevel:     models.LevelInfo,
			expectedMsg:       "Simple notice message",
			expectedMultiline: false,
		},
		{
			name: "fatal error with stack trace",
			lines: []string{
				`[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Uncaught Exception: Something went wrong in /var/www/html/wp-content/plugins/test/test.php:50`,
				`Stack trace:`,
				`#0 /var/www/html/wp-includes/plugin.php(525): call_user_func_array()`,
				`#1 /var/www/html/wp-includes/class-wp-hook.php(307): do_action()`,
				`#2 /var/www/html/wp-content/plugins/test/test.php(100): WP_Hook->do_action()`,
				`#3 {main}`,
			},
			expectError:        false,
			expectedLevel:      models.LevelFatal,
			expectedMsg:        "Uncaught Exception: Something went wrong in /var/www/html/wp-content/plugins/test/test.php:50",
			expectedFrameCount: 4,
			expectedMultiline:  true,
		},
		{
			name: "exception with thrown message",
			lines: []string{
				`[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Uncaught Error: Call to undefined method Test::missing()`,
				`  thrown in /var/www/html/wp-content/plugins/plugin.php on line 25`,
			},
			expectError:        false,
			expectedLevel:      models.LevelFatal,
			expectedMsg:        "Uncaught Error: Call to undefined method Test::missing()",
			expectedFrameCount: 0, // No numbered frames
			expectedMultiline:  true,
		},
		{
			name: "error with short stack trace",
			lines: []string{
				`[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Error occurred`,
				`#0 /path/to/file.php(10): function()`,
				`#1 {main}`,
			},
			expectError:        false,
			expectedLevel:      models.LevelFatal,
			expectedMsg:        "Error occurred",
			expectedFrameCount: 2,
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

// TestWordPressParser_IsStartOfEntry tests detection of log entry start.
func TestWordPressParser_IsStartOfEntry(t *testing.T) {
	parser := NewWordPressParser(nil)

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "valid start - PHP Notice",
			line:     `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Test`,
			expected: true,
		},
		{
			name:     "valid start - PHP Fatal error",
			line:     `[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Error`,
			expected: true,
		},
		{
			name:     "valid start - WordPress message",
			line:     `[15-Jan-2024 10:23:45 UTC] WordPress database error`,
			expected: true,
		},
		{
			name:     "valid start - different timestamp",
			line:     `[01-Dec-2000 23:59:59 PST] PHP Warning:  Warning`,
			expected: true,
		},
		{
			name:     "stack trace line",
			line:     `#0 /var/www/html/wp-includes/plugin.php(525): call_user_func()`,
			expected: false,
		},
		{
			name:     "continuation line",
			line:     `  thrown in /var/www/html/wp-content/plugins/test.php on line 25`,
			expected: false,
		},
		{
			name:     "Stack trace: label",
			line:     `Stack trace:`,
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
			name:     "Monolog format (not WordPress)",
			line:     `[2024-01-15 10:23:45] request.INFO: Message [] []`,
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

// TestWordPressParser_CanParse tests auto-detection capability.
func TestWordPressParser_CanParse(t *testing.T) {
	parser := NewWordPressParser(nil)

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "valid WordPress PHP Notice",
			line:     `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Test`,
			expected: true,
		},
		{
			name:     "valid WordPress PHP Fatal error",
			line:     `[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Error occurred`,
			expected: true,
		},
		{
			name:     "valid WordPress database error",
			line:     `[15-Jan-2024 10:23:45 UTC] WordPress database error Table not found`,
			expected: true,
		},
		{
			name:     "valid WordPress message",
			line:     `[15-Jan-2024 10:23:45 UTC] WordPress cron job executed`,
			expected: true,
		},
		{
			name:     "PrestaShop/Magento log (Monolog format)",
			line:     `[2024-01-15 10:23:45] request.INFO: Message [] []`,
			expected: false,
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
		{
			name:     "timestamp only but not PHP/WordPress",
			line:     `[15-Jan-2024 10:23:45 UTC] Some other message`,
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

// TestWordPressParser_Name tests the parser name.
func TestWordPressParser_Name(t *testing.T) {
	parser := NewWordPressParser(nil)
	expected := "wordpress"
	if parser.Name() != expected {
		t.Errorf("Name() = %q, want %q", parser.Name(), expected)
	}
}

// TestWordPressParser_Type tests the parser type.
func TestWordPressParser_Type(t *testing.T) {
	parser := NewWordPressParser(nil)
	expected := models.LogTypeWordPress
	if parser.Type() != expected {
		t.Errorf("Type() = %v, want %v", parser.Type(), expected)
	}
}

// TestWordPressParser_Options tests parser options application.
func TestWordPressParser_Options(t *testing.T) {
	t.Run("include raw line", func(t *testing.T) {
		opts := &ParserOptions{
			IncludeRaw: true,
		}
		parser := NewWordPressParser(opts)
		line := `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Test message`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Raw != line {
			t.Errorf("Raw = %q, want %q", entry.Raw, line)
		}
	})

	t.Run("exclude raw line", func(t *testing.T) {
		opts := &ParserOptions{
			IncludeRaw: false,
		}
		parser := NewWordPressParser(opts)
		line := `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Test message`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Raw != "" {
			t.Errorf("Raw = %q, want empty", entry.Raw)
		}
	})

	t.Run("with source", func(t *testing.T) {
		opts := &ParserOptions{
			Source: "wordpress-server-1",
		}
		parser := NewWordPressParser(opts)
		line := `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Test message`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Source != "wordpress-server-1" {
			t.Errorf("Source = %q, want %q", entry.Source, "wordpress-server-1")
		}
	})

	t.Run("with labels", func(t *testing.T) {
		opts := &ParserOptions{
			Labels: map[string]string{
				"environment": "production",
				"app":         "wordpress",
			},
		}
		parser := NewWordPressParser(opts)
		line := `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Test message`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.GetLabel("environment") != "production" {
			t.Errorf("Label[environment] = %q, want %q", entry.GetLabel("environment"), "production")
		}
		if entry.GetLabel("app") != "wordpress" {
			t.Errorf("Label[app] = %q, want %q", entry.GetLabel("app"), "wordpress")
		}
	})
}

// TestWordPressParser_RealWorldLogs tests with realistic WordPress log examples.
func TestWordPressParser_RealWorldLogs(t *testing.T) {
	parser := NewWordPressParser(nil)

	tests := []struct {
		name          string
		line          string
		expectedLevel models.LogLevel
		expectedType  string
	}{
		{
			name:          "WooCommerce undefined index",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Notice:  Undefined index: product_id in /var/www/html/wp-content/plugins/woocommerce/includes/class-wc-cart.php on line 567`,
			expectedLevel: models.LevelInfo,
			expectedType:  "php",
		},
		{
			name:          "Elementor deprecated function",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Deprecated:  Function Elementor\Core\Files\CSS\Base::get_css() is deprecated since version 2.1.0! in /var/www/html/wp-includes/functions.php on line 5379`,
			expectedLevel: models.LevelWarning,
			expectedType:  "php",
		},
		{
			name:          "WordPress database connection error",
			line:          `[15-Jan-2024 10:23:45 UTC] WordPress database error Error establishing a database connection`,
			expectedLevel: models.LevelError,
			expectedType:  "wordpress_database",
		},
		{
			name:          "Plugin activation fatal error",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Uncaught Error: Call to undefined function activate_plugin() in /var/www/html/wp-content/plugins/my-plugin/my-plugin.php:45`,
			expectedLevel: models.LevelFatal,
			expectedType:  "php",
		},
		{
			name:          "Theme file parse error",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Parse error:  syntax error, unexpected 'echo' (T_ECHO), expecting ',' or ';' in /var/www/html/wp-content/themes/my-theme/header.php on line 12`,
			expectedLevel: models.LevelFatal,
			expectedType:  "php",
		},
		{
			name:          "Memory limit exceeded",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Allowed memory size of 268435456 bytes exhausted (tried to allocate 20480 bytes) in /var/www/html/wp-includes/wp-db.php on line 1934`,
			expectedLevel: models.LevelFatal,
			expectedType:  "php",
		},
		{
			name:          "Max execution time exceeded",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Maximum execution time of 30 seconds exceeded in /var/www/html/wp-content/plugins/import-plugin/import.php on line 234`,
			expectedLevel: models.LevelFatal,
			expectedType:  "php",
		},
		{
			name:          "Class not found",
			line:          `[15-Jan-2024 10:23:45 UTC] PHP Fatal error:  Uncaught Error: Class 'WP_REST_Request' not found in /var/www/html/wp-content/plugins/api-plugin/api.php on line 15`,
			expectedLevel: models.LevelFatal,
			expectedType:  "php",
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

			if entry.GetFieldString("source_type") != tt.expectedType {
				t.Errorf("source_type = %q, want %q", entry.GetFieldString("source_type"), tt.expectedType)
			}
		})
	}
}

// TestWordPressPHPLevelParsing tests PHP level parsing.
func TestWordPressPHPLevelParsing(t *testing.T) {
	tests := []struct {
		input       string
		level       models.LogLevel
		cleanedMsg  string
	}{
		{"Fatal error:  Test message", models.LevelFatal, "Test message"},
		{"Parse error:  Test message", models.LevelFatal, "Test message"},
		{"Catchable fatal error:  Test message", models.LevelFatal, "Test message"},
		{"Error:  Test message", models.LevelError, "Test message"},
		{"Warning:  Test message", models.LevelWarning, "Test message"},
		{"Notice:  Test message", models.LevelInfo, "Test message"},
		{"Strict Standards:  Test message", models.LevelInfo, "Test message"},
		{"Deprecated:  Test message", models.LevelWarning, "Test message"},
		{"Unknown level message", models.LevelUnknown, "Unknown level message"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, msg := parseWordPressPHPLevel(tt.input)
			if level != tt.level {
				t.Errorf("parseWordPressPHPLevel(%q) level = %v, want %v", tt.input, level, tt.level)
			}
			if msg != tt.cleanedMsg {
				t.Errorf("parseWordPressPHPLevel(%q) msg = %q, want %q", tt.input, msg, tt.cleanedMsg)
			}
		})
	}
}

// TestWordPressParser_Timezones tests various timezone parsing.
func TestWordPressParser_Timezones(t *testing.T) {
	parser := NewWordPressParser(nil)

	timezones := []string{
		"UTC",
		"PST",
		"EST",
		"CST",
		"MST",
		"PDT",
		"EDT",
		"CDT",
		"MDT",
	}

	for _, tz := range timezones {
		t.Run(tz, func(t *testing.T) {
			line := `[15-Jan-2024 10:23:45 ` + tz + `] PHP Notice:  Test message`
			entry, err := parser.Parse(line)
			if err != nil {
				t.Fatalf("Parse: unexpected error: %v", err)
			}

			if entry.GetFieldString("timezone") != tz {
				t.Errorf("timezone = %q, want %q", entry.GetFieldString("timezone"), tz)
			}
		})
	}
}

// TestWordPressParser_MonthParsing tests all month abbreviations.
func TestWordPressParser_MonthParsing(t *testing.T) {
	parser := NewWordPressParser(nil)

	months := []struct {
		abbr     string
		expected time.Month
	}{
		{"Jan", time.January},
		{"Feb", time.February},
		{"Mar", time.March},
		{"Apr", time.April},
		{"May", time.May},
		{"Jun", time.June},
		{"Jul", time.July},
		{"Aug", time.August},
		{"Sep", time.September},
		{"Oct", time.October},
		{"Nov", time.November},
		{"Dec", time.December},
	}

	for _, m := range months {
		t.Run(m.abbr, func(t *testing.T) {
			line := `[15-` + m.abbr + `-2024 10:23:45 UTC] PHP Notice:  Test`
			entry, err := parser.Parse(line)
			if err != nil {
				t.Fatalf("Parse: unexpected error: %v", err)
			}

			if entry.Timestamp.Month() != m.expected {
				t.Errorf("month = %v, want %v", entry.Timestamp.Month(), m.expected)
			}
		})
	}
}

package parser

import (
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// TestNginxAccessParser_Parse tests the Nginx access log parser.
func TestNginxAccessParser_Parse(t *testing.T) {
	parser := NewNginxAccessParser(nil)

	tests := []struct {
		name           string
		line           string
		expectError    bool
		expectedLevel  models.LogLevel
		expectedStatus int
		expectedMethod string
		expectedURI    string
	}{
		{
			name:           "combined format - successful GET",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326 "http://example.com/" "Mozilla/5.0"`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 200,
			expectedMethod: "GET",
			expectedURI:    "/index.html",
		},
		{
			name:           "combined format - POST with auth user",
			line:           `192.168.1.1 - admin [10/Oct/2024:13:55:36 -0700] "POST /api/users HTTP/1.1" 201 156 "-" "curl/7.64.1"`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 201,
			expectedMethod: "POST",
			expectedURI:    "/api/users",
		},
		{
			name:           "combined format - 404 error",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /missing.html HTTP/1.1" 404 162 "-" "Mozilla/5.0"`,
			expectError:    false,
			expectedLevel:  models.LevelWarning,
			expectedStatus: 404,
			expectedMethod: "GET",
			expectedURI:    "/missing.html",
		},
		{
			name:           "combined format - 500 server error",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /error HTTP/1.1" 500 0 "-" "Mozilla/5.0"`,
			expectError:    false,
			expectedLevel:  models.LevelError,
			expectedStatus: 500,
			expectedMethod: "GET",
			expectedURI:    "/error",
		},
		{
			name:           "combined format - 503 service unavailable",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /api HTTP/1.1" 503 0 "-" "curl/7.64.1"`,
			expectError:    false,
			expectedLevel:  models.LevelError,
			expectedStatus: 503,
			expectedMethod: "GET",
			expectedURI:    "/api",
		},
		{
			name:           "common format - no referer/user-agent",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 200,
			expectedMethod: "GET",
			expectedURI:    "/index.html",
		},
		{
			name:           "PUT request",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "PUT /api/resource HTTP/1.1" 200 100 "-" "curl/7.64.1"`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 200,
			expectedMethod: "PUT",
			expectedURI:    "/api/resource",
		},
		{
			name:           "DELETE request",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "DELETE /api/resource/123 HTTP/1.1" 204 0 "-" "curl/7.64.1"`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 204,
			expectedMethod: "DELETE",
			expectedURI:    "/api/resource/123",
		},
		{
			name:           "URL with query string",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /search?q=test&page=1 HTTP/1.1" 200 1500 "-" "Mozilla/5.0"`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 200,
			expectedMethod: "GET",
			expectedURI:    "/search?q=test&page=1",
		},
		{
			name:           "301 redirect",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /old-page HTTP/1.1" 301 0 "-" "Mozilla/5.0"`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 301,
			expectedMethod: "GET",
			expectedURI:    "/old-page",
		},
		{
			name:           "401 unauthorized",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /admin HTTP/1.1" 401 0 "-" "Mozilla/5.0"`,
			expectError:    false,
			expectedLevel:  models.LevelWarning,
			expectedStatus: 401,
			expectedMethod: "GET",
			expectedURI:    "/admin",
		},
		{
			name:        "invalid format",
			line:        "this is not a valid log line",
			expectError: true,
		},
		{
			name:        "empty line",
			line:        "",
			expectError: true,
		},
		{
			name:        "partial log line",
			line:        `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700]`,
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
				t.Errorf("Parse(%q): expected level %v, got %v", tt.line, tt.expectedLevel, entry.Level)
			}

			if entry.GetFieldInt("status") != tt.expectedStatus {
				t.Errorf("Parse(%q): expected status %d, got %d", tt.line, tt.expectedStatus, entry.GetFieldInt("status"))
			}

			if entry.GetFieldString("method") != tt.expectedMethod {
				t.Errorf("Parse(%q): expected method %s, got %s", tt.line, tt.expectedMethod, entry.GetFieldString("method"))
			}

			if entry.GetFieldString("request_uri") != tt.expectedURI {
				t.Errorf("Parse(%q): expected URI %s, got %s", tt.line, tt.expectedURI, entry.GetFieldString("request_uri"))
			}

			if entry.Type != models.LogTypeNginx {
				t.Errorf("Parse(%q): expected type %v, got %v", tt.line, models.LogTypeNginx, entry.Type)
			}
		})
	}
}

// TestNginxAccessParser_CombinedFields tests that combined format extracts all fields.
func TestNginxAccessParser_CombinedFields(t *testing.T) {
	parser := NewNginxAccessParser(nil)
	line := `192.168.1.1 - john [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326 "http://example.com/" "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if entry.GetFieldString("remote_addr") != "192.168.1.1" {
		t.Errorf("Expected remote_addr '192.168.1.1', got '%s'", entry.GetFieldString("remote_addr"))
	}

	if entry.GetFieldString("remote_user") != "john" {
		t.Errorf("Expected remote_user 'john', got '%s'", entry.GetFieldString("remote_user"))
	}

	if entry.GetFieldString("protocol") != "HTTP/1.1" {
		t.Errorf("Expected protocol 'HTTP/1.1', got '%s'", entry.GetFieldString("protocol"))
	}

	if entry.GetFieldInt("body_bytes_sent") != 2326 {
		t.Errorf("Expected body_bytes_sent 2326, got %d", entry.GetFieldInt("body_bytes_sent"))
	}

	if entry.GetFieldString("http_referer") != "http://example.com/" {
		t.Errorf("Expected http_referer 'http://example.com/', got '%s'", entry.GetFieldString("http_referer"))
	}

	if entry.GetFieldString("http_user_agent") != "Mozilla/5.0 (Windows NT 10.0; Win64; x64)" {
		t.Errorf("Expected http_user_agent, got '%s'", entry.GetFieldString("http_user_agent"))
	}

	// Verify timestamp parsing
	expectedTime := time.Date(2024, 10, 10, 13, 55, 36, 0, time.FixedZone("", -7*60*60))
	if !entry.Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, entry.Timestamp)
	}
}

// TestNginxAccessParser_CanParse tests auto-detection.
func TestNginxAccessParser_CanParse(t *testing.T) {
	parser := NewNginxAccessParser(nil)

	tests := []struct {
		line     string
		expected bool
	}{
		{`192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326 "-" "Mozilla/5.0"`, true},
		{`192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326`, true},
		{`2024/10/10 13:55:36 [error] 12345#67890: test message`, false},
		{"invalid line", false},
		{"", false},
	}

	for _, tt := range tests {
		got := parser.CanParse(tt.line)
		if got != tt.expected {
			t.Errorf("CanParse(%q): expected %v, got %v", tt.line, tt.expected, got)
		}
	}
}

// TestNginxAccessParser_Name tests the parser name.
func TestNginxAccessParser_Name(t *testing.T) {
	parser := NewNginxAccessParser(nil)
	if parser.Name() != "nginx-access" {
		t.Errorf("Expected name 'nginx-access', got '%s'", parser.Name())
	}
}

// TestNginxAccessParser_Type tests the parser type.
func TestNginxAccessParser_Type(t *testing.T) {
	parser := NewNginxAccessParser(nil)
	if parser.Type() != models.LogTypeNginx {
		t.Errorf("Expected type %v, got %v", models.LogTypeNginx, parser.Type())
	}
}

// TestNginxAccessParser_Options tests that parser options are applied.
func TestNginxAccessParser_Options(t *testing.T) {
	opts := &Options{
		IncludeRaw: true,
		Source:     "nginx-server-1",
		Labels: map[string]string{
			"env": "production",
		},
	}

	parser := NewNginxAccessParser(opts)
	line := `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326 "-" "Mozilla/5.0"`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if entry.Raw != line {
		t.Errorf("Expected raw line to be included")
	}

	if entry.Source != "nginx-server-1" {
		t.Errorf("Expected source 'nginx-server-1', got '%s'", entry.Source)
	}

	if entry.GetLabel("env") != "production" {
		t.Errorf("Expected label env='production', got '%s'", entry.GetLabel("env"))
	}
}

// TestNginxErrorParser_Parse tests the Nginx error log parser.
func TestNginxErrorParser_Parse(t *testing.T) {
	parser := NewNginxErrorParser(nil)

	tests := []struct {
		name          string
		line          string
		expectError   bool
		expectedLevel models.LogLevel
		expectedMsg   string
	}{
		{
			name:          "error with connection ID",
			line:          `2024/10/10 13:55:36 [error] 12345#67890: *123 open() "/var/www/html/missing.html" failed (2: No such file or directory), client: 192.168.1.1, server: example.com, request: "GET /missing.html HTTP/1.1"`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   `open() "/var/www/html/missing.html" failed (2: No such file or directory), client: 192.168.1.1, server: example.com, request: "GET /missing.html HTTP/1.1"`,
		},
		{
			name:          "notice without connection ID",
			line:          `2024/10/10 13:55:36 [notice] 12345#67890: signal process started`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "signal process started",
		},
		{
			name:          "warn level",
			line:          `2024/10/10 13:55:36 [warn] 12345#67890: *456 something suspicious`,
			expectError:   false,
			expectedLevel: models.LevelWarning,
			expectedMsg:   "something suspicious",
		},
		{
			name:          "debug level",
			line:          `2024/10/10 13:55:36 [debug] 12345#67890: debug message here`,
			expectError:   false,
			expectedLevel: models.LevelDebug,
			expectedMsg:   "debug message here",
		},
		{
			name:          "info level",
			line:          `2024/10/10 13:55:36 [info] 12345#67890: informational message`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "informational message",
		},
		{
			name:          "crit level",
			line:          `2024/10/10 13:55:36 [crit] 12345#67890: critical error occurred`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "critical error occurred",
		},
		{
			name:          "alert level",
			line:          `2024/10/10 13:55:36 [alert] 12345#67890: alert message`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "alert message",
		},
		{
			name:          "emerg level",
			line:          `2024/10/10 13:55:36 [emerg] 12345#67890: emergency situation`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "emergency situation",
		},
		{
			name:        "invalid format",
			line:        "this is not a valid error log line",
			expectError: true,
		},
		{
			name:        "empty line",
			line:        "",
			expectError: true,
		},
		{
			name:        "access log format (should fail)",
			line:        `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326`,
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
				t.Errorf("Parse(%q): expected level %v, got %v", tt.line, tt.expectedLevel, entry.Level)
			}

			if entry.Message != tt.expectedMsg {
				t.Errorf("Parse(%q): expected message %q, got %q", tt.line, tt.expectedMsg, entry.Message)
			}

			if entry.Type != models.LogTypeNginx {
				t.Errorf("Parse(%q): expected type %v, got %v", tt.line, models.LogTypeNginx, entry.Type)
			}
		})
	}
}

// TestNginxErrorParser_Fields tests that all fields are extracted correctly.
func TestNginxErrorParser_Fields(t *testing.T) {
	parser := NewNginxErrorParser(nil)
	line := `2024/10/10 13:55:36 [error] 12345#67890: *123 test error, client: 192.168.1.100, server: api.example.com`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if entry.GetFieldInt("pid") != 12345 {
		t.Errorf("Expected pid 12345, got %d", entry.GetFieldInt("pid"))
	}

	if entry.GetFieldInt("tid") != 67890 {
		t.Errorf("Expected tid 67890, got %d", entry.GetFieldInt("tid"))
	}

	if entry.GetFieldInt("cid") != 123 {
		t.Errorf("Expected cid 123, got %d", entry.GetFieldInt("cid"))
	}

	if entry.GetFieldString("client") != "192.168.1.100" {
		t.Errorf("Expected client '192.168.1.100', got '%s'", entry.GetFieldString("client"))
	}

	if entry.GetFieldString("server") != "api.example.com" {
		t.Errorf("Expected server 'api.example.com', got '%s'", entry.GetFieldString("server"))
	}

	if entry.GetFieldString("nginx_level") != "error" {
		t.Errorf("Expected nginx_level 'error', got '%s'", entry.GetFieldString("nginx_level"))
	}

	// Verify timestamp parsing
	expectedTime := time.Date(2024, 10, 10, 13, 55, 36, 0, time.UTC)
	if !entry.Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, entry.Timestamp)
	}
}

// TestNginxErrorParser_NoConnectionID tests parsing without connection ID.
func TestNginxErrorParser_NoConnectionID(t *testing.T) {
	parser := NewNginxErrorParser(nil)
	line := `2024/10/10 13:55:36 [notice] 12345#67890: signal 17 (SIGCHLD) received from 12346`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// CID should not be set
	if _, ok := entry.GetField("cid"); ok {
		t.Error("Expected cid to not be set")
	}
}

// TestNginxErrorParser_CanParse tests auto-detection.
func TestNginxErrorParser_CanParse(t *testing.T) {
	parser := NewNginxErrorParser(nil)

	tests := []struct {
		line     string
		expected bool
	}{
		{`2024/10/10 13:55:36 [error] 12345#67890: *123 test message`, true},
		{`2024/10/10 13:55:36 [notice] 12345#67890: signal process started`, true},
		{`192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326`, false},
		{"invalid line", false},
		{"", false},
	}

	for _, tt := range tests {
		got := parser.CanParse(tt.line)
		if got != tt.expected {
			t.Errorf("CanParse(%q): expected %v, got %v", tt.line, tt.expected, got)
		}
	}
}

// TestNginxErrorParser_Name tests the parser name.
func TestNginxErrorParser_Name(t *testing.T) {
	parser := NewNginxErrorParser(nil)
	if parser.Name() != "nginx-error" {
		t.Errorf("Expected name 'nginx-error', got '%s'", parser.Name())
	}
}

// TestNginxErrorParser_Type tests the parser type.
func TestNginxErrorParser_Type(t *testing.T) {
	parser := NewNginxErrorParser(nil)
	if parser.Type() != models.LogTypeNginx {
		t.Errorf("Expected type %v, got %v", models.LogTypeNginx, parser.Type())
	}
}

// TestNginxErrorParser_Options tests that parser options are applied.
func TestNginxErrorParser_Options(t *testing.T) {
	opts := &Options{
		IncludeRaw: true,
		Source:     "nginx-error-log",
		Labels: map[string]string{
			"env": "staging",
		},
	}

	parser := NewNginxErrorParser(opts)
	line := `2024/10/10 13:55:36 [error] 12345#67890: test error`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if entry.Raw != line {
		t.Errorf("Expected raw line to be included")
	}

	if entry.Source != "nginx-error-log" {
		t.Errorf("Expected source 'nginx-error-log', got '%s'", entry.Source)
	}

	if entry.GetLabel("env") != "staging" {
		t.Errorf("Expected label env='staging', got '%s'", entry.GetLabel("env"))
	}
}

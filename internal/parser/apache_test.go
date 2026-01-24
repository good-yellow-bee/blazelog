package parser

import (
	"testing"
	"time"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

// TestApacheAccessParser_Parse tests the Apache access log parser.
func TestApacheAccessParser_Parse(t *testing.T) {
	parser := NewApacheAccessParser(nil)

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
			name:           "combined format - with ident",
			line:           `192.168.1.1 identd admin [10/Oct/2024:13:55:36 -0700] "GET /page HTTP/1.1" 200 512 "-" "Mozilla/5.0"`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 200,
			expectedMethod: "GET",
			expectedURI:    "/page",
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
			name:           "combined format - no bytes sent (dash)",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "HEAD /status HTTP/1.1" 204 - "-" "curl/7.64.1"`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 204,
			expectedMethod: "HEAD",
			expectedURI:    "/status",
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
			name:           "common format - no bytes (dash)",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 304 -`,
			expectError:    false,
			expectedLevel:  models.LevelInfo,
			expectedStatus: 304,
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
			name:           "403 forbidden",
			line:           `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /secret HTTP/1.1" 403 0 "-" "Mozilla/5.0"`,
			expectError:    false,
			expectedLevel:  models.LevelWarning,
			expectedStatus: 403,
			expectedMethod: "GET",
			expectedURI:    "/secret",
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

			if entry.Type != models.LogTypeApache {
				t.Errorf("Parse(%q): expected type %v, got %v", tt.line, models.LogTypeApache, entry.Type)
			}
		})
	}
}

// TestApacheAccessParser_CombinedFields tests that combined format extracts all fields.
func TestApacheAccessParser_CombinedFields(t *testing.T) {
	parser := NewApacheAccessParser(nil)
	line := `192.168.1.1 identd john [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326 "http://example.com/" "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if entry.GetFieldString("remote_host") != "192.168.1.1" {
		t.Errorf("Expected remote_host '192.168.1.1', got '%s'", entry.GetFieldString("remote_host"))
	}

	if entry.GetFieldString("ident") != "identd" {
		t.Errorf("Expected ident 'identd', got '%s'", entry.GetFieldString("ident"))
	}

	if entry.GetFieldString("remote_user") != "john" {
		t.Errorf("Expected remote_user 'john', got '%s'", entry.GetFieldString("remote_user"))
	}

	if entry.GetFieldString("protocol") != "HTTP/1.1" {
		t.Errorf("Expected protocol 'HTTP/1.1', got '%s'", entry.GetFieldString("protocol"))
	}

	if entry.GetFieldInt("bytes_sent") != 2326 {
		t.Errorf("Expected bytes_sent 2326, got %d", entry.GetFieldInt("bytes_sent"))
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

// TestApacheAccessParser_CanParse tests auto-detection.
func TestApacheAccessParser_CanParse(t *testing.T) {
	parser := NewApacheAccessParser(nil)

	tests := []struct {
		line     string
		expected bool
	}{
		{`192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326 "-" "Mozilla/5.0"`, true},
		{`192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326`, true},
		{`[Sat Oct 10 14:32:52.123456 2020] [core:error] [pid 12345:tid 123456789] AH00124: test`, false},
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

// TestApacheAccessParser_Name tests the parser name.
func TestApacheAccessParser_Name(t *testing.T) {
	parser := NewApacheAccessParser(nil)
	if parser.Name() != "apache-access" {
		t.Errorf("Expected name 'apache-access', got '%s'", parser.Name())
	}
}

// TestApacheAccessParser_Type tests the parser type.
func TestApacheAccessParser_Type(t *testing.T) {
	parser := NewApacheAccessParser(nil)
	if parser.Type() != models.LogTypeApache {
		t.Errorf("Expected type %v, got %v", models.LogTypeApache, parser.Type())
	}
}

// TestApacheAccessParser_Options tests that parser options are applied.
func TestApacheAccessParser_Options(t *testing.T) {
	opts := &Options{
		IncludeRaw: true,
		Source:     "apache-server-1",
		Labels: map[string]string{
			"env": "production",
		},
	}

	parser := NewApacheAccessParser(opts)
	line := `192.168.1.1 - - [10/Oct/2024:13:55:36 -0700] "GET /index.html HTTP/1.1" 200 2326 "-" "Mozilla/5.0"`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if entry.Raw != line {
		t.Errorf("Expected raw line to be included")
	}

	if entry.Source != "apache-server-1" {
		t.Errorf("Expected source 'apache-server-1', got '%s'", entry.Source)
	}

	if entry.GetLabel("env") != "production" {
		t.Errorf("Expected label env='production', got '%s'", entry.GetLabel("env"))
	}
}

// TestApacheErrorParser_Parse tests the Apache error log parser.
func TestApacheErrorParser_Parse(t *testing.T) {
	parser := NewApacheErrorParser(nil)

	tests := []struct {
		name          string
		line          string
		expectError   bool
		expectedLevel models.LogLevel
		expectedMsg   string
	}{
		{
			name:          "Apache 2.4 - error with client",
			line:          `[Sat Oct 10 14:32:52.123456 2020] [core:error] [pid 12345:tid 123456789] [client 192.168.1.1:56789] AH00124: Request exceeded the limit`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "AH00124: Request exceeded the limit",
		},
		{
			name:          "Apache 2.4 - notice without client",
			line:          `[Sat Oct 10 14:32:52.123456 2020] [mpm_prefork:notice] [pid 12345:tid 123456789] AH00163: Apache/2.4.41 configured`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "AH00163: Apache/2.4.41 configured",
		},
		{
			name:          "Apache 2.4 - warn level",
			line:          `[Sat Oct 10 14:32:52.123456 2020] [ssl:warn] [pid 12345:tid 123456789] AH01909: warning message`,
			expectError:   false,
			expectedLevel: models.LevelWarning,
			expectedMsg:   "AH01909: warning message",
		},
		{
			name:          "Apache 2.4 - debug level",
			line:          `[Sat Oct 10 14:32:52.123456 2020] [core:debug] [pid 12345:tid 123456789] debug message here`,
			expectError:   false,
			expectedLevel: models.LevelDebug,
			expectedMsg:   "debug message here",
		},
		{
			name:          "Apache 2.4 - info level",
			line:          `[Sat Oct 10 14:32:52.123456 2020] [core:info] [pid 12345:tid 123456789] informational message`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "informational message",
		},
		{
			name:          "Apache 2.4 - crit level",
			line:          `[Sat Oct 10 14:32:52.123456 2020] [core:crit] [pid 12345:tid 123456789] critical error occurred`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "critical error occurred",
		},
		{
			name:          "Apache 2.4 - alert level",
			line:          `[Sat Oct 10 14:32:52.123456 2020] [core:alert] [pid 12345:tid 123456789] alert message`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "alert message",
		},
		{
			name:          "Apache 2.4 - emerg level",
			line:          `[Sat Oct 10 14:32:52.123456 2020] [core:emerg] [pid 12345:tid 123456789] emergency situation`,
			expectError:   false,
			expectedLevel: models.LevelFatal,
			expectedMsg:   "emergency situation",
		},
		{
			name:          "Apache 2.2 - error with client",
			line:          `[Sat Oct 10 14:32:52 2020] [error] [client 192.168.1.1] File does not exist: /var/www/html/missing.html`,
			expectError:   false,
			expectedLevel: models.LevelError,
			expectedMsg:   "File does not exist: /var/www/html/missing.html",
		},
		{
			name:          "Apache 2.2 - notice without client",
			line:          `[Sat Oct 10 14:32:52 2020] [notice] Apache/2.2.22 configured -- resuming normal operations`,
			expectError:   false,
			expectedLevel: models.LevelInfo,
			expectedMsg:   "Apache/2.2.22 configured -- resuming normal operations",
		},
		{
			name:          "Apache 2.2 - warn level",
			line:          `[Sat Oct 10 14:32:52 2020] [warn] warning message here`,
			expectError:   false,
			expectedLevel: models.LevelWarning,
			expectedMsg:   "warning message here",
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

			if entry.Type != models.LogTypeApache {
				t.Errorf("Parse(%q): expected type %v, got %v", tt.line, models.LogTypeApache, entry.Type)
			}
		})
	}
}

// TestApacheErrorParser_Apache24Fields tests that all fields are extracted correctly for Apache 2.4+.
func TestApacheErrorParser_Apache24Fields(t *testing.T) {
	parser := NewApacheErrorParser(nil)
	line := `[Sat Oct 10 14:32:52.123456 2020] [core:error] [pid 12345:tid 67890] [client 192.168.1.100:54321] AH00124: test error`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if entry.GetFieldString("module") != "core" {
		t.Errorf("Expected module 'core', got '%s'", entry.GetFieldString("module"))
	}

	if entry.GetFieldInt("pid") != 12345 {
		t.Errorf("Expected pid 12345, got %d", entry.GetFieldInt("pid"))
	}

	if entry.GetFieldInt("tid") != 67890 {
		t.Errorf("Expected tid 67890, got %d", entry.GetFieldInt("tid"))
	}

	if entry.GetFieldString("client") != "192.168.1.100:54321" {
		t.Errorf("Expected client '192.168.1.100:54321', got '%s'", entry.GetFieldString("client"))
	}

	if entry.GetFieldString("client_ip") != "192.168.1.100" {
		t.Errorf("Expected client_ip '192.168.1.100', got '%s'", entry.GetFieldString("client_ip"))
	}

	if entry.GetFieldInt("client_port") != 54321 {
		t.Errorf("Expected client_port 54321, got %d", entry.GetFieldInt("client_port"))
	}

	if entry.GetFieldString("apache_level") != "error" {
		t.Errorf("Expected apache_level 'error', got '%s'", entry.GetFieldString("apache_level"))
	}

	// Verify timestamp parsing
	expectedTime := time.Date(2020, 10, 10, 14, 32, 52, 123456000, time.UTC)
	if !entry.Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, entry.Timestamp)
	}
}

// TestApacheErrorParser_Apache22Fields tests that all fields are extracted correctly for Apache 2.2.
func TestApacheErrorParser_Apache22Fields(t *testing.T) {
	parser := NewApacheErrorParser(nil)
	line := `[Sat Oct 10 14:32:52 2020] [error] [client 192.168.1.100] File does not exist: /var/www/test`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if entry.GetFieldString("client") != "192.168.1.100" {
		t.Errorf("Expected client '192.168.1.100', got '%s'", entry.GetFieldString("client"))
	}

	if entry.GetFieldString("client_ip") != "192.168.1.100" {
		t.Errorf("Expected client_ip '192.168.1.100', got '%s'", entry.GetFieldString("client_ip"))
	}

	if entry.GetFieldString("apache_level") != "error" {
		t.Errorf("Expected apache_level 'error', got '%s'", entry.GetFieldString("apache_level"))
	}

	// Verify timestamp parsing
	expectedTime := time.Date(2020, 10, 10, 14, 32, 52, 0, time.UTC)
	if !entry.Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, entry.Timestamp)
	}
}

// TestApacheErrorParser_NoClient tests parsing Apache 2.4 logs without client info.
func TestApacheErrorParser_NoClient(t *testing.T) {
	parser := NewApacheErrorParser(nil)
	line := `[Sat Oct 10 14:32:52.123456 2020] [mpm_prefork:notice] [pid 12345:tid 67890] AH00163: Apache configured`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Client should not be set
	if _, ok := entry.GetField("client"); ok {
		t.Error("Expected client to not be set")
	}

	if entry.GetFieldString("module") != "mpm_prefork" {
		t.Errorf("Expected module 'mpm_prefork', got '%s'", entry.GetFieldString("module"))
	}
}

// TestApacheErrorParser_CanParse tests auto-detection.
func TestApacheErrorParser_CanParse(t *testing.T) {
	parser := NewApacheErrorParser(nil)

	tests := []struct {
		line     string
		expected bool
	}{
		{`[Sat Oct 10 14:32:52.123456 2020] [core:error] [pid 12345:tid 67890] test message`, true},
		{`[Sat Oct 10 14:32:52.123456 2020] [mpm_prefork:notice] [pid 12345:tid 67890] [client 1.2.3.4:5678] test`, true},
		{`[Sat Oct 10 14:32:52 2020] [error] [client 192.168.1.1] test message`, true},
		{`[Sat Oct 10 14:32:52 2020] [notice] test message`, true},
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

// TestApacheErrorParser_Name tests the parser name.
func TestApacheErrorParser_Name(t *testing.T) {
	parser := NewApacheErrorParser(nil)
	if parser.Name() != "apache-error" {
		t.Errorf("Expected name 'apache-error', got '%s'", parser.Name())
	}
}

// TestApacheErrorParser_Type tests the parser type.
func TestApacheErrorParser_Type(t *testing.T) {
	parser := NewApacheErrorParser(nil)
	if parser.Type() != models.LogTypeApache {
		t.Errorf("Expected type %v, got %v", models.LogTypeApache, parser.Type())
	}
}

// TestApacheErrorParser_Options tests that parser options are applied.
func TestApacheErrorParser_Options(t *testing.T) {
	opts := &Options{
		IncludeRaw: true,
		Source:     "apache-error-log",
		Labels: map[string]string{
			"env": "staging",
		},
	}

	parser := NewApacheErrorParser(opts)
	line := `[Sat Oct 10 14:32:52.123456 2020] [core:error] [pid 12345:tid 67890] test error`

	entry, err := parser.Parse(line)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if entry.Raw != line {
		t.Errorf("Expected raw line to be included")
	}

	if entry.Source != "apache-error-log" {
		t.Errorf("Expected source 'apache-error-log', got '%s'", entry.Source)
	}

	if entry.GetLabel("env") != "staging" {
		t.Errorf("Expected label env='staging', got '%s'", entry.GetLabel("env"))
	}
}

// TestApacheErrorParser_TraceLevels tests trace levels are recognized.
func TestApacheErrorParser_TraceLevels(t *testing.T) {
	parser := NewApacheErrorParser(nil)

	for i := 1; i <= 8; i++ {
		line := `[Sat Oct 10 14:32:52.123456 2020] [core:trace` + string(rune('0'+i)) + `] [pid 12345:tid 67890] trace message`
		entry, err := parser.Parse(line)
		if err != nil {
			t.Fatalf("Parse trace%d error: %v", i, err)
		}
		if entry.Level != models.LevelDebug {
			t.Errorf("trace%d: expected level debug, got %v", i, entry.Level)
		}
	}
}

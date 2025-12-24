package alerting

import (
	"fmt"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/good-yellow-bee/blazelog/internal/models"
)

// ExprMatcher compiles and evaluates expr-lang expressions against log entries.
type ExprMatcher struct {
	expression string
	program    *vm.Program
}

// NewExprMatcher creates a new ExprMatcher for the given expression.
func NewExprMatcher(expression string) (*ExprMatcher, error) {
	m := &ExprMatcher{expression: expression}
	if err := m.compile(); err != nil {
		return nil, err
	}
	return m, nil
}

// compile compiles the expression with the expected environment.
func (m *ExprMatcher) compile() error {
	// Create sample environment for type checking
	sampleEnv := buildSampleEnv()

	// Compile with type checking
	// Note: expr-lang has built-in operators: contains, startsWith, endsWith
	// Syntax: message contains "timeout" (not contains(message, "timeout"))
	program, err := expr.Compile(m.expression,
		expr.Env(sampleEnv),
		expr.AsBool(),
	)
	if err != nil {
		return fmt.Errorf("compile expression: %w", err)
	}

	m.program = program
	return nil
}

// Match evaluates the expression against a log entry.
func (m *ExprMatcher) Match(entry *models.LogEntry) (bool, error) {
	env := buildEnvFromEntry(entry)

	result, err := expr.Run(m.program, env)
	if err != nil {
		return false, fmt.Errorf("evaluate expression: %w", err)
	}

	matched, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("expression did not return bool: got %T", result)
	}

	return matched, nil
}

// Expression returns the original expression string.
func (m *ExprMatcher) Expression() string {
	return m.expression
}

// buildSampleEnv creates a sample environment for expression compilation.
func buildSampleEnv() map[string]any {
	return map[string]any{
		"level":       "",
		"message":     "",
		"source":      "",
		"type":        "",
		"file_path":   "",
		"http_status": 0,
		"http_method": "",
		"uri":         "",
		"fields":      map[string]any{},
		"labels":      map[string]string{},
	}
}

// buildEnvFromEntry creates an evaluation environment from a log entry.
func buildEnvFromEntry(entry *models.LogEntry) map[string]any {
	env := map[string]any{
		"level":       strings.ToLower(string(entry.Level)),
		"message":     entry.Message,
		"source":      entry.Source,
		"type":        strings.ToLower(string(entry.Type)),
		"file_path":   entry.FilePath,
		"http_status": entry.GetFieldInt("status"),
		"http_method": entry.GetFieldString("method"),
		"uri":         entry.GetFieldString("uri"),
		"fields":      entry.Fields,
		"labels":      entry.Labels,
	}

	// Ensure fields and labels are never nil
	if env["fields"] == nil {
		env["fields"] = map[string]any{}
	}
	if env["labels"] == nil {
		env["labels"] = map[string]string{}
	}

	return env
}


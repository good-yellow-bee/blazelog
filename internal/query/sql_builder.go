package query

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/expr-lang/expr/ast"
)

// reDoSPattern detects potentially dangerous regex patterns that can cause catastrophic backtracking.
// Matches nested quantifiers like (a+)+, (a*)+, (a|a)+, etc.
var reDoSPattern = regexp.MustCompile(`\([^)]*[+*][^)]*\)[+*]|\([^)]*\|[^)]*\)[+*]`)

// SQLBuilder converts parsed expressions to ClickHouse SQL.
type SQLBuilder struct {
	fields map[string]FieldDef
}

// NewSQLBuilder creates a new SQL builder.
func NewSQLBuilder(fields map[string]FieldDef) *SQLBuilder {
	return &SQLBuilder{fields: fields}
}

// BuildResult contains the generated SQL and parameters.
type BuildResult struct {
	SQL  string
	Args []any
}

// Build generates SQL WHERE clause from a parsed query.
func (b *SQLBuilder) Build(pq *ParsedQuery) (*BuildResult, error) {
	v := &sqlVisitor{
		fields: b.fields,
		args:   make([]any, 0),
	}

	node := pq.Node()
	sql, err := v.visit(&node)
	if err != nil {
		return nil, err
	}

	return &BuildResult{
		SQL:  sql,
		Args: v.args,
	}, nil
}

// sqlVisitor traverses AST and generates SQL.
type sqlVisitor struct {
	fields map[string]FieldDef
	args   []any
}

func (v *sqlVisitor) visit(node *ast.Node) (string, error) {
	switch n := (*node).(type) {
	case *ast.BinaryNode:
		return v.visitBinary(n)
	case *ast.UnaryNode:
		return v.visitUnary(n)
	case *ast.IdentifierNode:
		return v.visitIdentifier(n)
	case *ast.StringNode:
		return v.visitString(n)
	case *ast.IntegerNode:
		return v.visitInteger(n)
	case *ast.FloatNode:
		return v.visitFloat(n)
	case *ast.BoolNode:
		return v.visitBool(n)
	case *ast.ArrayNode:
		return v.visitArray(n)
	case *ast.ConstantNode:
		return v.visitConstant(n)
	case *ast.CallNode:
		return v.visitCall(n)
	case *ast.MemberNode:
		return v.visitMember(n)
	case *ast.NilNode:
		return "NULL", nil
	default:
		return "", fmt.Errorf("unsupported node type: %T", n)
	}
}

func (v *sqlVisitor) visitBinary(n *ast.BinaryNode) (string, error) {
	// Handle string function calls like message contains "error"
	if v.isStringMethodCall(n) {
		return v.handleStringMethod(n)
	}

	left, err := v.visit(&n.Left)
	if err != nil {
		return "", err
	}

	right, err := v.visit(&n.Right)
	if err != nil {
		return "", err
	}

	// Handle 'in' operator specially
	if n.Operator == "in" {
		return fmt.Sprintf("%s IN %s", left, right), nil
	}

	op, err := v.mapOperator(n.Operator)
	if err != nil {
		return "", err
	}

	// Handle case-insensitive string comparison
	if v.isStringField(n.Left) && (n.Operator == "==" || n.Operator == "!=") {
		left = fmt.Sprintf("lower(%s)", left)
		// The right side value will be lowercased when added as arg
	}

	return fmt.Sprintf("(%s %s %s)", left, op, right), nil
}

func (v *sqlVisitor) visitUnary(n *ast.UnaryNode) (string, error) {
	operand, err := v.visit(&n.Node)
	if err != nil {
		return "", err
	}

	switch n.Operator {
	case "not", "!":
		return fmt.Sprintf("NOT (%s)", operand), nil
	case "-":
		return fmt.Sprintf("-%s", operand), nil
	default:
		return "", fmt.Errorf("unsupported unary operator: %s", n.Operator)
	}
}

func (v *sqlVisitor) visitIdentifier(n *ast.IdentifierNode) (string, error) {
	if field, ok := v.fields[n.Value]; ok {
		return field.Column, nil
	}
	return "", fmt.Errorf("unknown field: %s", n.Value)
}

func (v *sqlVisitor) visitString(n *ast.StringNode) (string, error) {
	v.args = append(v.args, strings.ToLower(n.Value))
	return "?", nil
}

func (v *sqlVisitor) visitInteger(n *ast.IntegerNode) (string, error) {
	v.args = append(v.args, n.Value)
	return "?", nil
}

func (v *sqlVisitor) visitFloat(n *ast.FloatNode) (string, error) {
	v.args = append(v.args, n.Value)
	return "?", nil
}

func (v *sqlVisitor) visitBool(n *ast.BoolNode) (string, error) {
	if n.Value {
		return "1", nil
	}
	return "0", nil
}

func (v *sqlVisitor) visitArray(n *ast.ArrayNode) (string, error) {
	parts := make([]string, len(n.Nodes))
	for i, node := range n.Nodes {
		sql, err := v.visit(&node)
		if err != nil {
			return "", err
		}
		parts[i] = sql
	}
	return fmt.Sprintf("(%s)", strings.Join(parts, ", ")), nil
}

func (v *sqlVisitor) visitConstant(n *ast.ConstantNode) (string, error) {
	// ConstantNode is used for optimized constant arrays like ["a", "b"]
	switch val := n.Value.(type) {
	case []any:
		parts := make([]string, len(val))
		for i, item := range val {
			switch item := item.(type) {
			case string:
				v.args = append(v.args, strings.ToLower(item))
			default:
				v.args = append(v.args, item)
			}
			parts[i] = "?"
		}
		return fmt.Sprintf("(%s)", strings.Join(parts, ", ")), nil
	case map[string]struct{}:
		// expr optimizes "in" arrays to maps for fast lookup
		parts := make([]string, 0, len(val))
		for key := range val {
			v.args = append(v.args, strings.ToLower(key))
			parts = append(parts, "?")
		}
		return fmt.Sprintf("(%s)", strings.Join(parts, ", ")), nil
	case string:
		v.args = append(v.args, strings.ToLower(val))
		return "?", nil
	case int, int64, float64:
		v.args = append(v.args, val)
		return "?", nil
	case bool:
		if val {
			return "1", nil
		}
		return "0", nil
	default:
		return "", fmt.Errorf("unsupported constant type: %T", val)
	}
}

func (v *sqlVisitor) visitCall(n *ast.CallNode) (string, error) {
	callee, ok := n.Callee.(*ast.IdentifierNode)
	if !ok {
		return "", fmt.Errorf("unsupported callee type")
	}

	switch callee.Value {
	case "now":
		return "now()", nil

	case "duration":
		if len(n.Arguments) != 1 {
			return "", fmt.Errorf("duration() requires exactly 1 argument")
		}
		strNode, ok := n.Arguments[0].(*ast.StringNode)
		if !ok {
			return "", fmt.Errorf("duration() argument must be a string")
		}
		dur, err := time.ParseDuration(strNode.Value)
		if err != nil {
			return "", fmt.Errorf("invalid duration: %w", err)
		}
		return v.durationToInterval(dur), nil

	case "lower":
		if len(n.Arguments) != 1 {
			return "", fmt.Errorf("lower() requires exactly 1 argument")
		}
		arg, err := v.visit(&n.Arguments[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("lower(%s)", arg), nil

	case "upper":
		if len(n.Arguments) != 1 {
			return "", fmt.Errorf("upper() requires exactly 1 argument")
		}
		arg, err := v.visit(&n.Arguments[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("upper(%s)", arg), nil

	case "len":
		if len(n.Arguments) != 1 {
			return "", fmt.Errorf("len() requires exactly 1 argument")
		}
		arg, err := v.visit(&n.Arguments[0])
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("length(%s)", arg), nil

	default:
		return "", fmt.Errorf("unsupported function: %s", callee.Value)
	}
}

func (v *sqlVisitor) visitMember(n *ast.MemberNode) (string, error) {
	// Handle JSON field access: fields.status or labels.key
	ident, ok := n.Node.(*ast.IdentifierNode)
	if !ok {
		return "", fmt.Errorf("unsupported member access")
	}

	field, ok := v.fields[ident.Value]
	if !ok {
		return "", fmt.Errorf("unknown field: %s", ident.Value)
	}

	if field.Type != FieldTypeJSON {
		return "", fmt.Errorf("field %q does not support member access", ident.Value)
	}

	var propName string
	switch prop := n.Property.(type) {
	case *ast.StringNode:
		propName = prop.Value
	case *ast.IdentifierNode:
		propName = prop.Value
	default:
		return "", fmt.Errorf("unsupported property type")
	}

	// Validate property name to prevent SQL injection
	// Only allow alphanumeric characters, underscores, and hyphens
	if !isValidJSONPropertyName(propName) {
		return "", fmt.Errorf("invalid JSON property name: %q", propName)
	}

	// Use JSONExtractString for JSON field access
	// Property name is validated above to be safe
	return fmt.Sprintf("JSONExtractString(%s, '%s')", field.Column, propName), nil
}

// isStringMethodCall checks if this is a string method call like "message contains x".
func (v *sqlVisitor) isStringMethodCall(n *ast.BinaryNode) bool {
	switch n.Operator {
	case "contains", "startsWith", "endsWith", "matches":
		return true
	}
	return false
}

// handleStringMethod handles string method operations.
func (v *sqlVisitor) handleStringMethod(n *ast.BinaryNode) (string, error) {
	left, err := v.visit(&n.Left)
	if err != nil {
		return "", err
	}

	right, err := v.visit(&n.Right)
	if err != nil {
		return "", err
	}

	switch n.Operator {
	case "contains":
		return fmt.Sprintf("position(lower(%s), %s) > 0", left, right), nil
	case "startsWith":
		return fmt.Sprintf("startsWith(lower(%s), %s)", left, right), nil
	case "endsWith":
		return fmt.Sprintf("endsWith(lower(%s), %s)", left, right), nil
	case "matches":
		// Validate regex for ReDoS before using
		if strNode, ok := n.Right.(*ast.StringNode); ok {
			if reDoSPattern.MatchString(strNode.Value) {
				return "", fmt.Errorf("potentially dangerous regex pattern: nested quantifiers detected")
			}
		}
		return fmt.Sprintf("match(lower(%s), %s)", left, right), nil
	default:
		return "", fmt.Errorf("unknown string method: %s", n.Operator)
	}
}

// isStringField checks if a node is a string type field.
func (v *sqlVisitor) isStringField(node ast.Node) bool {
	if ident, ok := node.(*ast.IdentifierNode); ok {
		if field, ok := v.fields[ident.Value]; ok {
			return field.Type == FieldTypeString
		}
	}
	return false
}

// mapOperator converts expr operators to SQL operators.
func (v *sqlVisitor) mapOperator(op string) (string, error) {
	switch op {
	case "==":
		return "=", nil
	case "!=":
		return "!=", nil
	case "and", "&&":
		return "AND", nil
	case "or", "||":
		return "OR", nil
	case ">=", "<=", ">", "<":
		return op, nil
	case "-", "+", "*", "/":
		return op, nil
	default:
		return "", fmt.Errorf("unknown operator: %s", op)
	}
}

// durationToInterval converts Go duration to ClickHouse interval.
func (v *sqlVisitor) durationToInterval(d time.Duration) string {
	hours := d.Hours()
	if hours >= 24 {
		days := int(hours / 24)
		return fmt.Sprintf("INTERVAL %d DAY", days)
	}
	if hours >= 1 {
		return fmt.Sprintf("INTERVAL %d HOUR", int(hours))
	}
	minutes := d.Minutes()
	if minutes >= 1 {
		return fmt.Sprintf("INTERVAL %d MINUTE", int(minutes))
	}
	return fmt.Sprintf("INTERVAL %d SECOND", int(d.Seconds()))
}

// isValidJSONPropertyName checks if a property name is safe for use in SQL.
// Only allows alphanumeric characters, underscores, and hyphens.
func isValidJSONPropertyName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

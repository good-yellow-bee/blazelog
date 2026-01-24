package query

import (
	"fmt"
	"reflect"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"
	"github.com/expr-lang/expr/vm"
)

const (
	maxASTDepth = 20
	maxASTNodes = 100
)

// ParsedQuery holds a validated and parsed expression.
type ParsedQuery struct {
	program *vm.Program
	node    ast.Node
	raw     string
}

// Node returns the AST root node.
func (pq *ParsedQuery) Node() ast.Node {
	return pq.node
}

// Raw returns the original expression string.
func (pq *ParsedQuery) Raw() string {
	return pq.raw
}

// DSL handles expression parsing and validation.
type DSL struct {
	fields map[string]FieldDef
}

// NewQueryDSL creates a new DSL parser with the given field definitions.
func NewQueryDSL(fields map[string]FieldDef) *DSL {
	return &DSL{fields: fields}
}

// Parse compiles and validates an expression string.
func (d *DSL) Parse(expression string) (*ParsedQuery, error) {
	if expression == "" {
		return nil, fmt.Errorf("empty expression")
	}

	env := d.buildEnv()

	program, err := expr.Compile(
		expression,
		expr.Env(env),
		expr.AsBool(),
	)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	node := program.Node()

	// Validate fields and operators
	if err := d.validateAST(&node); err != nil {
		return nil, err
	}

	return &ParsedQuery{
		program: program,
		node:    node,
		raw:     expression,
	}, nil
}

// buildEnv creates the environment for expr compilation.
func (d *DSL) buildEnv() map[string]any {
	env := make(map[string]any)

	// Add fields as typed placeholders
	for name, field := range d.fields {
		switch field.Type {
		case FieldTypeString:
			env[name] = ""
		case FieldTypeInt:
			env[name] = 0
		case FieldTypeFloat:
			env[name] = 0.0
		case FieldTypeTime:
			env[name] = time.Time{}
		case FieldTypeJSON:
			env[name] = map[string]any{}
		}
	}

	// Add allowed functions
	env["now"] = func() time.Time { return time.Now() }
	env["duration"] = func(s string) time.Duration {
		d, _ := time.ParseDuration(s)
		return d
	}

	// Add string functions for method-style calls
	env["contains"] = func(s, substr string) bool { return true }
	env["startsWith"] = func(s, prefix string) bool { return true }
	env["endsWith"] = func(s, suffix string) bool { return true }
	env["matches"] = func(s, pattern string) bool { return true }

	return env
}

// validateAST walks the AST to validate fields, operators, depth and complexity.
func (d *DSL) validateAST(node *ast.Node) error {
	v := &validationVisitor{fields: d.fields}
	ast.Walk(node, v)
	if v.err != nil {
		return v.err
	}

	// Check AST complexity limits
	depth, nodes := measureASTComplexity(node, 0)
	if depth > maxASTDepth {
		return fmt.Errorf("query too deeply nested: depth %d exceeds limit %d", depth, maxASTDepth)
	}
	if nodes > maxASTNodes {
		return fmt.Errorf("query too complex: %d nodes exceeds limit %d", nodes, maxASTNodes)
	}

	return nil
}

// measureASTComplexity recursively measures AST depth and node count.
func measureASTComplexity(node *ast.Node, currentDepth int) (maxDepth, nodeCount int) {
	if node == nil || *node == nil {
		return currentDepth, 0
	}

	maxDepth = currentDepth + 1
	nodeCount = 1

	switch n := (*node).(type) {
	case *ast.BinaryNode:
		leftDepth, leftCount := measureASTComplexity(&n.Left, currentDepth+1)
		rightDepth, rightCount := measureASTComplexity(&n.Right, currentDepth+1)
		if leftDepth > maxDepth {
			maxDepth = leftDepth
		}
		if rightDepth > maxDepth {
			maxDepth = rightDepth
		}
		nodeCount += leftCount + rightCount
	case *ast.UnaryNode:
		childDepth, childCount := measureASTComplexity(&n.Node, currentDepth+1)
		if childDepth > maxDepth {
			maxDepth = childDepth
		}
		nodeCount += childCount
	case *ast.ArrayNode:
		for _, elem := range n.Nodes {
			elemDepth, elemCount := measureASTComplexity(&elem, currentDepth+1)
			if elemDepth > maxDepth {
				maxDepth = elemDepth
			}
			nodeCount += elemCount
		}
	case *ast.CallNode:
		for _, arg := range n.Arguments {
			argDepth, argCount := measureASTComplexity(&arg, currentDepth+1)
			if argDepth > maxDepth {
				maxDepth = argDepth
			}
			nodeCount += argCount
		}
	case *ast.MemberNode:
		baseDepth, baseCount := measureASTComplexity(&n.Node, currentDepth+1)
		if baseDepth > maxDepth {
			maxDepth = baseDepth
		}
		nodeCount += baseCount
	}

	return maxDepth, nodeCount
}

// validationVisitor checks fields and operators in the AST.
type validationVisitor struct {
	fields map[string]FieldDef
	err    error
}

func (v *validationVisitor) Visit(node *ast.Node) {
	if v.err != nil {
		return
	}

	switch n := (*node).(type) {
	case *ast.IdentifierNode:
		// Check if identifier is a known field or function
		if _, ok := v.fields[n.Value]; !ok {
			if !AllowedFunctions[n.Value] && !isBuiltinFunction(n.Value) {
				v.err = fmt.Errorf("unknown field: %s", n.Value)
			}
		}

	case *ast.BinaryNode:
		// Validate operator against field type (check both left and right sides)
		if ident, ok := n.Left.(*ast.IdentifierNode); ok {
			if field, ok := v.fields[ident.Value]; ok {
				if !field.IsOperatorAllowed(n.Operator) {
					v.err = fmt.Errorf("operator %q not allowed for field %q", n.Operator, ident.Value)
					return
				}
			}
		}
		if ident, ok := n.Right.(*ast.IdentifierNode); ok {
			if field, ok := v.fields[ident.Value]; ok {
				if !field.IsOperatorAllowed(n.Operator) {
					v.err = fmt.Errorf("operator %q not allowed for field %q", n.Operator, ident.Value)
					return
				}
			}
		}

	case *ast.MemberNode:
		// Handle JSON field access like fields.status
		if ident, ok := n.Node.(*ast.IdentifierNode); ok {
			if field, ok := v.fields[ident.Value]; ok {
				if field.Type != FieldTypeJSON {
					v.err = fmt.Errorf("field %q does not support member access", ident.Value)
				}
			}
		}

	case *ast.CallNode:
		// Validate function calls
		if ident, ok := n.Callee.(*ast.IdentifierNode); ok {
			if !AllowedFunctions[ident.Value] && !isBuiltinFunction(ident.Value) {
				v.err = fmt.Errorf("function %q is not allowed", ident.Value)
			}
		}
	}
}

// isBuiltinFunction checks if a function is a built-in expr function.
func isBuiltinFunction(name string) bool {
	builtins := map[string]bool{
		"len": true, "lower": true, "upper": true, "trim": true,
		"int": true, "float": true, "string": true, "bool": true,
		"abs": true, "ceil": true, "floor": true, "round": true,
		"min": true, "max": true,
	}
	return builtins[name]
}

// FieldInfo extracts field information from an AST node.
type FieldInfo struct {
	Name     string
	Column   string
	JSONPath string // For JSON field access like fields.status
	Type     FieldType
}

// ExtractFieldInfo gets field information from an identifier or member node.
func ExtractFieldInfo(node ast.Node, fields map[string]FieldDef) (*FieldInfo, error) {
	switch n := node.(type) {
	case *ast.IdentifierNode:
		if field, ok := fields[n.Value]; ok {
			return &FieldInfo{
				Name:   n.Value,
				Column: field.Column,
				Type:   field.Type,
			}, nil
		}
		return nil, fmt.Errorf("unknown field: %s", n.Value)

	case *ast.MemberNode:
		// Handle fields.status or labels.key
		if ident, ok := n.Node.(*ast.IdentifierNode); ok {
			if field, ok := fields[ident.Value]; ok {
				if field.Type != FieldTypeJSON {
					return nil, fmt.Errorf("field %q does not support member access", ident.Value)
				}
				var propName string
				if prop, ok := n.Property.(*ast.StringNode); ok {
					propName = prop.Value
				} else if prop, ok := n.Property.(*ast.IdentifierNode); ok {
					propName = prop.Value
				}
				return &FieldInfo{
					Name:     ident.Value + "." + propName,
					Column:   field.Column,
					JSONPath: propName,
					Type:     FieldTypeJSON,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("cannot extract field info from node type: %s", reflect.TypeOf(node))
}

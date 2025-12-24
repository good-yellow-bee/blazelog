// Package query provides DSL parsing and SQL generation for log queries.
package query

// FieldType represents the data type of a queryable field.
type FieldType int

const (
	FieldTypeString FieldType = iota
	FieldTypeInt
	FieldTypeFloat
	FieldTypeTime
	FieldTypeJSON
)

// FieldDef defines a queryable field with its allowed operators.
type FieldDef struct {
	Name      string    // expr field name
	Column    string    // ClickHouse column name
	Type      FieldType // data type
	Operators []string  // allowed operators
}

// DefaultFields contains all queryable log fields.
var DefaultFields = map[string]FieldDef{
	// Core fields
	"level": {
		Name:      "level",
		Column:    "level",
		Type:      FieldTypeString,
		Operators: []string{"==", "!=", "in"},
	},
	"message": {
		Name:      "message",
		Column:    "message",
		Type:      FieldTypeString,
		Operators: []string{"==", "!=", "contains", "startsWith", "endsWith", "matches"},
	},
	"source": {
		Name:      "source",
		Column:    "source",
		Type:      FieldTypeString,
		Operators: []string{"==", "!=", "in", "contains"},
	},
	"type": {
		Name:      "type",
		Column:    "type",
		Type:      FieldTypeString,
		Operators: []string{"==", "!=", "in"},
	},
	"agent_id": {
		Name:      "agent_id",
		Column:    "agent_id",
		Type:      FieldTypeString,
		Operators: []string{"==", "!=", "in"},
	},
	"file_path": {
		Name:      "file_path",
		Column:    "file_path",
		Type:      FieldTypeString,
		Operators: []string{"==", "!=", "contains", "startsWith", "endsWith"},
	},

	// Time fields
	"timestamp": {
		Name:      "timestamp",
		Column:    "timestamp",
		Type:      FieldTypeTime,
		Operators: []string{">=", "<=", ">", "<"},
	},

	// HTTP fields
	"http_status": {
		Name:      "http_status",
		Column:    "http_status",
		Type:      FieldTypeInt,
		Operators: []string{"==", "!=", ">=", "<=", ">", "<", "in"},
	},
	"http_method": {
		Name:      "http_method",
		Column:    "http_method",
		Type:      FieldTypeString,
		Operators: []string{"==", "!=", "in"},
	},
	"uri": {
		Name:      "uri",
		Column:    "uri",
		Type:      FieldTypeString,
		Operators: []string{"==", "!=", "contains", "startsWith", "endsWith", "matches"},
	},

	// JSON fields prefix (special handling)
	"fields": {
		Name:      "fields",
		Column:    "fields",
		Type:      FieldTypeJSON,
		Operators: []string{"==", "!=", ">=", "<=", ">", "<", "in", "contains"},
	},
	"labels": {
		Name:      "labels",
		Column:    "labels",
		Type:      FieldTypeJSON,
		Operators: []string{"==", "!=", "in", "contains"},
	},
}

// IsOperatorAllowed checks if an operator is valid for a field.
func (f FieldDef) IsOperatorAllowed(op string) bool {
	for _, allowed := range f.Operators {
		if allowed == op {
			return true
		}
	}
	return false
}

// AllowedFunctions lists functions allowed in expressions.
var AllowedFunctions = map[string]bool{
	"now":      true,
	"duration": true,
}

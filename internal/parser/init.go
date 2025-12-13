// Package parser provides log parsing functionality for various log formats.
package parser

// init registers all built-in parsers with the default registry.
func init() {
	// Register Nginx access parser for auto-detection
	// Note: We only register the access parser by default since both parsers
	// return the same LogType (nginx). The error parser can be explicitly
	// requested via the CLI with "nginx-error".
	Register(NewNginxAccessParser(nil))
}

package parser

import (
	"fmt"
)

// LoadCustomParsers creates custom parsers from configuration.
func LoadCustomParsers(configs []CustomParserConfig) ([]*CustomParser, error) {
	parsers := make([]*CustomParser, 0, len(configs))

	for i := range configs {
		cfg := &configs[i]
		p, err := NewCustomParser(cfg, nil)
		if err != nil {
			return nil, fmt.Errorf("load parser %q: %w", cfg.Name, err)
		}
		parsers = append(parsers, p)
	}

	return parsers, nil
}

// RegisterCustomParsers loads and registers custom parsers in the registry.
func RegisterCustomParsers(registry *Registry, configs []CustomParserConfig) error {
	// Check for duplicate names with built-in parsers
	for i := range configs {
		name := configs[i].Name
		if _, ok := registry.GetByName(name); ok {
			return fmt.Errorf("parser name %q conflicts with existing parser", name)
		}
	}

	// Check for duplicates within custom parsers
	seen := make(map[string]bool)
	for i := range configs {
		name := configs[i].Name
		if seen[name] {
			return fmt.Errorf("duplicate custom parser name: %q", name)
		}
		seen[name] = true
	}

	// Load and register
	parsers, err := LoadCustomParsers(configs)
	if err != nil {
		return err
	}

	for _, p := range parsers {
		registry.Register(p)
	}

	return nil
}

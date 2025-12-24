package parser

import (
	"fmt"
)

// LoadCustomParsers creates custom parsers from configuration.
func LoadCustomParsers(configs []CustomParserConfig) ([]*CustomParser, error) {
	parsers := make([]*CustomParser, 0, len(configs))

	for i, cfg := range configs {
		p, err := NewCustomParser(&configs[i], nil)
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
	for _, cfg := range configs {
		if _, ok := registry.GetByName(cfg.Name); ok {
			return fmt.Errorf("parser name %q conflicts with existing parser", cfg.Name)
		}
	}

	// Check for duplicates within custom parsers
	seen := make(map[string]bool)
	for _, cfg := range configs {
		if seen[cfg.Name] {
			return fmt.Errorf("duplicate custom parser name: %q", cfg.Name)
		}
		seen[cfg.Name] = true
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

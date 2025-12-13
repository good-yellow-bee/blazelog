package alerting

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadRulesFromFile loads alert rules from a YAML file.
func LoadRulesFromFile(path string) ([]*Rule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open rules file: %w", err)
	}
	defer f.Close()

	return LoadRules(f)
}

// LoadRules loads alert rules from a reader.
func LoadRules(r io.Reader) ([]*Rule, error) {
	var config RulesConfig
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse rules YAML: %w", err)
	}

	// Validate all rules
	for i, rule := range config.Rules {
		if err := rule.Validate(); err != nil {
			return nil, fmt.Errorf("invalid rule at index %d: %w", i, err)
		}
	}

	return config.Rules, nil
}

// LoadRulesFromBytes loads alert rules from YAML bytes.
func LoadRulesFromBytes(data []byte) ([]*Rule, error) {
	var config RulesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse rules YAML: %w", err)
	}

	// Validate all rules
	for i, rule := range config.Rules {
		if err := rule.Validate(); err != nil {
			return nil, fmt.Errorf("invalid rule at index %d: %w", i, err)
		}
	}

	return config.Rules, nil
}

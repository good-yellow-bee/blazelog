package projects

import (
	"errors"
	"strings"
)

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 100 {
		return errors.New("name must be 100 characters or less")
	}
	return nil
}

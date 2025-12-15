package alerts

import (
	"errors"
	"strings"

	"github.com/good-yellow-bee/blazelog/internal/models"
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

func ValidateType(t string) (models.AlertType, error) {
	switch t {
	case "pattern", "threshold":
		return models.AlertType(t), nil
	default:
		return "", errors.New("type must be 'pattern' or 'threshold'")
	}
}

func ValidateSeverity(s string) (models.Severity, error) {
	switch s {
	case "low", "medium", "high", "critical":
		return models.Severity(s), nil
	default:
		return "", errors.New("severity must be 'low', 'medium', 'high', or 'critical'")
	}
}

func ValidateCondition(condition string) error {
	if strings.TrimSpace(condition) == "" {
		return errors.New("condition is required")
	}
	return nil
}

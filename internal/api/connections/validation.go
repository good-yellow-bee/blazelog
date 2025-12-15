package connections

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

func ValidateType(t string) (models.ConnectionType, error) {
	switch t {
	case "ssh", "local":
		return models.ConnectionType(t), nil
	default:
		return "", errors.New("type must be 'ssh' or 'local'")
	}
}

func ValidateHost(host string) error {
	if strings.TrimSpace(host) == "" {
		return errors.New("host is required for SSH connections")
	}
	return nil
}

func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	return nil
}

func ValidateUser(user string) error {
	if strings.TrimSpace(user) == "" {
		return errors.New("user is required for SSH connections")
	}
	return nil
}

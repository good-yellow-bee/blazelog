package models

import (
	"time"
)

// ConnectionType represents the type of connection.
type ConnectionType string

const (
	ConnectionTypeSSH   ConnectionType = "ssh"
	ConnectionTypeLocal ConnectionType = "local"
)

// ConnectionStatus represents the connection status.
type ConnectionStatus string

const (
	ConnectionStatusUnknown     ConnectionStatus = "unknown"
	ConnectionStatusConnected   ConnectionStatus = "connected"
	ConnectionStatusFailed      ConnectionStatus = "failed"
	ConnectionStatusDisconnected ConnectionStatus = "disconnected"
)

// Connection represents an SSH or system connection configuration.
type Connection struct {
	ID                   string           `json:"id"`
	Name                 string           `json:"name"`
	Type                 ConnectionType   `json:"type"`
	Host                 string           `json:"host,omitempty"`
	Port                 int              `json:"port,omitempty"`
	User                 string           `json:"user,omitempty"`
	CredentialsEncrypted []byte           `json:"-"` // Never expose in JSON
	Status               ConnectionStatus `json:"status"`
	LastTestedAt         *time.Time       `json:"last_tested_at,omitempty"`
	ProjectID            string           `json:"project_id,omitempty"`
	CreatedAt            time.Time        `json:"created_at"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

// NewConnection creates a new Connection with initialized timestamps.
func NewConnection(name string, connType ConnectionType) *Connection {
	now := time.Now()
	return &Connection{
		Name:      name,
		Type:      connType,
		Status:    ConnectionStatusUnknown,
		Port:      22, // Default SSH port
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsSSH returns true if this is an SSH connection.
func (c *Connection) IsSSH() bool {
	return c.Type == ConnectionTypeSSH
}

// ParseConnectionType converts a string to ConnectionType.
func ParseConnectionType(s string) ConnectionType {
	switch s {
	case "ssh":
		return ConnectionTypeSSH
	case "local":
		return ConnectionTypeLocal
	default:
		return ConnectionTypeSSH
	}
}

// ParseConnectionStatus converts a string to ConnectionStatus.
func ParseConnectionStatus(s string) ConnectionStatus {
	switch s {
	case "connected":
		return ConnectionStatusConnected
	case "failed":
		return ConnectionStatusFailed
	case "disconnected":
		return ConnectionStatusDisconnected
	default:
		return ConnectionStatusUnknown
	}
}

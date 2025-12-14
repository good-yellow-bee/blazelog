package models

import (
	"time"
)

// Project represents a logical grouping of connections and alerts.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewProject creates a new Project with initialized timestamps.
func NewProject(name, description string) *Project {
	now := time.Now()
	return &Project{
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// ProjectUser represents a user's membership in a project.
type ProjectUser struct {
	ProjectID string `json:"project_id"`
	UserID    string `json:"user_id"`
	Role      Role   `json:"role"`
}

package middleware

import (
	"context"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// ProjectAccess defines what projects a user can access.
type ProjectAccess struct {
	AllProjects       bool     // Admin override - can see everything
	ProjectIDs        []string // Specific project IDs user can access
	IncludeUnassigned bool     // Can see logs with empty project_id
	LegacyMode        bool     // Operator with no assignments (show warning)
}

// GetProjectAccess returns the project access rules for a user.
func GetProjectAccess(ctx context.Context, userID string, role models.Role, store storage.Storage) (*ProjectAccess, error) {
	// Admins can access everything
	if role == models.RoleAdmin {
		return &ProjectAccess{
			AllProjects:       true,
			IncludeUnassigned: true,
		}, nil
	}

	// Get user's project assignments
	projects, err := store.Projects().GetProjectsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	projectIDs := make([]string, len(projects))
	for i, p := range projects {
		projectIDs[i] = p.ID
	}

	// Operator with no assignments = legacy mode (full access + warning)
	if role == models.RoleOperator && len(projectIDs) == 0 {
		return &ProjectAccess{
			AllProjects:       true,
			IncludeUnassigned: true,
			LegacyMode:        true, // Triggers UI warning
		}, nil
	}

	// Operator with assignments = assigned projects + unassigned
	if role == models.RoleOperator {
		return &ProjectAccess{
			ProjectIDs:        projectIDs,
			IncludeUnassigned: true,
		}, nil
	}

	// Viewer with assignments = assigned projects only (no unassigned)
	if role == models.RoleViewer && len(projectIDs) > 0 {
		return &ProjectAccess{
			ProjectIDs:        projectIDs,
			IncludeUnassigned: false,
		}, nil
	}

	// Viewer with no assignments = unassigned only
	return &ProjectAccess{
		ProjectIDs:        []string{},
		IncludeUnassigned: true, // Only unassigned visible
	}, nil
}

// CanAccessProject checks if user can access a specific project.
func (pa *ProjectAccess) CanAccessProject(projectID string) bool {
	if pa.AllProjects {
		return true
	}
	if projectID == "" {
		return pa.IncludeUnassigned
	}
	for _, id := range pa.ProjectIDs {
		if id == projectID {
			return true
		}
	}
	return false
}

// ApplyToLogFilter applies the project access restrictions to a log filter.
func (pa *ProjectAccess) ApplyToLogFilter(filter *storage.LogFilter, requestedProjectID string) error {
	// If admin with AllProjects, only filter if explicitly requested
	if pa.AllProjects && requestedProjectID == "" {
		return nil // No restriction for admins
	}

	// If a specific project is requested, validate access
	if requestedProjectID != "" {
		if !pa.CanAccessProject(requestedProjectID) {
			return ErrProjectAccessDenied
		}
		filter.ProjectID = requestedProjectID
		return nil
	}

	// Apply multi-project access restriction
	filter.ProjectIDs = pa.ProjectIDs
	filter.IncludeUnassigned = pa.IncludeUnassigned
	return nil
}

// ApplyToAggregationFilter applies the project access restrictions to an aggregation filter.
func (pa *ProjectAccess) ApplyToAggregationFilter(filter *storage.AggregationFilter, requestedProjectID string) error {
	if pa.AllProjects && requestedProjectID == "" {
		return nil
	}

	if requestedProjectID != "" {
		if !pa.CanAccessProject(requestedProjectID) {
			return ErrProjectAccessDenied
		}
		filter.ProjectID = requestedProjectID
		return nil
	}

	filter.ProjectIDs = pa.ProjectIDs
	filter.IncludeUnassigned = pa.IncludeUnassigned
	return nil
}

// ErrProjectAccessDenied is returned when user tries to access a project they don't have access to.
var ErrProjectAccessDenied = &AccessDeniedError{Message: "no access to project"}

// AccessDeniedError represents an access denied error.
type AccessDeniedError struct {
	Message string
}

func (e *AccessDeniedError) Error() string {
	return e.Message
}

package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/good-yellow-bee/blazelog/internal/models"
)

type sqliteProjectRepo struct {
	db *sql.DB
}

func (r *sqliteProjectRepo) Create(ctx context.Context, project *models.Project) error {
	query := `
		INSERT INTO projects (id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		project.ID, project.Name, project.Description,
		project.CreatedAt, project.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

func (r *sqliteProjectRepo) GetByID(ctx context.Context, id string) (*models.Project, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM projects WHERE id = ?
	`
	project := &models.Project{}
	var description sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&project.ID, &project.Name, &description,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		//nolint:nilnil
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project by id: %w", err)
	}
	project.Description = description.String
	return project, nil
}

func (r *sqliteProjectRepo) GetByName(ctx context.Context, name string) (*models.Project, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM projects WHERE name = ?
	`
	project := &models.Project{}
	var description sql.NullString
	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&project.ID, &project.Name, &description,
		&project.CreatedAt, &project.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		//nolint:nilnil
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get project by name: %w", err)
	}
	project.Description = description.String
	return project, nil
}

func (r *sqliteProjectRepo) Update(ctx context.Context, project *models.Project) error {
	query := `
		UPDATE projects SET name = ?, description = ?, updated_at = ?
		WHERE id = ?
	`
	result, err := r.db.ExecContext(ctx, query,
		project.Name, project.Description, project.UpdatedAt,
		project.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("project not found: %s", project.ID)
	}
	return nil
}

func (r *sqliteProjectRepo) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("project not found: %s", id)
	}
	return nil
}

func (r *sqliteProjectRepo) List(ctx context.Context) ([]*models.Project, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM projects ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		project := &models.Project{}
		var description sql.NullString
		err := rows.Scan(
			&project.ID, &project.Name, &description,
			&project.CreatedAt, &project.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		project.Description = description.String
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (r *sqliteProjectRepo) AddUser(ctx context.Context, projectID, userID string, role models.Role) error {
	query := `
		INSERT OR REPLACE INTO project_users (project_id, user_id, role)
		VALUES (?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query, projectID, userID, role)
	if err != nil {
		return fmt.Errorf("add user to project: %w", err)
	}
	return nil
}

func (r *sqliteProjectRepo) RemoveUser(ctx context.Context, projectID, userID string) error {
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM project_users WHERE project_id = ? AND user_id = ?",
		projectID, userID,
	)
	if err != nil {
		return fmt.Errorf("remove user from project: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not in project")
	}
	return nil
}

func (r *sqliteProjectRepo) GetUsers(ctx context.Context, projectID string) ([]*models.User, error) {
	query := `
		SELECT u.id, u.username, u.email, u.password_hash, u.role, u.created_at, u.updated_at
		FROM users u
		INNER JOIN project_users pu ON u.id = pu.user_id
		WHERE pu.project_id = ?
		ORDER BY u.username
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *sqliteProjectRepo) GetProjectMembers(ctx context.Context, projectID string) ([]*models.ProjectMember, error) {
	query := `
		SELECT u.id, u.username, u.email, pu.role
		FROM users u
		INNER JOIN project_users pu ON u.id = pu.user_id
		WHERE pu.project_id = ?
		ORDER BY u.username
	`
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project members: %w", err)
	}
	defer rows.Close()

	var members []*models.ProjectMember
	for rows.Next() {
		member := &models.ProjectMember{}
		err := rows.Scan(&member.UserID, &member.Username, &member.Email, &member.Role)
		if err != nil {
			return nil, fmt.Errorf("scan project member: %w", err)
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *sqliteProjectRepo) GetProjectsForUser(ctx context.Context, userID string) ([]*models.Project, error) {
	query := `
		SELECT p.id, p.name, p.description, p.created_at, p.updated_at
		FROM projects p
		INNER JOIN project_users pu ON p.id = pu.project_id
		WHERE pu.user_id = ?
		ORDER BY p.name
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get user projects: %w", err)
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		project := &models.Project{}
		var description sql.NullString
		err := rows.Scan(
			&project.ID, &project.Name, &description,
			&project.CreatedAt, &project.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		project.Description = description.String
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

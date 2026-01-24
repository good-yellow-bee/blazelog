package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

// defaultDBPath is the default database path, can be overridden via BLAZELOG_DB_PATH env var
var defaultDBPath = "/data/blazelog.db"

func init() {
	if envPath := os.Getenv("BLAZELOG_DB_PATH"); envPath != "" {
		defaultDBPath = envPath
	}
}

var (
	projectDBPath     string
	projectName       string
	projectID         string
	projectDesc       string
	projectNewName    string
	projectForce      bool
	projectUsername   string
	projectUserID     string
	projectMemberRole string
)

// projectCmd represents the project command group
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Project management commands",
	Long: `Commands for managing BlazeLog projects.

Projects are used to organize logs, alerts, and user access.
These commands operate directly on the database file.

Examples:
  # List all projects
  blazectl project list

  # Create a new project
  blazectl project create --name my-project --description "My project"

  # Show project details
  blazectl project show --name my-project

  # List project members
  blazectl project members --name my-project

  # Add a member to a project
  blazectl project add-member --name my-project --username alice --role operator`,
}

// projectListCmd lists all projects
var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	Long: `List all projects in the database.

Displays project ID, name, description, member count, and creation date.

Example:
  blazectl project list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openProjectDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		projects, err := store.Projects().List(ctx)
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}

		if len(projects) == 0 {
			fmt.Println("No projects found.")
			return nil
		}

		// Print header
		fmt.Printf("\n%-36s  %-20s  %-30s  %-8s  %s\n",
			"ID", "NAME", "DESCRIPTION", "MEMBERS", "CREATED")
		fmt.Println(strings.Repeat("-", 110))

		for _, p := range projects {
			members, err := store.Projects().GetProjectMembers(ctx, p.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not fetch members for %s: %v\n", p.Name, err)
			}
			desc := p.Description
			if len(desc) > 28 {
				desc = desc[:28] + ".."
			}
			fmt.Printf("%-36s  %-20s  %-30s  %-8d  %s\n",
				p.ID,
				truncate(p.Name, 20),
				desc,
				len(members),
				p.CreatedAt.Format("2006-01-02 15:04"),
			)
		}
		fmt.Printf("\nTotal: %d project(s)\n", len(projects))

		return nil
	},
}

// projectCreateCmd creates a new project
var projectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new project",
	Long: `Create a new project in the database.

Example:
  blazectl project create --name my-project --description "My project description"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if projectName == "" {
			return fmt.Errorf("--name is required")
		}

		store, err := openProjectDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()

		// Check name uniqueness
		existing, err := store.Projects().GetByName(ctx, strings.TrimSpace(projectName))
		if err != nil {
			return fmt.Errorf("check existing project: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("project name already exists: %s", projectName)
		}

		now := time.Now()
		project := &models.Project{
			ID:          uuid.New().String(),
			Name:        strings.TrimSpace(projectName),
			Description: strings.TrimSpace(projectDesc),
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := store.Projects().Create(ctx, project); err != nil {
			return fmt.Errorf("create project: %w", err)
		}

		fmt.Printf("\nProject created successfully:\n")
		fmt.Printf("  ID:          %s\n", project.ID)
		fmt.Printf("  Name:        %s\n", project.Name)
		fmt.Printf("  Description: %s\n", project.Description)

		return nil
	},
}

// projectShowCmd shows project details
var projectShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show project details",
	Long: `Show detailed information about a project.

You can identify the project by either --name or --id.

Examples:
  blazectl project show --name my-project
  blazectl project show --id 550e8400-e29b-41d4-a716-446655440000`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openProjectDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		project, err := resolveProject(ctx, store.Projects(), projectName, projectID)
		if err != nil {
			return err
		}

		members, err := store.Projects().GetProjectMembers(ctx, project.ID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not fetch members: %v\n", err)
		}

		fmt.Println("\nProject Details:")
		fmt.Printf("  ID:          %s\n", project.ID)
		fmt.Printf("  Name:        %s\n", project.Name)
		fmt.Printf("  Description: %s\n", project.Description)
		fmt.Printf("  Members:     %d\n", len(members))
		fmt.Printf("  Created:     %s\n", project.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Updated:     %s\n", project.UpdatedAt.Format("2006-01-02 15:04:05"))

		return nil
	},
}

// projectUpdateCmd updates a project
var projectUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update project name or description",
	Long: `Update an existing project's name or description.

Examples:
  blazectl project update --name my-project --new-name new-name
  blazectl project update --name my-project --description "New description"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openProjectDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		project, err := resolveProject(ctx, store.Projects(), projectName, projectID)
		if err != nil {
			return err
		}

		// Check if any update is requested
		if projectNewName == "" && !cmd.Flags().Changed("description") {
			return fmt.Errorf("specify --new-name or --description to update")
		}

		if projectNewName != "" {
			// Check uniqueness of new name
			existing, err := store.Projects().GetByName(ctx, strings.TrimSpace(projectNewName))
			if err != nil {
				return fmt.Errorf("check existing project: %w", err)
			}
			if existing != nil && existing.ID != project.ID {
				return fmt.Errorf("project name already exists: %s", projectNewName)
			}
			project.Name = strings.TrimSpace(projectNewName)
		}

		if cmd.Flags().Changed("description") {
			project.Description = strings.TrimSpace(projectDesc)
		}

		project.UpdatedAt = time.Now()

		if err := store.Projects().Update(ctx, project); err != nil {
			return fmt.Errorf("update project: %w", err)
		}

		fmt.Printf("Project updated: %s\n", project.Name)
		return nil
	},
}

// projectDeleteCmd deletes a project
var projectDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a project",
	Long: `Delete a project from the database.

This will also remove all project memberships.
Alerts and other resources may have their project association removed.

Examples:
  blazectl project delete --name my-project
  blazectl project delete --name my-project --force  # skip confirmation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openProjectDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		project, err := resolveProject(ctx, store.Projects(), projectName, projectID)
		if err != nil {
			return err
		}

		if !projectForce {
			fmt.Printf("Delete project '%s'? [y/N]: ", project.Name)
			var confirm string
			fmt.Scanln(&confirm)
			if !strings.EqualFold(confirm, "y") {
				fmt.Println("Canceled.")
				return nil
			}
		}

		if err := store.Projects().Delete(ctx, project.ID); err != nil {
			return fmt.Errorf("delete project: %w", err)
		}

		fmt.Printf("Project deleted: %s\n", project.Name)
		return nil
	},
}

// projectMembersCmd lists project members
var projectMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "List project members",
	Long: `List all members of a project.

Examples:
  blazectl project members --name my-project
  blazectl project members --id 550e8400-e29b-41d4-a716-446655440000`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openProjectDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		project, err := resolveProject(ctx, store.Projects(), projectName, projectID)
		if err != nil {
			return err
		}

		members, err := store.Projects().GetProjectMembers(ctx, project.ID)
		if err != nil {
			return fmt.Errorf("get members: %w", err)
		}

		fmt.Printf("\nMembers of project '%s':\n\n", project.Name)

		if len(members) == 0 {
			fmt.Println("No members found.")
			return nil
		}

		fmt.Printf("%-36s  %-20s  %-30s  %s\n", "USER ID", "USERNAME", "EMAIL", "ROLE")
		fmt.Println(strings.Repeat("-", 100))

		for _, m := range members {
			fmt.Printf("%-36s  %-20s  %-30s  %s\n",
				m.UserID, m.Username, m.Email, m.Role)
		}
		fmt.Printf("\nTotal: %d member(s)\n", len(members))

		return nil
	},
}

// projectAddMemberCmd adds a member to a project
var projectAddMemberCmd = &cobra.Command{
	Use:   "add-member",
	Short: "Add or update a project member",
	Long: `Add a user to a project or update their role.

If the user is already a member, their role will be updated.

Available roles:
  - admin: Full access within the project
  - operator: Can modify resources
  - viewer: Read-only access

Examples:
  blazectl project add-member --name my-project --username alice --role admin
  blazectl project add-member --name my-project --user-id abc123 --role operator`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openProjectDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		project, err := resolveProject(ctx, store.Projects(), projectName, projectID)
		if err != nil {
			return err
		}

		user, err := resolveUser(ctx, store.Users(), projectUsername, projectUserID)
		if err != nil {
			return err
		}

		role := models.Role(projectMemberRole)
		if role != models.RoleAdmin && role != models.RoleOperator && role != models.RoleViewer {
			return fmt.Errorf("invalid role: %s (use: admin, operator, viewer)", projectMemberRole)
		}

		if err := store.Projects().AddUser(ctx, project.ID, user.ID, role); err != nil {
			return fmt.Errorf("add member: %w", err)
		}

		fmt.Printf("Added %s to project '%s' as %s\n", user.Username, project.Name, role)
		return nil
	},
}

// projectRemoveMemberCmd removes a member from a project
var projectRemoveMemberCmd = &cobra.Command{
	Use:   "remove-member",
	Short: "Remove a member from project",
	Long: `Remove a user from a project.

Examples:
  blazectl project remove-member --name my-project --username alice
  blazectl project remove-member --name my-project --user-id abc123`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openProjectDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		project, err := resolveProject(ctx, store.Projects(), projectName, projectID)
		if err != nil {
			return err
		}

		user, err := resolveUser(ctx, store.Users(), projectUsername, projectUserID)
		if err != nil {
			return err
		}

		if err := store.Projects().RemoveUser(ctx, project.ID, user.ID); err != nil {
			return fmt.Errorf("remove member: %w", err)
		}

		fmt.Printf("Removed %s from project '%s'\n", user.Username, project.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectShowCmd)
	projectCmd.AddCommand(projectUpdateCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectMembersCmd)
	projectCmd.AddCommand(projectAddMemberCmd)
	projectCmd.AddCommand(projectRemoveMemberCmd)

	// DB flag for all commands (optional, defaults to ./data/blazelog.db)
	allCmds := []*cobra.Command{
		projectListCmd, projectCreateCmd, projectShowCmd,
		projectUpdateCmd, projectDeleteCmd, projectMembersCmd,
		projectAddMemberCmd, projectRemoveMemberCmd,
	}
	for _, cmd := range allCmds {
		cmd.Flags().StringVar(&projectDBPath, "db", defaultDBPath, "path to SQLite database file")
	}

	// Create flags
	projectCreateCmd.Flags().StringVar(&projectName, "name", "", "project name (required)")
	projectCreateCmd.Flags().StringVar(&projectDesc, "description", "", "project description")
	projectCreateCmd.MarkFlagRequired("name")

	// Show flags
	projectShowCmd.Flags().StringVar(&projectName, "name", "", "project name")
	projectShowCmd.Flags().StringVar(&projectID, "id", "", "project ID")

	// Update flags
	projectUpdateCmd.Flags().StringVar(&projectName, "name", "", "project name")
	projectUpdateCmd.Flags().StringVar(&projectID, "id", "", "project ID")
	projectUpdateCmd.Flags().StringVar(&projectNewName, "new-name", "", "new project name")
	projectUpdateCmd.Flags().StringVar(&projectDesc, "description", "", "new project description")

	// Delete flags
	projectDeleteCmd.Flags().StringVar(&projectName, "name", "", "project name")
	projectDeleteCmd.Flags().StringVar(&projectID, "id", "", "project ID")
	projectDeleteCmd.Flags().BoolVar(&projectForce, "force", false, "skip confirmation prompt")

	// Members flags
	projectMembersCmd.Flags().StringVar(&projectName, "name", "", "project name")
	projectMembersCmd.Flags().StringVar(&projectID, "id", "", "project ID")

	// Add-member flags
	projectAddMemberCmd.Flags().StringVar(&projectName, "name", "", "project name")
	projectAddMemberCmd.Flags().StringVar(&projectID, "id", "", "project ID")
	projectAddMemberCmd.Flags().StringVar(&projectUsername, "username", "", "username to add")
	projectAddMemberCmd.Flags().StringVar(&projectUserID, "user-id", "", "user ID to add")
	projectAddMemberCmd.Flags().StringVar(&projectMemberRole, "role", "viewer", "role: admin, operator, viewer")

	// Remove-member flags
	projectRemoveMemberCmd.Flags().StringVar(&projectName, "name", "", "project name")
	projectRemoveMemberCmd.Flags().StringVar(&projectID, "id", "", "project ID")
	projectRemoveMemberCmd.Flags().StringVar(&projectUsername, "username", "", "username to remove")
	projectRemoveMemberCmd.Flags().StringVar(&projectUserID, "user-id", "", "user ID to remove")
}

// openProjectDB opens the SQLite database with default path.
func openProjectDB() (*storage.SQLiteStorage, error) {
	dbKey := os.Getenv("BLAZELOG_DB_KEY")
	if dbKey == "" {
		return nil, fmt.Errorf("BLAZELOG_DB_KEY environment variable is required")
	}
	masterKey := []byte(os.Getenv("BLAZELOG_MASTER_KEY"))

	store := storage.NewSQLiteStorage(projectDBPath, masterKey, []byte(dbKey))
	if err := store.Open(); err != nil {
		return nil, fmt.Errorf("open database at %s: %w", projectDBPath, err)
	}
	return store, nil
}

// resolveProject finds a project by name or ID (ID takes precedence).
func resolveProject(ctx context.Context, repo storage.ProjectRepository, name, id string) (*models.Project, error) {
	if id == "" && name == "" {
		return nil, fmt.Errorf("specify --name or --id")
	}
	if id != "" {
		p, err := repo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get project: %w", err)
		}
		if p == nil {
			return nil, fmt.Errorf("project not found: %s", id)
		}
		return p, nil
	}
	p, err := repo.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	if p == nil {
		return nil, fmt.Errorf("project not found: %s", name)
	}
	return p, nil
}

// resolveUser finds a user by username or ID.
func resolveUser(ctx context.Context, repo storage.UserRepository, username, userID string) (*models.User, error) {
	if userID == "" && username == "" {
		return nil, fmt.Errorf("specify --username or --user-id")
	}
	if userID != "" {
		u, err := repo.GetByID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("get user: %w", err)
		}
		if u == nil {
			return nil, fmt.Errorf("user not found: %s", userID)
		}
		return u, nil
	}
	u, err := repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if u == nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	return u, nil
}

// truncate truncates a string to the given length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-2] + ".."
}

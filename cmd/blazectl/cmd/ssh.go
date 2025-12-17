package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/ssh"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

var (
	sshDBPath      string
	sshName        string
	sshID          string
	sshHost        string
	sshPort        int
	sshUser        string
	sshKeyFile     string
	sshProjectName string
	sshProjectID   string
	sshForce       bool
)

// sshCmd represents the ssh command group
var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "SSH connection management commands",
	Long: `Commands for managing SSH connections.

SSH connections are used for pulling logs from remote servers.
These commands operate directly on the database file.

Examples:
  # List all SSH connections
  blazectl ssh list

  # Create a new SSH connection
  blazectl ssh create --name prod-web --host web.example.com --user loguser --project myapp

  # Show connection details
  blazectl ssh show --name prod-web

  # Test a connection
  blazectl ssh test --name prod-web

  # Delete a connection
  blazectl ssh delete --name prod-web`,
}

// sshListCmd lists all SSH connections
var sshListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all SSH connections",
	Long: `List all SSH connections in the database.

Use --project to filter by project.

Examples:
  blazectl ssh list
  blazectl ssh list --project myapp`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openSSHDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()

		var connections []*models.Connection
		var projectFilter string

		// Filter by project if specified
		if sshProjectName != "" || sshProjectID != "" {
			project, err := resolveProject(ctx, store.Projects(), sshProjectName, sshProjectID)
			if err != nil {
				return err
			}
			connections, err = store.Connections().ListByProject(ctx, project.ID)
			if err != nil {
				return fmt.Errorf("list connections: %w", err)
			}
			projectFilter = project.Name
		} else {
			connections, err = store.Connections().List(ctx)
			if err != nil {
				return fmt.Errorf("list connections: %w", err)
			}
		}

		if len(connections) == 0 {
			if projectFilter != "" {
				fmt.Printf("No SSH connections found in project '%s'.\n", projectFilter)
			} else {
				fmt.Println("No SSH connections found.")
			}
			return nil
		}

		// Print header
		fmt.Printf("\n%-36s  %-20s  %-25s  %-5s  %-12s  %s\n",
			"ID", "NAME", "HOST", "PORT", "STATUS", "PROJECT")
		fmt.Println(strings.Repeat("-", 120))

		for _, c := range connections {
			projectName := ""
			if c.ProjectID != "" {
				p, _ := store.Projects().GetByID(ctx, c.ProjectID)
				if p != nil {
					projectName = truncate(p.Name, 15)
				}
			}
			fmt.Printf("%-36s  %-20s  %-25s  %-5d  %-12s  %s\n",
				c.ID,
				truncate(c.Name, 20),
				truncate(c.Host, 25),
				c.Port,
				c.Status,
				projectName,
			)
		}
		fmt.Printf("\nTotal: %d connection(s)\n", len(connections))

		return nil
	},
}

// sshCreateCmd creates a new SSH connection
var sshCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new SSH connection",
	Long: `Create a new SSH connection in the database.

The private key passphrase (if needed) will be prompted interactively.

Examples:
  blazectl ssh create --name prod-web --host web.example.com --user loguser --project myapp
  blazectl ssh create --name prod-web --host web.example.com:2222 --user loguser --project myapp --key-file ~/.ssh/id_ed25519`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if sshName == "" {
			return fmt.Errorf("--name is required")
		}
		if sshHost == "" {
			return fmt.Errorf("--host is required")
		}
		if sshUser == "" {
			return fmt.Errorf("--user is required")
		}
		if sshProjectName == "" && sshProjectID == "" {
			return fmt.Errorf("--project is required")
		}

		store, err := openSSHDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()

		// Resolve project
		project, err := resolveProject(ctx, store.Projects(), sshProjectName, sshProjectID)
		if err != nil {
			return err
		}

		// Check name uniqueness
		existing, _ := store.Connections().GetByName(ctx, strings.TrimSpace(sshName))
		if existing != nil {
			return fmt.Errorf("connection name already exists: %s", sshName)
		}

		// Prepare credentials
		credentials := map[string]string{
			"user": sshUser,
		}
		if sshKeyFile != "" {
			credentials["key_file"] = sshKeyFile
			// Prompt for passphrase if key file is specified
			fmt.Print("Enter key passphrase (leave empty if none): ")
			passphrase, _ := promptPasswordOptional()
			if passphrase != "" {
				credentials["passphrase"] = passphrase
			}
		} else {
			// Prompt for password
			password, err := promptPassword("Enter SSH password: ")
			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			credentials["password"] = password
		}

		// Encrypt credentials
		credJSON, err := json.Marshal(credentials)
		if err != nil {
			return fmt.Errorf("marshal credentials: %w", err)
		}
		encryptedCreds, err := store.Connections().EncryptCredentials(credJSON)
		if err != nil {
			return fmt.Errorf("encrypt credentials: %w", err)
		}

		now := time.Now()
		conn := &models.Connection{
			ID:                   uuid.New().String(),
			Name:                 strings.TrimSpace(sshName),
			Type:                 models.ConnectionTypeSSH,
			Host:                 strings.TrimSpace(sshHost),
			Port:                 sshPort,
			User:                 strings.TrimSpace(sshUser),
			CredentialsEncrypted: encryptedCreds,
			Status:               models.ConnectionStatusUnknown,
			ProjectID:            project.ID,
			CreatedAt:            now,
			UpdatedAt:            now,
		}

		if err := store.Connections().Create(ctx, conn); err != nil {
			return fmt.Errorf("create connection: %w", err)
		}

		fmt.Printf("\nSSH connection created successfully:\n")
		fmt.Printf("  ID:      %s\n", conn.ID)
		fmt.Printf("  Name:    %s\n", conn.Name)
		fmt.Printf("  Host:    %s\n", conn.Host)
		fmt.Printf("  Port:    %d\n", conn.Port)
		fmt.Printf("  User:    %s\n", conn.User)
		fmt.Printf("  Project: %s\n", project.Name)

		return nil
	},
}

// sshShowCmd shows SSH connection details
var sshShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show SSH connection details",
	Long: `Show detailed information about an SSH connection.

Examples:
  blazectl ssh show --name prod-web
  blazectl ssh show --id 550e8400-e29b-41d4-a716-446655440000`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openSSHDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		conn, err := resolveConnection(ctx, store.Connections(), sshName, sshID)
		if err != nil {
			return err
		}

		projectName := ""
		if conn.ProjectID != "" {
			p, _ := store.Projects().GetByID(ctx, conn.ProjectID)
			if p != nil {
				projectName = p.Name
			}
		}

		fmt.Println("\nSSH Connection Details:")
		fmt.Printf("  ID:          %s\n", conn.ID)
		fmt.Printf("  Name:        %s\n", conn.Name)
		fmt.Printf("  Type:        %s\n", conn.Type)
		fmt.Printf("  Host:        %s\n", conn.Host)
		fmt.Printf("  Port:        %d\n", conn.Port)
		fmt.Printf("  User:        %s\n", conn.User)
		fmt.Printf("  Status:      %s\n", conn.Status)
		fmt.Printf("  Project:     %s\n", projectName)
		if conn.LastTestedAt != nil {
			fmt.Printf("  Last Tested: %s\n", conn.LastTestedAt.Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("  Created:     %s\n", conn.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Updated:     %s\n", conn.UpdatedAt.Format("2006-01-02 15:04:05"))

		return nil
	},
}

// sshDeleteCmd deletes an SSH connection
var sshDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an SSH connection",
	Long: `Delete an SSH connection from the database.

Examples:
  blazectl ssh delete --name prod-web
  blazectl ssh delete --name prod-web --force  # skip confirmation`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openSSHDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		conn, err := resolveConnection(ctx, store.Connections(), sshName, sshID)
		if err != nil {
			return err
		}

		if !sshForce {
			fmt.Printf("Delete SSH connection '%s'? [y/N]: ", conn.Name)
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Canceled.")
				return nil
			}
		}

		if err := store.Connections().Delete(ctx, conn.ID); err != nil {
			return fmt.Errorf("delete connection: %w", err)
		}

		fmt.Printf("SSH connection deleted: %s\n", conn.Name)
		return nil
	},
}

// sshTestCmd tests an SSH connection
var sshTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test an SSH connection",
	Long: `Test an SSH connection to verify connectivity.

Examples:
  blazectl ssh test --name prod-web`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openSSHDB()
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		conn, err := resolveConnection(ctx, store.Connections(), sshName, sshID)
		if err != nil {
			return err
		}

		fmt.Printf("Testing SSH connection '%s' (%s@%s:%d)...\n", conn.Name, conn.User, conn.Host, conn.Port)

		// Decrypt credentials
		credJSON, err := store.Connections().DecryptCredentials(conn.CredentialsEncrypted)
		if err != nil {
			return fmt.Errorf("decrypt credentials: %w", err)
		}

		var creds map[string]string
		if err := json.Unmarshal(credJSON, &creds); err != nil {
			return fmt.Errorf("parse credentials: %w", err)
		}

		// Create SSH client config
		sshConfig := &ssh.ClientConfig{
			Host:    fmt.Sprintf("%s:%d", conn.Host, conn.Port),
			User:    conn.User,
			Timeout: 10 * time.Second,
		}

		if keyFile, ok := creds["key_file"]; ok && keyFile != "" {
			sshConfig.KeyFile = keyFile
			if passphrase, ok := creds["passphrase"]; ok {
				sshConfig.KeyPassphrase = passphrase
			}
		} else if password, ok := creds["password"]; ok {
			sshConfig.Password = password
		}

		// Create client and test connection
		client := ssh.NewClient(sshConfig)
		if err := client.Connect(ctx); err != nil {
			// Update status to failed
			now := time.Now()
			store.Connections().UpdateStatus(ctx, conn.ID, models.ConnectionStatusFailed, now)
			return fmt.Errorf("connection failed: %w", err)
		}
		defer client.Close()

		// Update status to connected
		now := time.Now()
		store.Connections().UpdateStatus(ctx, conn.ID, models.ConnectionStatusConnected, now)

		fmt.Printf("✅ Connection successful!\n")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)
	sshCmd.AddCommand(sshListCmd)
	sshCmd.AddCommand(sshCreateCmd)
	sshCmd.AddCommand(sshShowCmd)
	sshCmd.AddCommand(sshDeleteCmd)
	sshCmd.AddCommand(sshTestCmd)

	// DB flag for all commands
	allCmds := []*cobra.Command{
		sshListCmd, sshCreateCmd, sshShowCmd, sshDeleteCmd, sshTestCmd,
	}
	for _, cmd := range allCmds {
		cmd.Flags().StringVar(&sshDBPath, "db", defaultDBPath, "path to SQLite database file")
	}

	// List flags
	sshListCmd.Flags().StringVar(&sshProjectName, "project", "", "filter by project name")
	sshListCmd.Flags().StringVar(&sshProjectID, "project-id", "", "filter by project ID")

	// Create flags
	sshCreateCmd.Flags().StringVar(&sshName, "name", "", "connection name (required)")
	sshCreateCmd.Flags().StringVar(&sshHost, "host", "", "SSH host address (required)")
	sshCreateCmd.Flags().IntVar(&sshPort, "port", 22, "SSH port (default: 22)")
	sshCreateCmd.Flags().StringVar(&sshUser, "user", "", "SSH username (required)")
	sshCreateCmd.Flags().StringVar(&sshKeyFile, "key-file", "", "path to private key file")
	sshCreateCmd.Flags().StringVar(&sshProjectName, "project", "", "project name (required)")
	sshCreateCmd.Flags().StringVar(&sshProjectID, "project-id", "", "project ID")
	sshCreateCmd.MarkFlagRequired("name")
	sshCreateCmd.MarkFlagRequired("host")
	sshCreateCmd.MarkFlagRequired("user")

	// Show flags
	sshShowCmd.Flags().StringVar(&sshName, "name", "", "connection name")
	sshShowCmd.Flags().StringVar(&sshID, "id", "", "connection ID")

	// Delete flags
	sshDeleteCmd.Flags().StringVar(&sshName, "name", "", "connection name")
	sshDeleteCmd.Flags().StringVar(&sshID, "id", "", "connection ID")
	sshDeleteCmd.Flags().BoolVar(&sshForce, "force", false, "skip confirmation prompt")

	// Test flags
	sshTestCmd.Flags().StringVar(&sshName, "name", "", "connection name")
	sshTestCmd.Flags().StringVar(&sshID, "id", "", "connection ID")
}

// openSSHDB opens the SQLite database.
func openSSHDB() (*storage.SQLiteStorage, error) {
	// Get master key from environment for credential encryption
	var masterKey []byte
	if key := os.Getenv("BLAZELOG_MASTER_KEY"); key != "" {
		masterKey = []byte(key)
	}

	store := storage.NewSQLiteStorage(sshDBPath, masterKey)
	if err := store.Open(); err != nil {
		return nil, fmt.Errorf("open database at %s: %w", sshDBPath, err)
	}
	return store, nil
}

// resolveConnection finds a connection by name or ID.
func resolveConnection(ctx context.Context, repo storage.ConnectionRepository, name, id string) (*models.Connection, error) {
	if id == "" && name == "" {
		return nil, fmt.Errorf("specify --name or --id")
	}
	if id != "" {
		c, err := repo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get connection: %w", err)
		}
		if c == nil {
			return nil, fmt.Errorf("connection not found: %s", id)
		}
		return c, nil
	}
	c, err := repo.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get connection: %w", err)
	}
	if c == nil {
		return nil, fmt.Errorf("connection not found: %s", name)
	}
	return c, nil
}

// promptPasswordOptional prompts for a password, allowing empty input.
func promptPasswordOptional() (string, error) {
	fd := syscall.Stdin
	if term.IsTerminal(fd) {
		passwordBytes, err := term.ReadPassword(fd)
		fmt.Println()
		if err != nil {
			return "", err
		}
		return string(passwordBytes), nil
	}
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(password), nil
}

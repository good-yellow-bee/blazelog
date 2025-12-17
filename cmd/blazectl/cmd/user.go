package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/good-yellow-bee/blazelog/internal/api/auth"
	"github.com/good-yellow-bee/blazelog/internal/api/users"
	"github.com/good-yellow-bee/blazelog/internal/models"
	"github.com/good-yellow-bee/blazelog/internal/storage"
)

var (
	userDBPath   string
	userUsername string
	userEmail    string
	userRole     string
)

// userCmd represents the user command group
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User management commands",
	Long: `Commands for managing BlazeLog users.

These commands operate directly on the database file and are intended
for system administrators to manage users outside of the web interface.

Examples:
  # List all users
  blazectl user list

  # Create an admin user
  blazectl user create --username admin --email admin@example.com --role admin

  # Change a user's password
  blazectl user passwd --username admin`,
}

// userListCmd lists all users
var userListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all users",
	Long: `List all users in the database.

Displays username, email, role, and creation date for each user.
Passwords are never displayed.

Example:
  blazectl user list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := openDatabase(userDBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()
		userList, err := store.Users().List(ctx)
		if err != nil {
			return fmt.Errorf("list users: %w", err)
		}

		if len(userList) == 0 {
			fmt.Println("No users found.")
			return nil
		}

		// Print header
		fmt.Printf("\n%-36s  %-20s  %-30s  %-10s  %s\n",
			"ID", "USERNAME", "EMAIL", "ROLE", "CREATED")
		fmt.Println(strings.Repeat("-", 120))

		for _, u := range userList {
			fmt.Printf("%-36s  %-20s  %-30s  %-10s  %s\n",
				u.ID,
				u.Username,
				u.Email,
				u.Role,
				u.CreatedAt.Format("2006-01-02 15:04:05"),
			)
		}
		fmt.Printf("\nTotal: %d user(s)\n", len(userList))

		return nil
	},
}

// userCreateCmd creates a new user
var userCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new user",
	Long: `Create a new user in the database.

The password will be prompted interactively for security reasons
(to avoid exposing it in shell history).

Password requirements:
  - Minimum 12 characters
  - At least 1 uppercase letter (A-Z)
  - At least 1 lowercase letter (a-z)
  - At least 1 digit (0-9)
  - At least 1 special character (!@#$%^&*...)

Available roles:
  - admin: Full access to all features
  - operator: Can modify resources but not manage users
  - viewer: Read-only access

Example:
  blazectl user create --username john --email john@example.com --role operator`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateCreateFlags(); err != nil {
			return err
		}

		// Validate username
		if err := users.ValidateUsername(userUsername); err != nil {
			return fmt.Errorf("invalid username: %w", err)
		}

		// Validate email
		if err := users.ValidateEmail(userEmail); err != nil {
			return fmt.Errorf("invalid email: %w", err)
		}

		// Validate role
		role, err := users.ValidateRole(userRole)
		if err != nil {
			return fmt.Errorf("invalid role: %w", err)
		}

		// Prompt for password securely
		password, err := promptPassword("Enter password: ")
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}

		// Validate password
		if err := auth.ValidatePassword(password); err != nil {
			return fmt.Errorf("invalid password: %w", err)
		}

		// Confirm password
		confirmPassword, err := promptPassword("Confirm password: ")
		if err != nil {
			return fmt.Errorf("read password confirmation: %w", err)
		}

		if password != confirmPassword {
			return fmt.Errorf("passwords do not match")
		}

		// Open database
		store, err := openDatabase(userDBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()

		// Check if username already exists
		existing, err := store.Users().GetByUsername(ctx, userUsername)
		if err != nil {
			return fmt.Errorf("check username: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("username '%s' already exists", userUsername)
		}

		// Check if email already exists
		existing, err = store.Users().GetByEmail(ctx, userEmail)
		if err != nil {
			return fmt.Errorf("check email: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("email '%s' already exists", userEmail)
		}

		// Hash password
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}

		// Create user
		now := time.Now()
		user := &models.User{
			ID:           uuid.New().String(),
			Username:     strings.TrimSpace(userUsername),
			Email:        strings.TrimSpace(userEmail),
			PasswordHash: string(hash),
			Role:         role,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := store.Users().Create(ctx, user); err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		fmt.Printf("\nUser created successfully:\n")
		fmt.Printf("  ID:       %s\n", user.ID)
		fmt.Printf("  Username: %s\n", user.Username)
		fmt.Printf("  Email:    %s\n", user.Email)
		fmt.Printf("  Role:     %s\n", user.Role)

		return nil
	},
}

// userPasswdCmd changes a user's password
var userPasswdCmd = &cobra.Command{
	Use:   "passwd",
	Short: "Change a user's password",
	Long: `Change the password for an existing user.

The new password will be prompted interactively for security reasons
(to avoid exposing it in shell history).

Password requirements:
  - Minimum 12 characters
  - At least 1 uppercase letter (A-Z)
  - At least 1 lowercase letter (a-z)
  - At least 1 digit (0-9)
  - At least 1 special character (!@#$%^&*...)

Example:
  blazectl user passwd --username admin`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if userUsername == "" {
			return fmt.Errorf("--username is required")
		}

		// Open database
		store, err := openDatabase(userDBPath)
		if err != nil {
			return err
		}
		defer store.Close()

		ctx := context.Background()

		// Find user
		user, err := store.Users().GetByUsername(ctx, userUsername)
		if err != nil {
			return fmt.Errorf("find user: %w", err)
		}
		if user == nil {
			return fmt.Errorf("user '%s' not found", userUsername)
		}

		// Prompt for new password
		password, err := promptPassword("Enter new password: ")
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}

		// Validate password
		if err := auth.ValidatePassword(password); err != nil {
			return fmt.Errorf("invalid password: %w", err)
		}

		// Confirm password
		confirmPassword, err := promptPassword("Confirm new password: ")
		if err != nil {
			return fmt.Errorf("read password confirmation: %w", err)
		}

		if password != confirmPassword {
			return fmt.Errorf("passwords do not match")
		}

		// Hash new password
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}

		// Update user
		user.PasswordHash = string(hash)
		user.UpdatedAt = time.Now()

		if err := store.Users().Update(ctx, user); err != nil {
			return fmt.Errorf("update user: %w", err)
		}

		// Revoke all refresh tokens for this user (force re-login)
		if err := store.Tokens().RevokeAllForUser(ctx, user.ID); err != nil {
			// Log warning but don't fail - password was already changed
			PrintVerbose("Warning: could not revoke existing sessions: %v", err)
		}

		fmt.Printf("\nPassword changed successfully for user '%s'.\n", user.Username)
		fmt.Println("All existing sessions have been revoked.")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userListCmd)
	userCmd.AddCommand(userCreateCmd)
	userCmd.AddCommand(userPasswdCmd)

	// Common flags (db has default value)
	for _, cmd := range []*cobra.Command{userListCmd, userCreateCmd, userPasswdCmd} {
		cmd.Flags().StringVar(&userDBPath, "db", defaultDBPath, "path to SQLite database file")
	}

	// Create-specific flags
	userCreateCmd.Flags().StringVar(&userUsername, "username", "", "username for the new user (required)")
	userCreateCmd.Flags().StringVar(&userEmail, "email", "", "email for the new user (required)")
	userCreateCmd.Flags().StringVar(&userRole, "role", "viewer", "role: admin, operator, or viewer (default: viewer)")
	userCreateCmd.MarkFlagRequired("username")
	userCreateCmd.MarkFlagRequired("email")

	// Passwd-specific flags
	userPasswdCmd.Flags().StringVar(&userUsername, "username", "", "username of the user to update (required)")
	userPasswdCmd.MarkFlagRequired("username")
}

func validateCreateFlags() error {
	if userUsername == "" {
		return fmt.Errorf("--username is required")
	}
	if userEmail == "" {
		return fmt.Errorf("--email is required")
	}
	return nil
}

// openDatabase opens the SQLite database.
func openDatabase(path string) (*storage.SQLiteStorage, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("database file not found: %s", path)
	}

	store := storage.NewSQLiteStorage(path, nil)
	if err := store.Open(); err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return store, nil
}

// promptPassword prompts for a password without echoing to the terminal.
func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)

	// Check if stdin is a terminal
	fd := syscall.Stdin
	if term.IsTerminal(fd) {
		// Read password without echo
		passwordBytes, err := term.ReadPassword(fd)
		fmt.Println() // Add newline after password input
		if err != nil {
			return "", err
		}
		return string(passwordBytes), nil
	}

	// Fallback for non-terminal input (e.g., piped input)
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(password), nil
}

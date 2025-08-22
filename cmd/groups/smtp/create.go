package smtp

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the smtp create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new SMTP credential",
		Long: `Create a new SMTP credential for sending emails via SMTP protocol.

When creating an SMTP credential, you can choose between:
- Global scope: Can send from any verified domain
- Scoped: Can only send from specified domains

A secure password will be generated automatically. Make sure to save it
as it will only be shown once and cannot be retrieved later.`,
		Example: `  # Create global SMTP credential interactively
  ahasend smtp create

  # Create with specific name
  ahasend smtp create --name "Production Server"

  # Create scoped credential for specific domains
  ahasend smtp create --name "Marketing" --scope scoped --domains "marketing.example.com,news.example.com"

  # Create sandbox credential for testing
  ahasend smtp create --name "Test Server" --sandbox

  # Create with custom username
  ahasend smtp create --name "API Server" --username "api-server-smtp"`,
		RunE: runSMTPCreate,
	}

	// Add flags
	cmd.Flags().String("name", "", "Credential name (required)")
	cmd.Flags().String("username", "", "SMTP username (auto-generated if not provided)")
	cmd.Flags().String("password", "", "SMTP password (auto-generated if not provided)")
	cmd.Flags().String("scope", "global", "Credential scope (global or scoped)")
	cmd.Flags().StringSlice("domains", []string{}, "Allowed domains for scoped credentials (comma-separated)")
	cmd.Flags().Bool("sandbox", false, "Create as sandbox credential for testing")
	cmd.Flags().Bool("non-interactive", false, "Disable interactive prompts")

	return cmd
}

func runSMTPCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flag values
	name, _ := cmd.Flags().GetString("name")
	username, _ := cmd.Flags().GetString("username")
	password, _ := cmd.Flags().GetString("password")
	scope, _ := cmd.Flags().GetString("scope")
	domains, _ := cmd.Flags().GetStringSlice("domains")
	sandbox, _ := cmd.Flags().GetBool("sandbox")
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

	// Interactive mode if name not provided
	if name == "" && !nonInteractive {
		name, username, password, scope, domains, sandbox, err = promptSMTPCredentialDetails()
		if err != nil {
			return err
		}
	}

	// Validate required fields
	if name == "" {
		return errors.NewValidationError("credential name is required", nil)
	}

	// Validate scope
	if scope != "global" && scope != "scoped" {
		return errors.NewValidationError("scope must be 'global' or 'scoped'", nil)
	}

	// Validate domains for scoped credentials
	if scope == "scoped" && len(domains) == 0 {
		return errors.NewValidationError("domains are required for scoped credentials", nil)
	}

	// Generate username if not provided
	if username == "" {
		username = generateUsername(name)
	}

	// Generate secure password if not provided
	if password == "" {
		password = generateSecurePassword()
	}

	logger.Get().WithFields(map[string]interface{}{
		"name":     name,
		"username": username,
		"scope":    scope,
		"sandbox":  sandbox,
		"domains":  domains,
	}).Debug("Creating SMTP credential")

	// Create the request
	req := requests.CreateSMTPCredentialRequest{
		Name:     name,
		Username: username,
		Password: password,
		Scope:    scope,
		Sandbox:  sandbox,
		Domains:  domains,
	}

	// Create SMTP credential
	credential, err := client.CreateSMTPCredential(req)
	if err != nil {
		return handler.HandleError(err)
	}

	if credential == nil {
		return handler.HandleError(errors.NewAPIError("received nil response from API", nil))
	}

	// Use the new ResponseHandler to display created SMTP credential
	return handler.HandleCreateSMTP(credential, printer.CreateConfig{
		SuccessMessage: "SMTP credential created successfully",
		ItemName:       "smtp_credential",
		FieldOrder:     []string{"id", "name", "username", "scope", "domains", "sandbox", "created_at", "updated_at"},
	})
}

func promptSMTPCredentialDetails() (name, username, password, scope string, domains []string, sandbox bool, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Create SMTP Credential")
	fmt.Println("======================")

	// Prompt for name
	fmt.Print("Credential name: ")
	name, err = reader.ReadString('\n')
	if err != nil {
		return "", "", "", "", nil, false, fmt.Errorf("failed to read name: %w", err)
	}
	name = strings.TrimSpace(name)

	// Prompt for username (optional)
	fmt.Printf("Username (leave empty for auto-generated): ")
	username, err = reader.ReadString('\n')
	if err != nil {
		return "", "", "", "", nil, false, fmt.Errorf("failed to read username: %w", err)
	}
	username = strings.TrimSpace(username)

	// Prompt for scope
	fmt.Print("Scope (global/scoped) [global]: ")
	scope, err = reader.ReadString('\n')
	if err != nil {
		return "", "", "", "", nil, false, fmt.Errorf("failed to read scope: %w", err)
	}
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "global"
	}

	// Prompt for domains if scoped
	if scope == "scoped" {
		fmt.Print("Allowed domains (comma-separated): ")
		domainsStr, err := reader.ReadString('\n')
		if err != nil {
			return "", "", "", "", nil, false, fmt.Errorf("failed to read domains: %w", err)
		}
		domainsStr = strings.TrimSpace(domainsStr)
		if domainsStr != "" {
			for _, d := range strings.Split(domainsStr, ",") {
				domain := strings.TrimSpace(d)
				if domain != "" {
					domains = append(domains, domain)
				}
			}
		}
	}

	// Prompt for sandbox mode
	fmt.Print("Sandbox mode for testing? (y/N): ")
	sandboxStr, err := reader.ReadString('\n')
	if err != nil {
		return "", "", "", "", nil, false, fmt.Errorf("failed to read sandbox mode: %w", err)
	}
	sandboxStr = strings.ToLower(strings.TrimSpace(sandboxStr))
	sandbox = sandboxStr == "y" || sandboxStr == "yes"

	// Password will be auto-generated
	fmt.Println("\nA secure password will be generated automatically.")

	return name, username, password, scope, domains, sandbox, nil
}

func generateUsername(name string) string {
	// Create a username from the name
	base := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	// Remove any special characters
	var result strings.Builder
	for _, ch := range base {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			result.WriteRune(ch)
		}
	}

	// Add a random suffix for uniqueness
	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	suffix := base64.RawURLEncoding.EncodeToString(randomBytes)[:4]

	return fmt.Sprintf("%s-%s", result.String(), suffix)
}

func generateSecurePassword() string {
	// Generate a secure random password
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a less secure method if crypto/rand fails
		return fmt.Sprintf("AhaSend-%d-Pass", os.Getpid())
	}

	// Use URL-safe base64 encoding for the password
	return base64.RawURLEncoding.EncodeToString(bytes)
}

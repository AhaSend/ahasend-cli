package smtp

import (
	"bufio"
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
  ahasend smtp create --name "onboarding" --scope scoped --domains "onboarding.example.com,drip.example.com"

  # Create sandbox credential for testing
  ahasend smtp create --name "Test Server" --sandbox
`,
		RunE: runSMTPCreate,
	}

	// Add flags
	cmd.Flags().String("name", "", "Credential name (required)")
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
	scope, _ := cmd.Flags().GetString("scope")
	domains, _ := cmd.Flags().GetStringSlice("domains")
	sandbox, _ := cmd.Flags().GetBool("sandbox")
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")

	// Interactive mode if name not provided
	if name == "" && !nonInteractive {
		name, scope, domains, sandbox, err = promptSMTPCredentialDetails()
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

	logger.Get().WithFields(map[string]interface{}{
		"name":    name,
		"scope":   scope,
		"sandbox": sandbox,
		"domains": domains,
	}).Debug("Creating SMTP credential")

	// Create the request
	req := requests.CreateSMTPCredentialRequest{
		Name:    name,
		Scope:   scope,
		Sandbox: sandbox,
		Domains: domains,
	}

	// Create SMTP credential
	credential, err := client.CreateSMTPCredential(req)
	if err != nil {
		return err
	}

	if credential == nil {
		return errors.NewAPIError("received nil response from API", nil)
	}

	// Use the new ResponseHandler to display created SMTP credential
	return handler.HandleCreateSMTP(credential, printer.CreateConfig{
		SuccessMessage: "SMTP credential created successfully",
		ItemName:       "smtp_credential",
		FieldOrder:     []string{"id", "name", "username", "password", "scope", "domains", "sandbox", "created_at", "updated_at"},
	})
}

func promptSMTPCredentialDetails() (name, scope string, domains []string, sandbox bool, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Create SMTP Credential")
	fmt.Println("======================")

	// Prompt for name
	fmt.Print("Credential name: ")
	name, err = reader.ReadString('\n')
	if err != nil {
		return "", "", nil, false, fmt.Errorf("failed to read name: %w", err)
	}
	name = strings.TrimSpace(name)

	// Prompt for scope
	fmt.Print("Scope (global/scoped) [global]: ")
	scope, err = reader.ReadString('\n')
	if err != nil {
		return "", "", nil, false, fmt.Errorf("failed to read scope: %w", err)
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
			return "", "", nil, false, fmt.Errorf("failed to read domains: %w", err)
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
		return "", "", nil, false, fmt.Errorf("failed to read sandbox mode: %w", err)
	}
	sandboxStr = strings.ToLower(strings.TrimSpace(sandboxStr))
	sandbox = sandboxStr == "y" || sandboxStr == "yes"

	// Password will be auto-generated
	fmt.Println("\nA secure password will be generated automatically.")

	return name, scope, domains, sandbox, nil
}

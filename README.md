# AhaSend CLI

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-00ADD8.svg)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/AhaSend/ahasend-go)](https://goreportcard.com/report/github.com/AhaSend/ahasend-go)
[![API Documentation](https://img.shields.io/badge/docs-api-green.svg)](https://ahasend.com/docs/api-reference)
[![License: Apache 2.0](https://img.shields.io/github/license/ahasend/ahasend-cli)](https://opensource.org/licenses/apache-2.0)

A powerful command-line interface for [AhaSend](https://ahasend.com), the reliable transactional email service. Send emails, manage domains, configure webhooks, and monitor email analytics directly from your terminal.

## Features

- **🚀 Email Sending**: Send single or batch emails with templates, attachments, and scheduling
- **🌐 Domain Management**: Add, verify, and manage sending domains with DNS configuration
- **🔔 Webhook Management**: Configure, test, and monitor real-time event notifications
- **🔐 Authentication**: Secure profile-based authentication with API key management
- **📊 Analytics**: Comprehensive email statistics and reporting
- **📦 Batch Processing**: High-performance concurrent operations with progress tracking
- **🎨 Multiple Output Formats**: JSON, table, CSV, and plain text
- **🐛 Debug Mode**: Detailed request/response logging for troubleshooting

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/AhaSend/ahasend-cli.git
cd ahasend-cli

# Build the CLI
make build

# Install to your PATH
sudo cp ./bin/ahasend /usr/local/bin/

# Verify installation
ahasend --help
```

### Binary Installation

Download the latest binary for your platform from the [releases page](https://github.com/AhaSend/ahasend-cli/releases).

```bash
# Linux/macOS
chmod +x ahasend
sudo mv ahasend /usr/local/bin/

# Verify installation
ahasend --version
```

## Quick Start

### 1. Authentication

```bash
# Interactive login
ahasend auth login

# Or provide credentials directly
ahasend auth login --api-key YOUR_API_KEY --account-id YOUR_ACCOUNT_ID
```

### 2. Add a Domain

```bash
# Add a domain for email sending
ahasend domains create example.com

# Verify the domain after DNS configuration
ahasend domains verify example.com
```

### 3. Send an Email

```bash
# Simple text email
ahasend messages send \
  --from noreply@example.com \
  --to user@recipient.com \
  --subject "Hello from AhaSend" \
  --text "Welcome to AhaSend!"

# HTML email with template
ahasend messages send \
  --from noreply@example.com \
  --to user@recipient.com \
  --subject "Welcome {{name}}" \
  --html-template welcome.html \
  --global-substitutions data.json
```

#### Email testing with Sandbox

You can include `--sandbox` to send the email in [sandbox mode](https://ahasend.com/docs/send-api/sandbox).  Sandbox mode simulate the entire email sending process without actually delivering emails to recipients. It’s the perfect solution for testing your API integration safely, testing webhook workflows, and developing email features without worrying about costs or accidental sends.

```bash
# Simple text email
ahasend messages send \
  --from noreply@example.com \
  --to user@recipient.com \
  --subject "Hello from AhaSend" \
  --text "Welcome to AhaSend!" \
  --sandbox
```

### 4. Send Batch Emails

```bash
# High-performance batch sending
ahasend messages send \
  --from noreply@example.com \
  --recipients users.csv \
  --subject "Welcome to AhaSend" \
  --html-template welcome.html \
  --max-concurrency 5 \
  --progress \
  --show-metrics
```

### 5. Test Webhooks & Inbound Routes

```bash
# Listen for outbound email webhook events in real-time
ahasend webhooks listen http://localhost:8080/webhook \
  --events "on_delivered,on_opened,on_clicked"

# Trigger test webhook events for development
ahasend webhooks trigger abcd1234-5678-90ef-abcd-1234567890ab \
  --events "on_delivered,on_opened"

# Listen for inbound email route events
ahasend routes listen --recipient "*@example.com" \
  --forward-to http://localhost:3000/webhook

# Trigger route events for testing (development only)
ahasend routes trigger route-id-here
```

## Command Reference

### Core Commands

| Command | Description |
|---------|-------------|
| `auth` | Manage authentication and profiles |
| `domains` | Manage sending domains |
| `messages` | Send and manage email messages |
| `webhooks` | Configure webhook endpoints |
| `suppressions` | Manage suppression lists |
| `stats` | View email statistics |
| `apikeys` | Manage API keys |
| `smtp` | SMTP credentials and testing |
| `routes` | Email routing rules |
| `ping` | Test API connectivity |

### Global Flags

```bash
--api-key        # Override API key for this command
--account-id     # Override Account ID
--profile        # Use specific profile
--output         # Output format (json, table, csv, plain)
--no-color       # Disable colored output
--verbose        # Enable verbose logging
--debug          # Enable debug logging with HTTP details
--help           # Show help for any command
```

## Examples

### Sending Emails with Templates

```bash
# Create a template file
cat > welcome.html << EOF
<!DOCTYPE html>
<html>
<body>
    <h1>Welcome {{first_name}}!</h1>
    <p>Thank you for joining {{company_name}}.</p>
</body>
</html>
EOF

# Send with substitutions
ahasend messages send \
  --from noreply@company.com \
  --to user@example.com \
  --subject "Welcome to {{company_name}}" \
  --html-template welcome.html \
  --global-substitutions '{"first_name": "John", "company_name": "ACME Corp"}'
```

### Batch Processing with CSV

```csv
# recipients.csv
email,name,first_name,account_type
john@example.com,John Doe,John,premium
jane@example.com,Jane Smith,Jane,basic
```

```bash
ahasend messages send \
  --from noreply@example.com \
  --recipients recipients.csv \
  --subject "Account Update for {{first_name}}" \
  --html-template notification.html \
  --max-concurrency 5 \
  --progress
```

### Managing Multiple Environments

```bash
# Set up profiles for different environments
ahasend auth login --profile production
ahasend auth login --profile staging

# Use specific profile for commands
ahasend messages send --profile staging \
  --from test@staging.com \
  --to dev@example.com \
  --subject "Test" \
  --text "Testing staging environment"

# Switch default profile
ahasend auth switch production
```

### Webhook Development and Testing

```bash
# Configure a webhook endpoint
ahasend webhooks create \
  --url https://api.example.com/webhooks/ahasend \
  --events "on_delivered,on_bounced,on_failed" \
  --description "Production webhook handler"

# Test webhook locally with real-time monitoring
ahasend webhooks listen http://localhost:3000/webhook \
  --events "all" \
  --verbose

# Trigger test events for integration testing
ahasend webhooks trigger webhook-id-here \
  --all-events

# List all configured webhooks
ahasend webhooks list --output table
```

### Inbound Email Route Testing

```bash
# Listen for inbound emails with temporary route
ahasend routes listen --recipient "*@example.com" \
  --forward-to http://localhost:3000/webhook

# Listen with existing route and slim output
ahasend routes listen --route-id abc123 \
  --slim-output

# Test route processing without real emails (dev only)
ahasend routes trigger route-id-here

# Create and test a support email route
ahasend routes create \
  --match-recipient "support@example.com" \
  --forward-to "team@company.com"

ahasend routes listen --recipient "support@example.com" \
  --forward-to http://localhost:8080/support-webhook
```

### Monitoring and Analytics

```bash
# View delivery statistics
ahasend stats deliverability \
  --start-date 2024-01-01 \
  --end-date 2024-01-31 \
  --group-by day

# Check bounce rates
ahasend stats bounces --group-by day

# Export stats to CSV
ahasend stats deliverability --output csv > stats.csv
```

## Configuration

Configuration is stored in `~/.ahasend/config.yaml`:

```yaml
default_profile: production
profiles:
  production:
    api_key: "your-api-key"
    account_id: "your-account-id"
    api_url: "https://api.ahasend.com"
preferences:
  output_format: table
  color_output: true
  batch_concurrency: 5
```

## Output Formats

The CLI supports multiple output formats:

- **Table** (default): Human-readable formatted tables
- **JSON**: Machine-readable for automation
- **CSV**: For data export and analysis
- **Plain**: Simple key-value format

```bash
# Examples
ahasend domains list              # Table format
ahasend domains list --output json # JSON format
ahasend stats bounces --output csv # CSV format
```

## Development

### Prerequisites

- Go 1.21 or higher
- Make

### Building from Source

```bash
# Clone the repository
git clone https://github.com/AhaSend/ahasend-cli.git
cd ahasend-cli

# Install dependencies
go mod download

# Run tests
make test

# Build binary
make build

# Run linter
make lint
```

### Project Structure

```
ahasend-cli/
├── cmd/                    # Command implementations
│   ├── root.go            # Root command
│   └── groups/            # Command groups
│       ├── auth/          # Authentication commands
│       ├── domains/       # Domain management
│       ├── messages/      # Message sending
│       └── ...
├── internal/              # Internal packages
│   ├── client/           # API client wrapper
│   ├── config/           # Configuration management
│   ├── printer/          # Output formatting
│   └── ...
├── docs/                  # Documentation
│   └── openapi.yaml      # API specification
├── test/                  # Integration tests
└── Makefile              # Build automation
```

### Running Tests

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests
make test-integration

# Run with coverage
make test-coverage
```

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## Documentation

- [Full CLI Documentation](USAGE.mdx)
- [AhaSend API Documentation](https://ahasend.com/docs/api-reference)
- [AhaSend Website](https://ahasend.com)

## Support

- **Issues**: [GitHub Issues](https://github.com/AhaSend/ahasend-cli/issues)
- **Email**: support@ahasend.com
- **Documentation**: [ahasend.com/docs](https://ahasend.com/docs)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [AhaSend Go SDK](https://github.com/AhaSend/ahasend-go) - API client

---

Made with ❤️ by the AhaSend team
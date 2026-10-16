# AhaSend CLI

The command-line interface for [AhaSend](https://ahasend.com), the
transactional email service. Send email, and manage domains, webhooks,
routes, suppressions, SMTP credentials, API keys, sub-accounts, contacts,
lists, templates and campaigns from your terminal, with table, JSON, CSV or
plain text output.

## Install

Needs Node.js 18 or later.

```bash
npm install -g @ahasend/cli
```

Or run it once without installing:

```bash
npx @ahasend/cli --version
```

pnpm and Bun work the same way (`pnpm add -g @ahasend/cli`,
`bun add -g @ahasend/cli`). The launcher runs with Node.js, so Bun also needs
Node.js 18 or later.

To upgrade, run `npm install -g @ahasend/cli@latest`. To uninstall, run
`npm uninstall -g @ahasend/cli`.

## Quick start

```bash
# Log in with an API key from https://dash.ahasend.com
ahasend auth login

# Check the key and the connection
ahasend ping

# Send a test email in sandbox mode, from a domain you verified
ahasend messages send \
  --from you@yourdomain.com \
  --to recipient@example.com \
  --subject "Hello from the AhaSend CLI" \
  --text "It works." \
  --sandbox

# See every command
ahasend --help
```

## How the package works

`@ahasend/cli` holds a small Node.js launcher. The `ahasend` binary for your
system comes in one of six platform packages, which npm installs as an
optional dependency:

| Package | System |
|---|---|
| `@ahasend/cli-darwin-arm64` | macOS, Apple Silicon |
| `@ahasend/cli-darwin-x64` | macOS, Intel |
| `@ahasend/cli-linux-arm64` | Linux, ARM64 |
| `@ahasend/cli-linux-x64` | Linux, x86_64 |
| `@ahasend/cli-win32-arm64` | Windows, ARM64 |
| `@ahasend/cli-win32-x64` | Windows, x64 |

If you install with optional dependencies turned off (for example
`--omit=optional`), `ahasend` stops with a message that says so. The
package has no install scripts.

The launcher passes the binary's exit code through. Two signal cases differ
from installing the binary another way:

- A `SIGINT` sent to the launcher process alone, by a tool that signals one
  process, does not reach the CLI. Ctrl+C in a terminal works as usual.
- A tool that sends `SIGTERM` to the whole process group, such as `timeout`,
  makes the CLI receive it twice, so `messages send` and `messages cancel`
  of several messages skip their clean stop.

If either case matters to you, for example under a process supervisor,
install the binary another way.

## Other ways to install

Homebrew, Scoop, install scripts for macOS, Linux and Windows, .deb and
.rpm packages, and plain binaries: see the
[installation guide](https://ahasend.com/docs/cli/installation).

## Documentation

- [CLI documentation](https://ahasend.com/docs/cli)
- [Command reference](https://github.com/AhaSend/ahasend-cli/blob/main/USAGE.mdx)
- [Changelog](https://github.com/AhaSend/ahasend-cli/blob/main/CHANGELOG.md)
- [Report a problem](https://github.com/AhaSend/ahasend-cli/issues)

## License

Apache-2.0. The `THIRD_PARTY_NOTICES` file in each package lists the
licences of the code built into the binary.

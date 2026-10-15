package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// exampleCommands returns the ahasend command lines of an Example text, a
// line ending in a backslash joined with the next, split into words as a
// shell would, each command of a pipeline or a list on its own, such as the
// ahasend command after the pipe of printf ... | ahasend smtp send.
func exampleCommands(example string) [][]string {
	var commands [][]string
	var line strings.Builder
	for _, raw := range strings.Split(example, "\n") {
		trimmed := strings.TrimSpace(raw)
		if continued, ok := strings.CutSuffix(trimmed, `\`); ok {
			line.WriteString(continued + " ")
			continue
		}
		line.WriteString(trimmed)
		for _, words := range shellCommands(line.String()) {
			if words[0] == "ahasend" {
				commands = append(commands, words[1:])
			}
		}
		line.Reset()
	}
	return commands
}

// shellCommands splits line into the words of each command it runs, as a
// shell would: it honors single and double quotes and backslash escapes,
// starts a new command after each unquoted |, ; or &, and ends the words of
// a command at its first unquoted > or <, a redirection.
func shellCommands(line string) [][]string {
	var commands [][]string
	var words []string
	var word strings.Builder
	inWord, redirected, escaped := false, false, false
	var quote rune
	endWord := func() {
		if inWord && !redirected {
			words = append(words, word.String())
		}
		word.Reset()
		inWord = false
	}
	endCommand := func() {
		endWord()
		if len(words) > 0 {
			commands = append(commands, words)
		}
		words, redirected = nil, false
	}
	for _, r := range line {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped = false
		case quote != 0:
			if r == quote {
				quote = 0
			} else if r == '\\' && quote == '"' {
				escaped = true
			} else {
				word.WriteRune(r)
			}
		case r == '\\':
			escaped, inWord = true, true
		case r == '"' || r == '\'':
			quote, inWord = r, true
		case strings.ContainsRune("|;&", r):
			endCommand()
		case strings.ContainsRune("><", r):
			endWord()
			redirected = true
		case r == ' ' || r == '\t':
			endWord()
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	endCommand()
	return commands
}

// TestEveryExampleRunsACommandWithItsFlags checks that every ahasend command
// line in the examples of every command names a command that exists, with
// flags that command has and that are not deprecated.
func TestEveryExampleRunsACommandWithItsFlags(t *testing.T) {
	root := NewRootCmdForTesting()
	walkCommands(root, func(cmd *cobra.Command) {
		for _, words := range exampleCommands(cmd.Example) {
			example := "ahasend " + strings.Join(words, " ")
			target, args, err := root.Find(words)
			if !assert.NoError(t, err, "%s: %s", cmd.CommandPath(), example) {
				continue
			}
			for _, arg := range args {
				if arg == "--" {
					break
				}
				if !strings.HasPrefix(arg, "-") || arg == "-" {
					continue
				}
				name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
				flag := target.Flag(name)
				if !strings.HasPrefix(arg, "--") {
					flag = target.Flags().ShorthandLookup(name[:1])
				}
				if assert.NotNil(t, flag, "%s: %s uses %s, which %s does not have", cmd.CommandPath(), example, arg, target.CommandPath()) {
					assert.Empty(t, flag.Deprecated, "%s: %s uses the deprecated %s", cmd.CommandPath(), example, arg)
				}
			}
		}
	})
}

func TestShellCommandsSplitsLikeAShell(t *testing.T) {
	assert.Equal(t, [][]string{{"ahasend", "send", "--subject", "Hi {{name}}", "--html", "<h1>x</h1>"}, {"jq", "."}},
		shellCommands(`ahasend send --subject "Hi {{name}}" --html '<h1>x</h1>' | jq . > out.json`))
	assert.Equal(t, [][]string{{"printf", "%s\\n", "$SMTP_PASSWORD"}, {"ahasend", "smtp", "send", "--password-stdin"}},
		shellCommands(`printf '%s\n' "$SMTP_PASSWORD" | ahasend smtp send --password-stdin`))
	assert.Equal(t, [][]string{{"routes", "listen", "--recipient", "a@b.c", "--forward-to", "http://x"}},
		exampleCommands("  # caption\n  ahasend routes listen --recipient \"a@b.c\" \\\n    --forward-to http://x"))
	assert.Equal(t, [][]string{{"domains", "delete", "example.com"}},
		exampleCommands(`  echo "yes" | ahasend domains delete example.com`))
}

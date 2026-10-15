package cmdutil

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/spf13/cobra"
)

// ReadOrAsk returns the value of the required text flag name. A blank
// value is the validation error "--name cannot be empty". When the flag is
// missing and prompt.CanPrompt allows it, it asks for the value with label; a
// missing value, or a blank answer, is the validation error
// "--name is required".
func ReadOrAsk(cmd *cobra.Command, name, label string) (string, error) {
	return readOrAsk(cmd, name, label, prompt.Prompt, isBlank)
}

// ReadSecretOrAsk is ReadOrAsk for a secret, such as a password, which
// is taken as given, so only an empty value fails: it asks with
// prompt.PromptSecret, so a terminal hides the answer as it is typed.
func ReadSecretOrAsk(cmd *cobra.Command, name, label string) (string, error) {
	return readOrAsk(cmd, name, label, prompt.PromptSecret, func(value string) bool { return value == "" })
}

// readOrAsk is ReadOrAsk, with empty deciding which values count as
// empty.
func readOrAsk(cmd *cobra.Command, name, label string, ask promptFunc, empty func(string) bool) (string, error) {
	if value := ChangedString(cmd, name); value != nil {
		if empty(*value) {
			return "", emptyValueError(cmd, name, "")
		}
		return *value, nil
	}
	missing := fmt.Sprintf("--%s is required", name)
	value, err := askIfEmpty(cmd, "", label, missing, ask)
	if err != nil {
		return "", err
	}
	if empty(value) {
		return "", errors.NewValidationError(missing, nil)
	}
	return value, nil
}

// promptFunc asks for an answer with message: prompt.Prompt or
// prompt.PromptSecret.
type promptFunc func(cmd *cobra.Command, message string) (string, error)

// AskIfEmpty returns value, or, when it is empty and prompt.CanPrompt allows it, the
// answer to a prompt with label. A value that is still empty is the
// validation error missing.
func AskIfEmpty(cmd *cobra.Command, value, label, missing string) (string, error) {
	return askIfEmpty(cmd, value, label, missing, prompt.Prompt)
}

func askIfEmpty(cmd *cobra.Command, value, label, missing string, ask promptFunc) (string, error) {
	if value == "" && prompt.CanPrompt(cmd) {
		answer, err := ask(cmd, label+": ")
		if err != nil {
			return "", err
		}
		value = answer
	}
	if value == "" {
		return "", errors.NewValidationError(missing, nil)
	}
	return value, nil
}

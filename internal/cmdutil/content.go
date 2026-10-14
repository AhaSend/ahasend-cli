package cmdutil

import (
	"bytes"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// editors are the editors of a template's design, the values of --editor, as
// the API defines them.
var editors = []string{"advanced", "simple", "html"}

// AddCreateContentFlags adds the content flags of a create command:
// --mjml-file, --html-file, --text-file and --editor.
func AddCreateContentFlags(cmd *cobra.Command) {
	addContentFileFlags(cmd)
	AddOneOfFlag(cmd, "editor", "", "Editor of the design: %s", editors)
}

// AddUpdateContentFlags adds the content flags of an update command:
// --mjml-file, --html-file, --text-file and --regenerate-text.
func AddUpdateContentFlags(cmd *cobra.Command) {
	addContentFileFlags(cmd)
	cmd.Flags().Bool("regenerate-text", false, "Make the text version again from the HTML")
}

func addContentFileFlags(cmd *cobra.Command) {
	cmd.Flags().String("mjml-file", "", "Path to the MJML source")
	cmd.Flags().String("html-file", "", "Path to the HTML")
	cmd.Flags().String("text-file", "", "Path to the plain text version")
	for _, name := range []string{"mjml-file", "html-file", "text-file"} {
		MarkFileFlag(cmd, name)
	}
}

// ReadContent builds the content from the content flags, or returns nil when
// none is set.
func ReadContent(cmd *cobra.Command) (*requests.TemplateContentInput, error) {
	if err := RejectTogether(cmd, "mjml-file", "html-file"); err != nil {
		return nil, err
	}
	if err := RejectTogether(cmd, "text-file", "regenerate-text"); err != nil {
		return nil, err
	}
	regenerateText, _ := cmd.Flags().GetBool("regenerate-text")

	content := requests.TemplateContentInput{}
	var err error
	if content.MJML, err = readContentFile(cmd, "mjml-file"); err != nil {
		return nil, err
	}
	if content.HTML, err = readContentFile(cmd, "html-file"); err != nil {
		return nil, err
	}
	if content.Text, err = readContentFile(cmd, "text-file"); err != nil {
		return nil, err
	}
	// The SDK sends an empty text as null, which would make the text from the
	// HTML instead of setting it.
	if content.Text != nil && *content.Text == "" {
		return nil, errors.NewValidationError("--text-file is empty", nil)
	}
	if regenerateText {
		empty := ""
		content.Text = &empty
	}

	if content.MJML == nil && content.HTML == nil && content.Text == nil {
		return nil, nil
	}
	return &content, nil
}

// ReadTemplateEditor returns --editor of a template create, or nil when it is
// not set. The API needs an editor when the content holds neither MJML nor
// HTML.
func ReadTemplateEditor(cmd *cobra.Command, content *requests.TemplateContentInput) (*string, error) {
	editor, err := ReadOneOf(cmd, "editor")
	if err != nil {
		return nil, err
	}
	if editor == nil && (content == nil || (content.MJML == nil && content.HTML == nil)) {
		return nil, errors.NewValidationError("--editor is needed when there is neither --mjml-file nor --html-file", nil)
	}
	return editor, nil
}

func readContentFile(cmd *cobra.Command, flag string) (*string, error) {
	path := ChangedString(cmd, flag)
	if path == nil {
		return nil, nil
	}
	value, err := ReadFlagFile(flag, *path)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

// ReadFlagFile returns the text of the file at path, which the flag named
// flag gives, as ReadInputFile reads it.
func ReadFlagFile(flag, path string) (string, error) {
	data, err := ReadInputFile(flag, path)
	return string(data), err
}

// utf8BOM is the byte order mark some programs, such as Excel saving
// "CSV UTF-8", put at the start of a UTF-8 file.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// The byte order marks of UTF-16, little and big endian, which Windows
// programs such as Notepad save "Unicode" text with.
var (
	utf16LEBOM = []byte{0xFF, 0xFE}
	utf16BEBOM = []byte{0xFE, 0xFF}
)

// ReadInputFile returns the content of the file at path, which the flag named
// flag gives, without a leading UTF-8 byte order mark; every text or data
// file a user gives is read with it. A missing or unreadable file, and a file
// that is not UTF-8 text, are validation errors: the API takes UTF-8, and
// sending other bytes would replace each character that is not UTF-8.
func ReadInputFile(flag, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.NewValidationError(fmt.Sprintf("failed to read --%s", flag), err)
	}
	if bytes.HasPrefix(data, utf16LEBOM) || bytes.HasPrefix(data, utf16BEBOM) {
		return nil, errors.NewValidationError(fmt.Sprintf("%s is UTF-16 text, not UTF-8; save it as UTF-8", path), nil)
	}
	data = bytes.TrimPrefix(data, utf8BOM)
	if !utf8.Valid(data) {
		return nil, errors.NewValidationError(fmt.Sprintf("%s is not UTF-8 text; save it as UTF-8", path), nil)
	}
	return data, nil
}

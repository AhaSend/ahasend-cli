package cmdutil

import (
	"bytes"
	"fmt"
	"os"
	"unicode/utf8"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// editors are the editors of a template's or a campaign's design, the values
// of --editor, as the API defines them.
var editors = []string{"advanced", "simple", "html"}

// AddCreateContentFlags adds the content flags of a create command:
// --markup-file, --mjml-file, --html-file, --text-file and --editor.
func AddCreateContentFlags(cmd *cobra.Command) {
	addContentFileFlags(cmd)
	AddOneOfFlag(cmd, "editor", "", "Editor of the design: %s", editors)
}

// AddUpdateContentFlags adds the content flags of an update command:
// --markup-file, --mjml-file, --html-file, --text-file and --regenerate-text.
func AddUpdateContentFlags(cmd *cobra.Command) {
	addContentFileFlags(cmd)
	cmd.Flags().Bool("regenerate-text", false, "Make the text version again from the HTML")
}

func addContentFileFlags(cmd *cobra.Command) {
	cmd.Flags().String("markup-file", "", "Path to the AhaML of a Simple Editor design")
	cmd.Flags().String("mjml-file", "", "Path to the MJML source")
	cmd.Flags().String("html-file", "", "Path to the HTML")
	cmd.Flags().String("text-file", "", "Path to the plain text version")
	for _, name := range []string{"markup-file", "mjml-file", "html-file", "text-file"} {
		MarkFileFlag(cmd, name)
	}
}

// ReadContent builds the content from the content flags, or returns nil when
// none is set. --markup-file goes with neither --mjml-file, as the API
// refuses MJML beside markup, nor --html-file, which the API ignores beside
// it.
func ReadContent(cmd *cobra.Command) (*requests.TemplateContentInput, error) {
	for _, pair := range [][2]string{
		{"markup-file", "mjml-file"}, {"markup-file", "html-file"}, {"mjml-file", "html-file"},
		{"text-file", "regenerate-text"},
	} {
		if err := RejectTogether(cmd, pair[0], pair[1]); err != nil {
			return nil, err
		}
	}
	regenerateText, _ := cmd.Flags().GetBool("regenerate-text")

	content := requests.TemplateContentInput{}
	var err error
	if content.Markup, err = readContentFile(cmd, "markup-file"); err != nil {
		return nil, err
	}
	// The API refuses an empty markup, which is not AhaML.
	if content.Markup != nil && *content.Markup == "" {
		return nil, errors.NewValidationError("--markup-file is empty", nil)
	}
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

	if content.Markup == nil && content.MJML == nil && content.HTML == nil && content.Text == nil {
		return nil, nil
	}
	return &content, nil
}

// ReadTemplateEditor returns the editor of a template create, as
// MarkupEditor gives it from --editor, or nil when it is not set. The API
// needs an editor when the content holds none of markup, MJML and HTML.
func ReadTemplateEditor(cmd *cobra.Command, content *requests.TemplateContentInput) (*string, error) {
	editor, err := ReadOneOf(cmd, "editor")
	if err != nil {
		return nil, err
	}
	if editor, err = MarkupEditor(content, editor); err != nil {
		return nil, err
	}
	if editor == nil && (content == nil || (content.MJML == nil && content.HTML == nil)) {
		return nil, errors.NewValidationError("--editor is needed when there is no --markup-file, --mjml-file or --html-file", nil)
	}
	return editor, nil
}

// MarkupEditor returns the editor a create sends for content, given editor,
// the value of --editor or nil. Markup is the design of the Simple Editor,
// so content with markup has the editor simple when --editor is not given,
// and any other --editor is a validation error. Content without markup
// keeps editor as it is.
func MarkupEditor(content *requests.TemplateContentInput, editor *string) (*string, error) {
	if content == nil || content.Markup == nil {
		return editor, nil
	}
	if editor == nil {
		simple := common.TemplateEditorSimple
		return &simple, nil
	}
	if *editor != common.TemplateEditorSimple {
		return nil, errors.NewValidationError(fmt.Sprintf(
			"--editor %s cannot be used with --markup-file, which holds a Simple Editor design", *editor), nil)
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

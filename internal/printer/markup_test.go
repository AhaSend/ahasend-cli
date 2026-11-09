package printer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixtureMarkup is the AhaML of a Simple Editor design.
const fixtureMarkup = `<Email version="1">
  <Heading level="2">Welcome</Heading>
  <Paragraph>Hi <Var name="first_name" fallback="there"/>, {{ plan }}</Paragraph>
</Email>`

// markupContentFixture returns the content of a Simple Editor design with
// its markup.
func markupContentFixture() *responses.TemplateContent {
	markup := fixtureMarkup
	return &responses.TemplateContent{Markup: &markup, HTML: "<h2>Welcome</h2>", Text: "Welcome"}
}

// markupErrorContentFixture returns the content of a Simple Editor design
// whose markup the API could not make.
func markupErrorContentFixture() *responses.TemplateContent {
	return &responses.TemplateContent{
		HTML: "<div>Custom</div>", Text: "Custom",
		MarkupError: &responses.MarkupError{Code: responses.MarkupErrorHTMLBlock, Message: "the design holds an HTML block"},
	}
}

// markupWarningsFixture returns the two warnings of a write that carried
// markup.
func markupWarningsFixture() []common.MarkupProblem {
	return []common.MarkupProblem{
		{Code: common.MarkupWarningJinjaInText, Message: "text holds {{, which stays text", Line: 3, Column: 48,
			Path: "Email > Paragraph[1]"},
		{Code: common.MarkupWarningJinjaInText, Message: "text holds {{, which stays text"},
	}
}

// simpleTemplateFixture returns a template of the Simple Editor as a write
// that carried its markup returns it, with warnings and a variable of each
// kind.
func simpleTemplateFixture() responses.Template {
	template := templateFixture()
	template.Editor = common.TemplateEditorSimple
	template.Variables = []responses.TemplateVariable{
		{Name: "first_name", Kind: responses.TemplateVariableKindValue},
		{Name: "vip", Kind: responses.TemplateVariableKindCondition},
		{Name: "items", Required: true, Kind: responses.TemplateVariableKindList, ItemFields: []string{"name", "price"}},
	}
	template.Content = markupContentFixture()
	template.MarkupWarnings = markupWarningsFixture()
	return template
}

// simpleCampaignFixture returns a campaign of the Simple Editor as a write
// that carried its markup returns it.
func simpleCampaignFixture() responses.Campaign {
	campaign := campaignFixture()
	editor := common.TemplateEditorSimple
	campaign.Editor = &editor
	campaign.Content = markupContentFixture()
	campaign.MarkupWarnings = markupWarningsFixture()
	return campaign
}

// simpleCampaignTemplateFixture returns a campaign template of the Simple
// Editor as a write that carried its markup returns it.
func simpleCampaignTemplateFixture() responses.CampaignTemplate {
	template := campaignTemplateFixture()
	template.Editor = common.TemplateEditorSimple
	template.Content = markupContentFixture()
	template.MarkupWarnings = markupWarningsFixture()
	return template
}

// addMarkupRenders adds the views of every content view with markup and with
// a markup error.
func addMarkupRenders(renders map[string]func(ResponseHandler) error) {
	template, campaign, campaignTemplate := simpleTemplateFixture(), simpleCampaignFixture(), simpleCampaignTemplateFixture()
	version := templateVersionDetailFixture()
	version.Variables = template.Variables
	version.Content = markupContentFixture()

	failedTemplate, failedCampaign, failedCampaignTemplate := templateFixture(), campaignFixture(), campaignTemplateFixture()
	failedTemplate.Editor, failedCampaignTemplate.Editor = common.TemplateEditorSimple, common.TemplateEditorSimple
	editor := common.TemplateEditorSimple
	failedCampaign.Editor = &editor
	failedVersion := templateVersionDetailFixture()
	failedTemplate.Content, failedCampaign.Content = markupErrorContentFixture(), markupErrorContentFixture()
	failedCampaignTemplate.Content, failedVersion.Content = markupErrorContentFixture(), markupErrorContentFixture()

	add(renders, map[string]func(ResponseHandler) error{
		"template with markup": func(h ResponseHandler) error { return h.HandleSingleTemplate(&template, SingleConfig{}) },
		"template with a markup error": func(h ResponseHandler) error {
			return h.HandleSingleTemplate(&failedTemplate, SingleConfig{})
		},
		"template version with markup": func(h ResponseHandler) error {
			return h.HandleSingleTemplateVersion(&version, SingleConfig{})
		},
		"template version with a markup error": func(h ResponseHandler) error {
			return h.HandleSingleTemplateVersion(&failedVersion, SingleConfig{})
		},
		"campaign with markup": func(h ResponseHandler) error { return h.HandleSingleCampaign(&campaign, SingleConfig{}) },
		"campaign with a markup error": func(h ResponseHandler) error {
			return h.HandleSingleCampaign(&failedCampaign, SingleConfig{})
		},
		"campaign template with markup": func(h ResponseHandler) error {
			return h.HandleSingleCampaignTemplate(&campaignTemplate, SingleConfig{})
		},
		"campaign template with a markup error": func(h ResponseHandler) error {
			return h.HandleSingleCampaignTemplate(&failedCampaignTemplate, SingleConfig{})
		},
	})
}

func TestMarkupProblemLeavesOutWhatItDoesNotHave(t *testing.T) {
	tests := map[string]struct {
		problem common.MarkupProblem
		want    string
	}{
		"every field": {
			common.MarkupProblem{Code: "unknown_attribute", Message: `Paragraph has no attribute "color"`, Line: 4, Column: 14,
				Path: "Email > Section[2] > Paragraph[1]", Hint: `wrap the text in <Color value="..."> instead`},
			`line 4, column 14: Paragraph has no attribute "color" (unknown_attribute) at Email > Section[2] > Paragraph[1]; ` +
				`hint: wrap the text in <Color value="..."> instead`,
		},
		"no line": {
			common.MarkupProblem{Code: "too_large", Message: "the markup is larger than 512 KiB"},
			"the markup is larger than 512 KiB (too_large)",
		},
		"no path": {
			common.MarkupProblem{Code: "syntax", Message: "unclosed tag", Line: 2, Column: 3},
			"line 2, column 3: unclosed tag (syntax)",
		},
		"no hint": {
			common.MarkupProblem{Code: "jinja_in_text", Message: "text holds {{", Line: 1, Column: 5, Path: "Email > Paragraph[1]"},
			"line 1, column 5: text holds {{ (jinja_in_text) at Email > Paragraph[1]",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.want, formatMarkupProblem(test.problem))
		})
	}
}

// refusedMarkupError returns the error of a write whose AhaML the API
// refused with three problems: one with a line, a column and a path, one
// with no line, and one with a hint.
func refusedMarkupError() *api.APIError {
	body := `{"message":"content.markup has 3 problems","errors":[` +
		`{"code":"unknown_tag","message":"Box is not a tag","line":2,"column":3,"path":"Email > Box[1]"},` +
		`{"code":"too_large","message":"the markup is too large"},` +
		`{"code":"unknown_attribute","message":"Paragraph has no attribute \"color\"","line":4,"column":14,` +
		`"hint":"wrap the text in <Color> instead"}]}`
	return &api.APIError{
		Type: api.ErrorTypeValidation, StatusCode: 400, Message: "content.markup has 3 problems", Raw: []byte(body),
		Errors: []common.MarkupProblem{
			{Code: "unknown_tag", Message: "Box is not a tag", Line: 2, Column: 3, Path: "Email > Box[1]"},
			{Code: "too_large", Message: "the markup is too large"},
			{Code: "unknown_attribute", Message: `Paragraph has no attribute "color"`, Line: 4, Column: 14,
				Hint: "wrap the text in <Color> instead"},
		},
	}
}

func TestHandleErrorListsTheMarkupProblemsOutsideJSON(t *testing.T) {
	for _, format := range []string{"table", "plain", "csv"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			require.Error(t, testHandler(t, format, &stdout, &stderr).HandleError(refusedMarkupError()))

			assert.Empty(t, stdout.String())
			assert.Equal(t, "Error: validation error (HTTP 400): content.markup has 3 problems\n"+
				"  line 2, column 3: Box is not a tag (unknown_tag) at Email > Box[1]\n"+
				"  the markup is too large (too_large)\n"+
				`  line 4, column 14: Paragraph has no attribute "color" (unknown_attribute); hint: wrap the text in <Color> instead`+"\n",
				stderr.String())
		})
	}
}

func TestJSONHandleErrorPrintsTheRefusedMarkupBodyUnchanged(t *testing.T) {
	var stdout, stderr bytes.Buffer
	apiErr := refusedMarkupError()

	require.Error(t, testHandler(t, "json", &stdout, &stderr).HandleError(apiErr))

	assert.JSONEq(t, string(apiErr.Raw), stdout.String())
	assert.Empty(t, stderr.String())
}

// TestMarkupProblemsWithALineBreakPrintOnOneLine checks that a problem of an
// error and a warning of a write, whose messages hold a line break and a
// control character, each print on one line, escaped.
func TestMarkupProblemsWithALineBreakPrintOnOneLine(t *testing.T) {
	problem := common.MarkupProblem{Code: "bad\ncode", Message: "first\nsecond\x1b[31m", Line: 1, Column: 2,
		Path: "Email\r> Paragraph[1]", Hint: "use\ta <Var>"}
	const want = `line 1, column 2: first\u000asecond\u001b[31m (bad\u000acode) at Email\u000d> Paragraph[1]; hint: use\u0009a <Var>`

	t.Run("error", func(t *testing.T) {
		var stderr bytes.Buffer
		_ = testHandler(t, "plain", &bytes.Buffer{}, &stderr).HandleError(&api.APIError{
			Type: api.ErrorTypeValidation, StatusCode: 400, Message: "content.markup has 1 problem",
			Errors: []common.MarkupProblem{problem},
		})
		assert.Equal(t, "Error: validation error (HTTP 400): content.markup has 1 problem\n  "+want+"\n", stderr.String())
	})
	t.Run("warning", func(t *testing.T) {
		template := simpleTemplateFixture()
		template.MarkupWarnings = []common.MarkupProblem{problem}
		render := func(h ResponseHandler) error { return h.HandleSingleTemplate(&template, SingleConfig{}) }

		assert.True(t, strings.HasSuffix(renderWith(t, "plain", render), "Markup Warnings:\n  "+want+"\n"))
		assert.Contains(t, tableCells(renderWith(t, "table", render)), []string{"Markup Warnings", want})
	})
}

// warningViews renders the single view of a template, a campaign and a
// campaign template, as a write that carried markup returns them, with
// warnings.
func warningViews(warnings []common.MarkupProblem) map[string]func(ResponseHandler) error {
	template, campaign, campaignTemplate := simpleTemplateFixture(), simpleCampaignFixture(), simpleCampaignTemplateFixture()
	template.MarkupWarnings, campaign.MarkupWarnings, campaignTemplate.MarkupWarnings = warnings, warnings, warnings
	return map[string]func(ResponseHandler) error{
		"template": func(h ResponseHandler) error { return h.HandleSingleTemplate(&template, SingleConfig{}) },
		"campaign": func(h ResponseHandler) error { return h.HandleSingleCampaign(&campaign, SingleConfig{}) },
		"campaign template": func(h ResponseHandler) error {
			return h.HandleSingleCampaignTemplate(&campaignTemplate, SingleConfig{})
		},
	}
}

func TestWriteAnswersShowTheirMarkupWarnings(t *testing.T) {
	const (
		first  = "line 3, column 48: text holds {{, which stays text (jinja_in_text) at Email > Paragraph[1]"
		second = "text holds {{, which stays text (jinja_in_text)"
	)
	for name, render := range warningViews(markupWarningsFixture()) {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, tableCells(renderWith(t, "table", render)), []string{"Markup Warnings", first})
			assert.Contains(t, tableCells(renderWith(t, "table", render)), []string{"", second})
			assert.True(t, strings.HasSuffix(renderWith(t, "plain", render), "Markup Warnings:\n  "+first+"\n  "+second+"\n"))
			header, rows := readCSV(t, renderWith(t, "csv", render))
			assert.Equal(t, "markup_warnings", header[len(header)-1])
			assert.Equal(t, `[{"code":"jinja_in_text","message":"text holds {{, which stays text","line":3,"column":48,`+
				`"path":"Email > Paragraph[1]"},{"code":"jinja_in_text","message":"text holds {{, which stays text"}]`,
				rows[0]["markup_warnings"])
		})
	}
}

func TestWriteAnswersWithoutMarkupWarningsShowNone(t *testing.T) {
	for _, warnings := range [][]common.MarkupProblem{nil, {}} {
		for name, render := range warningViews(warnings) {
			t.Run(name, func(t *testing.T) {
				assert.NotContains(t, renderWith(t, "table", render), "Markup Warnings")
				assert.NotContains(t, renderWith(t, "plain", render), "Markup Warnings")
				header, rows := readCSV(t, renderWith(t, "csv", render))
				assert.Equal(t, "markup_warnings", header[len(header)-1])
				assert.Equal(t, "", rows[0]["markup_warnings"])
			})
		}
	}
}

// TestViewsThatShareTheFieldsHaveNoMarkupWarnings checks that the CSV header
// of the views that share the fields of a template, a campaign template and a
// campaign, but are no write answer, holds no markup_warnings column.
func TestViewsThatShareTheFieldsHaveNoMarkupWarnings(t *testing.T) {
	send := campaignSendFixture()
	views := map[string]func(ResponseHandler) error{
		"templates list": func(h ResponseHandler) error { return h.HandleTemplateList(templatesPageFixture(), ListConfig{}) },
		"campaign templates list": func(h ResponseHandler) error {
			return h.HandleCampaignTemplateList(campaignTemplatesPageFixture(), ListConfig{})
		},
		"campaign send": func(h ResponseHandler) error { return h.HandleCampaignSend(send, SingleConfig{}) },
	}
	for name, render := range views {
		t.Run(name, func(t *testing.T) {
			header, _ := readCSV(t, renderWith(t, "csv", render))
			assert.Contains(t, header, "markup")
			assert.NotContains(t, header, "markup_warnings")
		})
	}
}

// TestContentViewsShowTheMarkupAndItsError checks that every view of content
// shows the markup, or why there is none, after the MJML, and that its CSV
// cells sit under their headers.
func TestContentViewsShowTheMarkupAndItsError(t *testing.T) {
	views := map[string]func(*responses.TemplateContent) func(ResponseHandler) error{
		"template": func(content *responses.TemplateContent) func(ResponseHandler) error {
			template := templateFixture()
			template.Content = content
			return func(h ResponseHandler) error { return h.HandleSingleTemplate(&template, SingleConfig{}) }
		},
		"template version": func(content *responses.TemplateContent) func(ResponseHandler) error {
			version := templateVersionDetailFixture()
			version.Content = content
			return func(h ResponseHandler) error { return h.HandleSingleTemplateVersion(&version, SingleConfig{}) }
		},
		"campaign": func(content *responses.TemplateContent) func(ResponseHandler) error {
			campaign := campaignFixture()
			campaign.Content = content
			return func(h ResponseHandler) error { return h.HandleSingleCampaign(&campaign, SingleConfig{}) }
		},
		"campaign template": func(content *responses.TemplateContent) func(ResponseHandler) error {
			template := campaignTemplateFixture()
			template.Content = content
			return func(h ResponseHandler) error { return h.HandleSingleCampaignTemplate(&template, SingleConfig{}) }
		},
	}
	for name, view := range views {
		t.Run(name, func(t *testing.T) {
			withMarkup, withError := view(markupContentFixture()), view(markupErrorContentFixture())

			assert.Contains(t, renderWith(t, "plain", withMarkup), "Markup:\n  "+strings.ReplaceAll(fixtureMarkup, "\n", "\n  ")+"\nHTML:\n")
			assert.Contains(t, renderWith(t, "plain", withError), "Markup Error:\n  html_block: the design holds an HTML block\nHTML:\n")
			assert.Contains(t, tableCells(renderWith(t, "table", withError)),
				[]string{"Markup Error", "html_block: the design holds an HTML block"})
			assert.NotContains(t, renderWith(t, "table", withMarkup), "Markup Error")

			_, rows := readCSV(t, renderWith(t, "csv", withMarkup))
			assert.Equal(t, fixtureMarkup, rows[0]["markup"])
			assert.Equal(t, "", rows[0]["markup_error"])
			assert.Equal(t, "<h2>Welcome</h2>", rows[0]["html"])
			_, rows = readCSV(t, renderWith(t, "csv", withError))
			assert.Equal(t, "", rows[0]["markup"])
			assert.Equal(t, "html_block: the design holds an HTML block", rows[0]["markup_error"])
			assert.Equal(t, "<div>Custom</div>", rows[0]["html"])
		})
	}
}

func TestContentRowsPutTheMarkupAfterTheMJML(t *testing.T) {
	content := markupContentFixture()
	content.MJML = "<mjml></mjml>"
	content.MarkupError = &responses.MarkupError{Code: "rate_limited", Message: "try again"}

	var labels []string
	for _, row := range templateContentRows(content) {
		labels = append(labels, row[0])
	}

	assert.Equal(t, []string{"MJML", "Markup", "Markup Error", "HTML", "Text", "Text Is Custom"}, labels)
	assert.Equal(t, []string{"mjml", "markup", "markup_error", "html", "text", "text_is_custom"}, templateContentColumns)
}

func TestMarkupErrorWithALineBreakPrintsOnOneLine(t *testing.T) {
	markupError := &responses.MarkupError{Code: "html_block", Message: "the design holds\nan HTML block"}
	assert.Equal(t, `html_block: the design holds\u000aan HTML block`, formatMarkupError(markupError))
}

package segments

import (
	"testing"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefinitionReaderDecodesTheHelpExample(t *testing.T) {
	definition, err := parseDefinition([]byte(definitionExample))

	require.NoError(t, err)
	assert.Equal(t, testDefinition(), *definition)
}

// TestDefinitionReaderAcceptsNumbersAFloat64Keeps checks values the SDK
// sends unchanged: a number with a fraction, such as 0.1, and a whole number
// in any notation from -2^53 to 2^53.
func TestDefinitionReaderAcceptsNumbersAFloat64Keeps(t *testing.T) {
	for _, value := range []string{"0.1", "3.0", "1e15", "9007199254740992", "-9007199254740992", "90071992547409.92e2",
		"9007199254740992.5", "0", "-0.0", "0e999", "1e-400", "1e-99999999999999999999"} {
		t.Run(value, func(t *testing.T) {
			_, err := parseDefinition([]byte(`{"conditions": [{"field": "attributes.score", "op": "equal", "value": ` + value + `}]}`))
			assert.NoError(t, err)
		})
	}
}

func TestDefinitionReaderAcceptsEveryKindOfValue(t *testing.T) {
	definition, err := parseDefinition([]byte(`{"schema_version": 1, "match": "any", "conditions": [
		{"field": "first_name", "op": "is_empty"},
		{"field": "first_name", "op": "is_empty", "value": null},
		{"field": "attributes.vip", "op": "equal", "value": true},
		{"field": "attributes.score", "op": "greater", "value": 2.5},
		{"match": "all", "conditions": []}
	]}`))

	require.NoError(t, err)
	require.Len(t, definition.Conditions, 5)
	assert.Nil(t, definition.Conditions[1].Condition.Value)
	assert.True(t, *definition.Conditions[2].Condition.Value.Bool)
	assert.InDelta(t, 2.5, *definition.Conditions[3].Condition.Value.Number, 0)
	assert.Equal(t, &common.SegmentGroup{Match: "all", Conditions: []common.SegmentCondition{}}, definition.Conditions[4].Group)
}

// TestDefinitionReaderNamesThePlaceOfEachMistake checks that each mistake of
// shape is a validation error that names its path in the file.
func TestDefinitionReaderNamesThePlaceOfEachMistake(t *testing.T) {
	condition := func(value string) string {
		return `{"conditions": [{"field": "campaign_count", "op": "opened_at_least", "value": ` + value + `}]}`
	}
	tests := []struct {
		name, file, message string
	}{
		{"unknown key of a condition",
			`{"conditions": [{"field": "a", "op": "equal"}, {"match": "any", "conditions": [{"field": "b", "op": "equal", "vlaue": 1}]}]}`,
			"conditions[1].conditions[0].vlaue is not a key of a segment condition"},
		{"unknown key of a group", `{"conditions": [{"match": "any", "condition": []}]}`,
			"conditions[0].condition is not a key of a segment group"},
		{"unknown key of a count", condition(`{"count": 3, "snice": "90 days ago"}`),
			"conditions[0].value.snice is not a key of a count"},
		{"unknown key of the definition", `{"schema_version": 1, "scheme": "all"}`,
			"scheme is not a key of a segment definition"},
		{"entry that is not an object", `{"conditions": ["attributes.plan"]}`,
			"conditions[0] must be an object, a condition or a group"},
		{"group entry that is not an object", `{"conditions": [{"match": "any", "conditions": [null]}]}`,
			"conditions[0].conditions[0] must be an object, a condition"},
		{"string schema_version", `{"schema_version": "1"}`,
			"schema_version must be an integer, written without a fraction or exponent"},
		{"schema_version of 1.0", `{"schema_version": 1.0}`,
			"schema_version must be an integer, written without a fraction or exponent"},
		{"schema_version too large", `{"schema_version": 99999999999999999999}`,
			"schema_version is too large an integer"},
		{"conditions that is not an array", `{"conditions": {"field": "a"}}`,
			"conditions must be an array"},
		{"group conditions that is not an array", `{"conditions": [{"match": "any", "conditions": null}]}`,
			"conditions[0].conditions must be an array"},
		{"string match", `{"match": ["all"]}`, "match must be a string"},
		{"number field", `{"conditions": [{"field": 1}]}`, "conditions[0].field must be a string"},
		{"array value", condition(`["pro"]`),
			"conditions[0].value must be a string, number, true, false, null or a count"},
		{"string count", condition(`{"count": "3"}`),
			"conditions[0].value.count must be an integer, written without a fraction or exponent"},
		{"count of 1e2", condition(`{"count": 1e2}`),
			"conditions[0].value.count must be an integer, written without a fraction or exponent"},
		{"number since", condition(`{"since": 90}`), "conditions[0].value.since must be a string"},
		{"value of 2^53 + 1", condition(`9007199254740993`),
			"conditions[0].value is a whole number outside -2^53 to 2^53 (9007199254740992), which cannot be sent exactly"},
		{"value of 2^53 + 1 with a fraction of zero", condition(`9007199254740993.0`),
			"conditions[0].value is a whole number outside -2^53 to 2^53 (9007199254740992), which cannot be sent exactly"},
		{"value of -2^53 - 1", condition(`-9007199254740993`),
			"conditions[0].value is a whole number outside -2^53 to 2^53 (9007199254740992), which cannot be sent exactly"},
		{"value of 1e16", condition(`1e16`),
			"conditions[0].value is a whole number outside -2^53 to 2^53 (9007199254740992), which cannot be sent exactly"},
		{"value of 1e400", condition(`1e400`), "conditions[0].value is too large a number"},
		{"key named twice", `{"conditions": [{"field": "a", "op": "equal", "field": "b"}]}`,
			"conditions[0].field is given twice"},
		{"key of the definition named twice", `{"match": "all", "match": "any"}`, "match is given twice"},
		{"group inside a group", `{"conditions": [{"field": "a"}, {"match": "any", "conditions": [{"match": "all", "bogus": 1}]}]}`,
			"conditions[1].conditions[0] is a group inside a group, which the API does not allow"},
		{"keys checked in sorted order", `{"zeta": 1, "alpha": 2}`, "alpha is not a key of a segment definition"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseDefinition([]byte(test.file))

			require.Error(t, err)
			assert.Equal(t, "--definition-file: "+test.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
		})
	}
}

func TestDefinitionReaderRefusesAFileThatIsNotAJSONObject(t *testing.T) {
	tests := map[string]struct {
		file, message string
	}{
		"empty":           {"", "--definition-file is not JSON: unexpected end of JSON input"},
		"white space":     {" \n", "--definition-file is not JSON: unexpected end of JSON input"},
		"not JSON":        {"match: all", "--definition-file is not JSON: invalid character 'm' looking for beginning of value"},
		"two values":      {"{} {}", "--definition-file is not JSON: invalid character '{' after top-level value"},
		"an array":        {`[{"match": "all"}]`, "--definition-file must be a JSON object, the segment definition"},
		"null":            {"null", "--definition-file must be a JSON object, the segment definition"},
		"a cut-off value": {`{"match": "all"`, "--definition-file is not JSON: unexpected end of JSON input"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseDefinition([]byte(test.file))

			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
		})
	}
}

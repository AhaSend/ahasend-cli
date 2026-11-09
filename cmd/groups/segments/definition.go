package segments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/spf13/cobra"
)

// definitionFlag is the flag that names a definition file.
const definitionFlag = "definition-file"

// definitionExample is the definition the help of the commands that take a
// definition file shows.
const definitionExample = `  {
    "schema_version": 1,
    "match": "all",
    "conditions": [
      {"field": "attributes.plan", "op": "equal", "value": "pro"},
      {"match": "any", "conditions": [
        {"field": "attributes.country", "op": "equal", "value": "DE"},
        {"field": "campaign_count", "op": "opened_at_least",
         "value": {"count": 3, "since": "90 days ago"}}
      ]}
    ]
  }`

// definitionHelp is the part of the help of the commands that take a
// definition file that says what the file holds.
const definitionHelp = `--definition-file names a JSON file that holds one segment definition in
the API's shape. A definition matches "all" or "any" of its conditions, and
may hold groups of conditions, each with a match of its own; a group cannot
hold a group. This file matches pro customers who are in Germany or opened
at least 3 campaigns in the last 90 days:

` + definitionExample + `

The CLI checks the file's keys and JSON types before the request, and names
the place of a mistake, such as conditions[1].conditions[0].value; the API
checks the fields, operators and values.`

// addDefinitionFlag adds --definition-file with usage.
func addDefinitionFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().String(definitionFlag, "", usage)
	cmdutil.MarkFileFlag(cmd, definitionFlag)
}

// readDefinition returns the definition in the file --definition-file names,
// or nil when the flag is not set. A file that does not hold a definition is
// a validation error that names the place of the mistake.
func readDefinition(cmd *cobra.Command) (*common.SegmentDefinition, error) {
	path, err := cmdutil.ReadNonEmpty(cmd, definitionFlag, "")
	if err != nil || path == nil {
		return nil, err
	}
	data, err := cmdutil.ReadInputFile(definitionFlag, *path)
	if err != nil {
		return nil, err
	}
	return parseDefinition(data)
}

// parseDefinition checks that data holds a segment definition, with the keys
// and JSON types the API takes, and decodes it.
func parseDefinition(data []byte) (*common.SegmentDefinition, error) {
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, errors.NewValidationError(fmt.Sprintf("--%s is not JSON: %s",
			definitionFlag, strings.TrimPrefix(err.Error(), "json: ")), nil)
	}
	root, err := readJSONValue(json.NewDecoder(bytes.NewReader(raw)))
	if err != nil {
		return nil, errors.NewValidationError(fmt.Sprintf("--%s is not JSON: %s",
			definitionFlag, strings.TrimPrefix(err.Error(), "json: ")), nil)
	}
	if root.kind != jsonObject {
		return nil, errors.NewValidationError(fmt.Sprintf("--%s must be a JSON object, the segment definition", definitionFlag), nil)
	}
	if err := checkDefinition(root); err != nil {
		return nil, err
	}

	var definition common.SegmentDefinition
	if err := json.Unmarshal(raw, &definition); err != nil {
		return nil, errors.NewValidationError(fmt.Sprintf("--%s: %s",
			definitionFlag, strings.TrimPrefix(err.Error(), "json: ")), nil)
	}
	return &definition, nil
}

// jsonKind is the kind of a JSON value.
type jsonKind int

const (
	jsonObject jsonKind = iota
	jsonArray
	jsonString
	jsonNumber
	jsonBool
	jsonNull
)

// jsonValue is a JSON value as the file writes it: an object keeps each of
// its keys, a key named twice included, and a number keeps its literal.
type jsonValue struct {
	kind    jsonKind
	members []jsonMember
	items   []jsonValue
	number  json.Number
}

// jsonMember is one key of an object and its value.
type jsonMember struct {
	key   string
	value jsonValue
}

// readJSONValue reads the next JSON value of decoder, which keeps numbers as
// json.Number.
func readJSONValue(decoder *json.Decoder) (jsonValue, error) {
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil {
		return jsonValue{}, err
	}
	switch token := token.(type) {
	case json.Delim:
		if token == '[' {
			value := jsonValue{kind: jsonArray}
			for decoder.More() {
				item, err := readJSONValue(decoder)
				if err != nil {
					return jsonValue{}, err
				}
				value.items = append(value.items, item)
			}
			_, err := decoder.Token()
			return value, err
		}
		value := jsonValue{kind: jsonObject}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return jsonValue{}, err
			}
			member, err := readJSONValue(decoder)
			if err != nil {
				return jsonValue{}, err
			}
			value.members = append(value.members, jsonMember{key: key.(string), value: member})
		}
		_, err := decoder.Token()
		return value, err
	case string:
		return jsonValue{kind: jsonString}, nil
	case json.Number:
		return jsonValue{kind: jsonNumber, number: token}, nil
	case bool:
		return jsonValue{kind: jsonBool}, nil
	default:
		return jsonValue{kind: jsonNull}, nil
	}
}

// keyChecks maps each key an object may hold to the check of its value at a
// path.
type keyChecks map[string]func(path string, value jsonValue) error

// checkDefinition checks the definition, the root object of the file.
func checkDefinition(definition jsonValue) error {
	return checkObject(definition, "", "a segment definition", keyChecks{
		"schema_version": checkInteger,
		"match":          checkString,
		"conditions": func(path string, value jsonValue) error {
			return checkEntries(path, value, checkRule)
		},
	})
}

// checkRule checks an entry of the definition's conditions: a group, which
// has a match key, or a condition.
func checkRule(path string, rule jsonValue) error {
	if rule.kind != jsonObject {
		return definitionError(path, "must be an object, a condition or a group")
	}
	if !rule.has("match") {
		return checkCondition(path, rule)
	}
	return checkObject(rule, path, "a segment group", keyChecks{
		"match": checkString,
		"conditions": func(path string, value jsonValue) error {
			return checkEntries(path, value, checkGroupEntry)
		},
	})
}

// checkGroupEntry checks an entry of a group's conditions, which is a
// condition: a group inside a group fails before its other keys are checked.
func checkGroupEntry(path string, entry jsonValue) error {
	if entry.kind != jsonObject {
		return definitionError(path, "must be an object, a condition")
	}
	if entry.has("match") {
		return definitionError(path, "is a group inside a group, which the API does not allow")
	}
	return checkCondition(path, entry)
}

// checkCondition checks a condition object.
func checkCondition(path string, condition jsonValue) error {
	return checkObject(condition, path, "a segment condition", keyChecks{
		"field": checkString,
		"op":    checkString,
		"value": checkConditionValue,
	})
}

// checkConditionValue checks the value of a condition: a string, a number a
// float64 keeps, true, false, null or a count.
func checkConditionValue(path string, value jsonValue) error {
	switch value.kind {
	case jsonString, jsonBool, jsonNull:
		return nil
	case jsonNumber:
		return checkSentNumber(path, value.number)
	case jsonObject:
		return checkObject(value, path, "a count", keyChecks{
			"count": checkInteger,
			"since": checkString,
		})
	default:
		return definitionError(path, "must be a string, number, true, false, null or a count")
	}
}

// checkObject checks object, at path, whose keys are those of checks, each
// named once, in the sorted order of its keys. what names the object in the
// error of a key it cannot hold.
func checkObject(object jsonValue, path, what string, checks keyChecks) error {
	members := slices.Clone(object.members)
	slices.SortStableFunc(members, func(a, b jsonMember) int { return strings.Compare(a.key, b.key) })
	for i, member := range members {
		memberPath := joinKey(path, member.key)
		if i > 0 && members[i-1].key == member.key {
			return definitionError(memberPath, "is given twice")
		}
		check, ok := checks[member.key]
		if !ok {
			return definitionError(memberPath, "is not a key of "+what)
		}
		if err := check(memberPath, member.value); err != nil {
			return err
		}
	}
	return nil
}

// checkEntries checks that value, at path, is an array, and each of its
// entries with check.
func checkEntries(path string, value jsonValue, check func(string, jsonValue) error) error {
	if value.kind != jsonArray {
		return definitionError(path, "must be an array")
	}
	for i, entry := range value.items {
		if err := check(fmt.Sprintf("%s[%d]", path, i), entry); err != nil {
			return err
		}
	}
	return nil
}

// checkString checks that value, at path, is a string.
func checkString(path string, value jsonValue) error {
	if value.kind != jsonString {
		return definitionError(path, "must be a string")
	}
	return nil
}

// checkInteger checks that value, at path, is a JSON integer: a number
// written with no fraction or exponent that fits an int, which the SDK
// decodes it into.
func checkInteger(path string, value jsonValue) error {
	if value.kind != jsonNumber || strings.ContainsAny(string(value.number), ".eE") {
		return definitionError(path, "must be an integer, written without a fraction or exponent")
	}
	if _, err := strconv.Atoi(string(value.number)); err != nil {
		return definitionError(path, "is too large an integer")
	}
	return nil
}

// maxExactWhole is 2^53, beyond which a float64, which the SDK sends a
// condition's number as, does not hold every whole number.
const maxExactWhole = "9007199254740992"

// checkSentNumber checks that number, at path, is one the SDK sends as it is
// written: it is finite as a float64, and when it has no fraction it lies
// between -2^53 and 2^53. A number with a fraction, such as 0.1, passes.
func checkSentNumber(path string, number json.Number) error {
	if f, err := number.Float64(); err != nil || math.IsInf(f, 0) {
		return definitionError(path, "is too large a number")
	}
	digits, exponent := decimalDigits(string(number))
	if digits == "" || exponent < 0 {
		// Zero, or a number with a fraction
		return nil
	}
	if width := len(digits) + exponent; width > len(maxExactWhole) ||
		(width == len(maxExactWhole) && digits+strings.Repeat("0", exponent) > maxExactWhole) {
		return definitionError(path, "is a whole number outside -2^53 to 2^53 (9007199254740992), which cannot be sent exactly")
	}
	return nil
}

// decimalDigits returns the significant digits of the JSON number literal,
// without leading or trailing zeros, and the power of ten that scales them
// to its value, which is negative for a value with a fraction. Zero has no
// digits.
func decimalDigits(literal string) (string, int) {
	mantissa, exponentPart, _ := strings.Cut(strings.ToLower(strings.TrimPrefix(literal, "-")), "e")
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return "", 0
	}
	trimmed := strings.TrimRight(digits, "0")
	return trimmed, decimalExponent(exponentPart) - len(fraction) + len(digits) - len(trimmed)
}

// maxDecimalExponent bounds the exponent decimalExponent returns, leaving
// room in an int, 32 bits included, for the digit counts added to it.
const maxDecimalExponent = 1 << 30

// decimalExponent returns the exponent of a JSON number literal, after its
// "e", or 0 for none. A larger exponent than maxDecimalExponent is clamped to
// it, since such a number is either not finite as a float64 or rounds to
// zero.
func decimalExponent(exponent string) int {
	if exponent == "" {
		return 0
	}
	value, err := strconv.Atoi(exponent)
	switch {
	case err == nil && value >= -maxDecimalExponent && value <= maxDecimalExponent:
		return value
	case strings.HasPrefix(exponent, "-"):
		return -maxDecimalExponent
	default:
		return maxDecimalExponent
	}
}

// has reports whether the object has key.
func (v jsonValue) has(key string) bool {
	return slices.ContainsFunc(v.members, func(member jsonMember) bool { return member.key == key })
}

// joinKey returns the path of key in the object at path.
func joinKey(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// definitionError is the validation error of the value at path of the
// definition file.
func definitionError(path, problem string) error {
	return errors.NewValidationError(fmt.Sprintf("--%s: %s %s", definitionFlag, path, problem), nil)
}

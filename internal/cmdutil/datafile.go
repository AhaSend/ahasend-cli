package cmdutil

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/errors"
)

// DecodeJSON decodes the one JSON value r holds, a value of kind such as
// "JSON object", into target; every JSON input a user gives is read with it.
// Numbers are kept as json.Number, so they are sent as written. Input that is
// empty, is not one value of kind (null included) or does not fit target is
// a validation error that names the input as subject, such as "--attributes".
func DecodeJSON(r io.Reader, target any, subject, kind string) error {
	return decodeJSON(r, target, subject, kind, false)
}

// DecodeJSONStrict is DecodeJSON for input whose every key target names,
// such as a recipients file: a key target lacks, such as a misspelt one, is a
// validation error that names it.
func DecodeJSONStrict(r io.Reader, target any, subject, kind string) error {
	return decodeJSON(r, target, subject, kind, true)
}

func decodeJSON(r io.Reader, target any, subject, kind string, strict bool) error {
	decoder := json.NewDecoder(r)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return jsonInputError(subject, kind, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.NewValidationError(subject+" must contain exactly one "+kind, nil)
	}
	return decodeRaw(raw, target, subject, kind, strict)
}

// decodeRaw decodes raw, one JSON value of kind, into target, keeping numbers
// as json.Number; strict rejects fields target lacks. null, and a value that
// does not fit target, are validation errors that name the input as subject.
func decodeRaw(raw json.RawMessage, target any, subject, kind string, strict bool) error {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.NewValidationError(subject+" must be a "+kind, nil)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if strict {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return jsonInputError(subject, kind, err)
	}
	return nil
}

// jsonInputError is the validation error of JSON input, named by subject,
// that does not decode as a value of kind. Its message names no Go type: a
// field of the wrong type is named with the JSON type it needs.
func jsonInputError(subject, kind string, err error) error {
	var typeErr *json.UnmarshalTypeError
	var syntaxErr *json.SyntaxError
	switch {
	case err == io.EOF:
		return errors.NewValidationError(subject+" is empty", nil)
	case stderrors.As(err, &typeErr) && fieldPath(typeErr.Field) != "":
		return errors.NewValidationError(fmt.Sprintf("%s is invalid: %s must be %s",
			subject, fieldPath(typeErr.Field), jsonTypeName(typeErr.Type)), nil)
	case stderrors.As(err, &typeErr), stderrors.As(err, &syntaxErr), stderrors.Is(err, io.ErrUnexpectedEOF):
		return errors.NewValidationError(subject+" must be a "+kind, nil)
	default:
		// Such as an unknown field, or a field value the field type refuses
		return errors.NewValidationError(subject+" is invalid: "+strings.TrimPrefix(err.Error(), "json: "), nil)
	}
}

// fieldPath is the path of keys a decoder error names, such as
// "substitutions.name", without the array indexes some Go versions put in it,
// so the message is the same whatever Go version built the CLI.
func fieldPath(field string) string {
	var keys []string
	for _, key := range strings.Split(field, ".") {
		if _, err := strconv.Atoi(key); err != nil {
			keys = append(keys, key)
		}
	}
	return strings.Join(keys, ".")
}

// jsonTypeName names the JSON type a value of Go type t is decoded from.
func jsonTypeName(t reflect.Type) string {
	if t == reflect.TypeOf(json.Number("")) {
		return "a number"
	}
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "true or false"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice, reflect.Array:
		return "a JSON array"
	default:
		return "a JSON object"
	}
}

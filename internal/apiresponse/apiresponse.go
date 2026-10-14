// Package apiresponse remembers the exact body the API returned for each
// decoded SDK result, so JSON output can print it unchanged.
//
// --output json prints the API's response body as the API sent it. A
// command holds only the SDK struct the body was decoded into, and encoding
// that struct again would not give the body back: it would reorder keys,
// drop keys the struct does not have, add keys the API left out and change
// number literals a float64 cannot hold. So the client records each body
// against the pointer to the struct decoded from it, and the JSON printer
// looks the body up by the pointer the command hands it.
//
// The lookup is by pointer identity, so it finds a body only for the very
// value the client returned: a command that prints a copy, a field of the
// result or a value it built gets no body, and the printer encodes that
// value instead. A response the client does not decode into a result, such
// as the event stream of the listen commands, has no body here either.
// Bodies are kept until the process exits, which a CLI run soon does.
package apiresponse

import "sync"

// bodies maps a decoded result pointer to the API response body behind it.
// Entries live until the process exits, so a key's address is never reused.
var bodies sync.Map

// Remember records body as the API response behind result, a pointer.
func Remember(result any, body []byte) {
	bodies.Store(result, body)
}

// Body returns the API response body recorded for result.
func Body(result any) ([]byte, bool) {
	body, ok := bodies.Load(result)
	if !ok {
		return nil, false
	}
	return body.([]byte), true
}

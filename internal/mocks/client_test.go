package mocks

import (
	"reflect"
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/AhaSend/ahasend-cli/internal/client"
)

func TestMockClient_HelperMethods(t *testing.T) {
	messageResponse := NewMessageResponse("msg-123")
	assert.Equal(t, "msg-123", *messageResponse.Data[0].ID)
}

func TestMockClient_TypedNilResult(t *testing.T) {
	mockClient := &MockClient{}
	mockClient.On("GetDomain", "example.com").Return((*responses.Domain)(nil), assert.AnError)

	result, err := mockClient.GetDomain("example.com")

	assert.ErrorIs(t, err, assert.AnError)
	assert.Nil(t, result)
}

// TestMockClient_EveryMethodReturnsNilAndTheError calls each client method
// that returns a pointer and an error with Return(nil, err): none panics, and
// each returns a nil pointer and the error.
func TestMockClient_EveryMethodReturnsNilAndTheError(t *testing.T) {
	clientType := reflect.TypeOf((*client.AhaSendClient)(nil)).Elem()
	errType := reflect.TypeOf((*error)(nil)).Elem()
	for i := 0; i < clientType.NumMethod(); i++ {
		method := clientType.Method(i)
		if method.Type.NumOut() != 2 || method.Type.Out(0).Kind() != reflect.Ptr || method.Type.Out(1) != errType {
			continue
		}
		t.Run(method.Name, func(t *testing.T) {
			mockClient := &MockClient{}
			fixed := method.Type.NumIn()
			if method.Type.IsVariadic() {
				fixed--
			}
			matchers := make([]interface{}, fixed)
			in := make([]reflect.Value, method.Type.NumIn())
			for j := range in {
				in[j] = reflect.Zero(method.Type.In(j))
				if j < fixed {
					matchers[j] = mock.Anything
				}
			}
			mockClient.On(method.Name, matchers...).Return(nil, assert.AnError)

			call := reflect.ValueOf(mockClient).MethodByName(method.Name)
			var out []reflect.Value
			if method.Type.IsVariadic() {
				out = call.CallSlice(in)
			} else {
				out = call.Call(in)
			}

			assert.True(t, out[0].IsNil())
			assert.Equal(t, assert.AnError, out[1].Interface())
		})
	}
}

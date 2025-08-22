package printer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// jsonHandler handles JSON output formatting with complete type safety
type jsonHandler struct {
	handlerBase
}

// GetFormat returns the format name
func (h *jsonHandler) GetFormat() string {
	return "json"
}

// Error handling
func (h *jsonHandler) HandleError(err error) error {
	if err == nil {
		return nil
	}

	errorOutput := map[string]interface{}{
		"error":   true,
		"message": err.Error(),
	}

	// Handle SDK APIError objects by returning their raw JSON response
	if apiErr, ok := err.(*api.APIError); ok {
		if len(apiErr.Raw) > 0 {
			// Try to format the raw JSON response
			var rawData interface{}
			if json.Unmarshal(apiErr.Raw, &rawData) == nil {
				// Output the raw API response and return nil (exit code 0)
				// This is intentional - in JSON mode we faithfully output API responses
				return h.printJSON(rawData)
			}
		}
		// If no raw response, return the APIError fields as JSON
		errorOutput = map[string]interface{}{
			"error":       true,
			"message":     apiErr.Message,
			"code":        apiErr.Code,
			"status_code": apiErr.StatusCode,
			"request_id":  apiErr.RequestID,
		}
	}

	// Add additional fields for structured errors
	if cliErr, ok := err.(interface{ Code() string }); ok {
		errorOutput["code"] = cliErr.Code()
	}

	if cliErr, ok := err.(interface{ Details() map[string]interface{} }); ok {
		errorOutput["details"] = cliErr.Details()
	}

	// Print the error as JSON
	if printErr := h.printJSON(errorOutput); printErr != nil {
		// If we can't print the JSON error, return both errors
		return fmt.Errorf("failed to print error as JSON: %w (original error: %v)", printErr, err)
	}

	// For API errors that we've already handled above, we returned nil
	// For other errors (connection, auth, etc), return the error for non-zero exit code
	if _, isAPIErr := err.(*api.APIError); isAPIErr {
		// API errors with structured responses return nil (exit code 0)
		return nil
	}
	// Non-API errors return the error (non-zero exit code)
	return err
}

// Domain responses
func (h *jsonHandler) HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error {
	if response == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleSingleDomain(domain *responses.Domain, config SingleConfig) error {
	if domain == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(domain)
}

// Message responses
func (h *jsonHandler) HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error {
	if response == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleSingleMessage(message *responses.Message, config SingleConfig) error {
	if message == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(message)
}

func (h *jsonHandler) HandleCreateMessage(response *responses.CreateMessageResponse, config CreateConfig) error {
	if response == nil {
		return h.HandleEmpty("No response received")
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleCancelMessage(response *CancelMessageResponse, config SimpleConfig) error {
	if response == nil {
		return h.HandleEmpty("No response received")
	}
	return h.printJSON(response)
}

// Webhook responses
func (h *jsonHandler) HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error {
	if response == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error {
	if webhook == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(webhook)
}

func (h *jsonHandler) HandleCreateWebhook(webhook *responses.Webhook, config CreateConfig) error {
	if webhook == nil {
		return h.HandleEmpty("No webhook created")
	}
	return h.printJSON(webhook)
}

func (h *jsonHandler) HandleUpdateWebhook(webhook *responses.Webhook, config UpdateConfig) error {
	if webhook == nil {
		return h.HandleEmpty("No webhook updated")
	}
	return h.printJSON(webhook)
}

func (h *jsonHandler) HandleDeleteWebhook(success bool, config DeleteConfig) error {
	result := map[string]interface{}{
		"success": success,
		"message": config.SuccessMessage,
	}
	return h.printJSON(result)
}

// Route responses
func (h *jsonHandler) HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error {
	if response == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleSingleRoute(route *responses.Route, config SingleConfig) error {
	if route == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(route)
}

func (h *jsonHandler) HandleCreateRoute(route *responses.Route, config CreateConfig) error {
	if route == nil {
		return h.HandleEmpty("No route created")
	}
	return h.printJSON(route)
}

func (h *jsonHandler) HandleUpdateRoute(route *responses.Route, config UpdateConfig) error {
	if route == nil {
		return h.HandleEmpty("No route updated")
	}
	return h.printJSON(route)
}

func (h *jsonHandler) HandleDeleteRoute(success bool, config DeleteConfig) error {
	result := map[string]interface{}{
		"success": success,
		"message": config.SuccessMessage,
	}
	return h.printJSON(result)
}

// Suppression responses
func (h *jsonHandler) HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error {
	if response == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleSingleSuppression(suppression *responses.Suppression, config SingleConfig) error {
	if suppression == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(suppression)
}

func (h *jsonHandler) HandleCreateSuppression(response *responses.CreateSuppressionResponse, config CreateConfig) error {
	if response == nil {
		return h.HandleEmpty("No response received")
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleDeleteSuppression(success bool, config DeleteConfig) error {
	result := map[string]interface{}{
		"success": success,
		"message": config.SuccessMessage,
	}
	return h.printJSON(result)
}

func (h *jsonHandler) HandleWipeSuppression(count int, config WipeConfig) error {
	result := map[string]interface{}{
		"success": true,
		"message": config.SuccessMessage,
		"count":   count,
	}
	return h.printJSON(result)
}

func (h *jsonHandler) HandleCheckSuppression(suppression *responses.Suppression, found bool, config CheckConfig) error {
	result := map[string]interface{}{
		"found": found,
	}

	if found {
		result["message"] = config.FoundMessage
		result["suppression"] = suppression
	} else {
		result["message"] = config.NotFoundMessage
	}

	return h.printJSON(result)
}

// SMTP responses
func (h *jsonHandler) HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error {
	if response == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error {
	if credential == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(credential)
}

func (h *jsonHandler) HandleCreateSMTP(credential *responses.SMTPCredential, config CreateConfig) error {
	if credential == nil {
		return h.HandleEmpty("No credential created")
	}
	return h.printJSON(credential)
}

func (h *jsonHandler) HandleDeleteSMTP(success bool, config DeleteConfig) error {
	result := map[string]interface{}{
		"success": success,
		"message": config.SuccessMessage,
	}
	return h.printJSON(result)
}

func (h *jsonHandler) HandleSMTPSend(result *SMTPSendResult, config SMTPSendConfig) error {
	if result == nil {
		return h.HandleEmpty("No send result available")
	}
	return h.printJSON(result)
}

// API Key responses
func (h *jsonHandler) HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error {
	if response == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error {
	if key == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.printJSON(key)
}

func (h *jsonHandler) HandleCreateAPIKey(key *responses.APIKey, config CreateConfig) error {
	if key == nil {
		return h.HandleEmpty("No API key created")
	}
	return h.printJSON(key)
}

func (h *jsonHandler) HandleUpdateAPIKey(key *responses.APIKey, config UpdateConfig) error {
	if key == nil {
		return h.HandleEmpty("No API key updated")
	}
	return h.printJSON(key)
}

func (h *jsonHandler) HandleDeleteAPIKey(success bool, config DeleteConfig) error {
	result := map[string]interface{}{
		"success": success,
		"message": config.SuccessMessage,
	}
	return h.printJSON(result)
}

// Statistics responses
func (h *jsonHandler) HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error {
	if response == nil {
		return h.HandleEmpty("No statistics available")
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error {
	if response == nil {
		return h.HandleEmpty("No statistics available")
	}
	return h.printJSON(response)
}

func (h *jsonHandler) HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error {
	if response == nil {
		return h.HandleEmpty("No statistics available")
	}
	return h.printJSON(response)
}

// Auth responses
func (h *jsonHandler) HandleAuthLogin(success bool, profile string, config AuthConfig) error {
	result := map[string]interface{}{
		"success": success,
		"message": config.SuccessMessage,
		"profile": profile,
	}
	return h.printJSON(result)
}

func (h *jsonHandler) HandleAuthLogout(success bool, config AuthConfig) error {
	result := map[string]interface{}{
		"success": success,
		"message": config.SuccessMessage,
	}
	return h.printJSON(result)
}

func (h *jsonHandler) HandleAuthStatus(status *AuthStatus, config AuthConfig) error {
	if status == nil {
		return h.HandleEmpty("No authentication status available")
	}
	return h.printJSON(status)
}

func (h *jsonHandler) HandleAuthSwitch(newProfile string, config AuthConfig) error {
	result := map[string]interface{}{
		"success": true,
		"message": config.SuccessMessage,
		"profile": newProfile,
	}
	return h.printJSON(result)
}

// Simple success and empty responses
func (h *jsonHandler) HandleSimpleSuccess(message string) error {
	result := map[string]interface{}{
		"message": message,
	}
	return h.printJSON(result)
}

func (h *jsonHandler) HandleEmpty(message string) error {
	result := map[string]interface{}{
		"empty":   true,
		"message": message,
	}
	return h.printJSON(result)
}

// printJSON is the core JSON formatting method
func (h *jsonHandler) printJSON(data interface{}) error {
	// Clean data by removing empty additionalproperties fields
	cleanedData := h.removeEmptyAdditionalProperties(data)

	buf := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(cleanedData); err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	_, err := h.writer.Write(buf.Bytes())
	return err
}

// removeEmptyAdditionalProperties recursively removes empty additionalproperties fields
func (h *jsonHandler) removeEmptyAdditionalProperties(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return h.removeEmptyAdditionalProperties(v.Elem().Interface())

	case reflect.Slice:
		result := make([]interface{}, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			cleanedItem := h.removeEmptyAdditionalProperties(v.Index(i).Interface())
			result = append(result, cleanedItem)
		}
		return result

	case reflect.Map:
		if v.Type().Key().Kind() == reflect.String {
			result := make(map[string]interface{})
			for _, key := range v.MapKeys() {
				keyStr := key.String()
				value := v.MapIndex(key).Interface()

				// Skip empty additionalproperties fields
				if keyStr == "additionalproperties" || keyStr == "AdditionalProperties" {
					if h.isEmptyValue(value) {
						continue // Skip this field
					}
				}

				cleanedValue := h.removeEmptyAdditionalProperties(value)
				result[keyStr] = cleanedValue
			}
			return result
		}
		return data

	case reflect.Struct:
		// Special handling for time.Time - don't process it as a regular struct
		if v.Type().String() == "time.Time" {
			return data
		}
		return h.cleanStruct(v)

	default:
		return data
	}
}

// cleanStruct cleans a struct by removing empty additionalproperties fields
func (h *jsonHandler) cleanStruct(v reflect.Value) interface{} {
	result := make(map[string]interface{})
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Get field name (use json tag if available)
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			if commaIdx := strings.Index(jsonTag, ","); commaIdx != -1 {
				fieldName = jsonTag[:commaIdx]
			} else {
				fieldName = jsonTag
			}
		}

		// Skip json:"-" fields
		if jsonTag := field.Tag.Get("json"); jsonTag == "-" {
			continue
		}

		// Skip empty additionalproperties fields
		if fieldName == "additionalproperties" || fieldName == "AdditionalProperties" {
			if h.isEmptyValue(fieldValue.Interface()) {
				continue // Skip this field
			}
		}

		cleanedValue := h.removeEmptyAdditionalProperties(fieldValue.Interface())
		result[fieldName] = cleanedValue
	}

	return result
}

// isEmptyValue checks if a value should be considered "empty" for additionalproperties
func (h *jsonHandler) isEmptyValue(value interface{}) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Ptr:
		return v.IsNil() || h.isEmptyValue(v.Elem().Interface())
	case reflect.Slice, reflect.Array:
		return v.Len() == 0
	case reflect.Map:
		return v.Len() == 0
	case reflect.String:
		return v.String() == ""
	case reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

package printer

import (
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// The fields of the views of a route, which the table, plain and CSV output
// of each view show in this order, except a table of routes, which shows
// routeTableColumns.
var (
	// routeFields are the fields of a route's list and detail views, without
	// the secret, which the API returns only when it creates the route.
	routeFields = slices.Concat([]string{
		"name", "id", "url", "enabled", "recipient", "attachments", "headers",
		"group_by_message_id", "strip_replies", "created_at", "updated_at",
	}, callFields)
	// createdRouteFields are the fields of a new route, which has no calls
	// yet.
	createdRouteFields = []string{
		"name", "id", "url", "enabled", "recipient", "secret", "attachments", "headers",
		"group_by_message_id", "strip_replies", "created_at", "updated_at",
	}
)

// routeTableColumns are the fields a table of routes shows.
var routeTableColumns = []string{"name", "id", "url", "enabled", "recipient"}

// routeHeaders names the table column or row of each route field.
var routeHeaders = withCallHeaders(map[string]string{
	"name":                "Name",
	"id":                  "ID",
	"url":                 "URL",
	"enabled":             "Enabled",
	"recipient":           "Recipient Filter",
	"secret":              "Secret",
	"attachments":         "Include Attachments",
	"headers":             "Include Headers",
	"group_by_message_id": "Group by Message ID",
	"strip_replies":       "Strip Replies",
	"created_at":          "Created",
	"updated_at":          "Updated",
})

func (h *jsonHandler) HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleRoute(route *responses.Route, config SingleConfig) error {
	return h.printAPIResponse(route)
}

func (h *jsonHandler) HandleCreateRoute(route *responses.Route, config ResultConfig) error {
	return h.printAPIResponse(route)
}

func (h *jsonHandler) HandleUpdateRoute(route *responses.Route, config ResultConfig) error {
	return h.printAPIResponse(route)
}

func (h *tableHandler) HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error {
	return writeTableList(h, config, pageOf(response), "routes", routeTableColumns, routeHeaders, func(route *responses.Route) map[string]string {
		return routeDisplayFields(route, "")
	})
}

func (h *tableHandler) HandleSingleRoute(route *responses.Route, config SingleConfig) error {
	if route == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(routeDetailRows(route))
	return nil
}

// HandleCreateRoute shows the route's secret, which the API shows only on
// creation.
func (h *tableHandler) HandleCreateRoute(route *responses.Route, config ResultConfig) error {
	if route == nil {
		return h.HandleEmpty("No route data received")
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(routeRows(createdRouteFields, route, route.Secret))
	return writeCreatedRouteNotices(h.writer, route)
}

func (h *tableHandler) HandleUpdateRoute(route *responses.Route, config ResultConfig) error {
	if route == nil {
		return h.HandleEmpty("No route data received")
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(routeDetailRows(route))
	return nil
}

func (h *plainHandler) HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error {
	return writePlainList(h, config, pageOf(response), "routes", func(route *responses.Route) error {
		return writeLabelLines(h.writer, "", routeDetailRows(route))
	})
}

func (h *plainHandler) HandleSingleRoute(route *responses.Route, config SingleConfig) error {
	if route == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", routeDetailRows(route))
}

// HandleCreateRoute shows the route's secret, which the API shows only on
// creation.
func (h *plainHandler) HandleCreateRoute(route *responses.Route, config ResultConfig) error {
	if route == nil {
		return h.HandleEmpty("No route data received")
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", routeRows(createdRouteFields, route, route.Secret)); err != nil {
		return err
	}
	return writeCreatedRouteNotices(h.writer, route)
}

func (h *plainHandler) HandleUpdateRoute(route *responses.Route, config ResultConfig) error {
	if route == nil {
		return h.HandleEmpty("No route data received")
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", routeDetailRows(route))
}

func (h *csvHandler) HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error {
	return writeCSVList(h, pageOf(response), routeFields, func(route *responses.Route) map[string]string {
		return routeCSVFields(route, "")
	})
}

func (h *csvHandler) HandleSingleRoute(route *responses.Route, config SingleConfig) error {
	if route == nil {
		return nil // No CSV output for empty data
	}
	return h.writeFieldMaps(routeFields, routeCSVFields(route, ""))
}

// HandleCreateRoute writes the route with its secret, which the API shows
// only on creation.
func (h *csvHandler) HandleCreateRoute(route *responses.Route, config ResultConfig) error {
	if route == nil {
		return nil // No CSV output for empty data
	}
	return h.writeFieldMaps(createdRouteFields, routeCSVFields(route, route.Secret))
}

func (h *csvHandler) HandleUpdateRoute(route *responses.Route, config ResultConfig) error {
	if route == nil {
		return nil // No CSV output for empty data
	}
	return h.writeFieldMaps(routeFields, routeCSVFields(route, ""))
}

// routeCSVFields maps a route to its csv values, with secret as the value of
// the secret column.
func routeCSVFields(route *responses.Route, secret string) map[string]string {
	values := callValues(route.SuccessCount, route.ErrorCount, route.ErrorsSinceLastSuccess,
		formatOptionalCSVTime(route.LastRequestAt))
	maps.Copy(values, map[string]string{
		"id":                  formatUUID(route.ID),
		"name":                route.Name,
		"url":                 route.URL,
		"enabled":             formatBool(route.Enabled),
		"recipient":           route.Recipient,
		"secret":              secret,
		"attachments":         formatBool(route.Attachments),
		"headers":             formatBool(route.Headers),
		"group_by_message_id": formatBool(route.GroupByMessageID),
		"strip_replies":       formatBool(route.StripReplies),
		"created_at":          formatCSVTime(route.CreatedAt),
		"updated_at":          formatCSVTime(route.UpdatedAt),
	})
	return values
}

// routeDisplayFields maps each route field to its table and plain value, with
// secret as the value of the secret field.
func routeDisplayFields(route *responses.Route, secret string) map[string]string {
	values := callValues(route.SuccessCount, route.ErrorCount, route.ErrorsSinceLastSuccess,
		formatOptionalTime(route.LastRequestAt))
	maps.Copy(values, map[string]string{
		"name":                route.Name,
		"id":                  formatUUID(route.ID),
		"url":                 route.URL,
		"enabled":             formatBooleanStatus(route.Enabled),
		"recipient":           route.Recipient,
		"secret":              secret,
		"attachments":         formatBooleanStatus(route.Attachments),
		"headers":             formatBooleanStatus(route.Headers),
		"group_by_message_id": formatBooleanStatus(route.GroupByMessageID),
		"strip_replies":       formatBooleanStatus(route.StripReplies),
		"created_at":          formatTime(route.CreatedAt),
		"updated_at":          formatTime(route.UpdatedAt),
	})
	return values
}

// routeDetailRows returns the rows of a route's detail and plain list views.
func routeDetailRows(route *responses.Route) [][]string {
	return routeRows(routeFields, route, "")
}

// routeRows returns the rows of fields of a route, with secret as the value
// of the secret row.
func routeRows(fields []string, route *responses.Route, secret string) [][]string {
	return fieldRows(fields, routeHeaders, routeDisplayFields(route, secret))
}

// writeCreatedRouteNotices tells to save the secret of a new route, when the
// API returned one, and how to start the route when it is disabled.
func writeCreatedRouteNotices(w io.Writer, route *responses.Route) error {
	if route.Secret != "" {
		if _, err := fmt.Fprintf(w, "\n%s\n", routeSecretNotice); err != nil {
			return err
		}
	}
	if route.Enabled {
		return nil
	}
	_, err := fmt.Fprintf(w, "\n%s\n", routeDisabledNotice)
	return err
}

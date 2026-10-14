package cmdutil

import (
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/spf13/cobra"
)

const limitUsage = "Maximum number of items to return (0-100; 0 uses the API default)"

// AddPaginationFlags adds --limit, --after and --before to a list command whose
// endpoint pages both ways.
func AddPaginationFlags(cmd *cobra.Command) {
	cmd.Flags().Int32("limit", 0, limitUsage)
	cmd.Flags().String("after", "", "Return the page after this cursor")
	cmd.Flags().String("before", "", "Return the page before this cursor")
}

// ReadPagination returns the pagination flags for the command to copy into its
// SDK params. A zero limit and an empty cursor are returned as nil.
func ReadPagination(cmd *cobra.Command) (limit *int32, after, before *string, err error) {
	limitValue, _ := cmd.Flags().GetInt32("limit")
	if err := validation.ValidatePageLimit(limitValue); err != nil {
		return nil, nil, nil, err
	}
	if err := RejectTogether(cmd, "after", "before"); err != nil {
		return nil, nil, nil, err
	}
	afterValue, _ := cmd.Flags().GetString("after")
	beforeValue, _ := cmd.Flags().GetString("before")

	if limitValue > 0 {
		limit = &limitValue
	}
	if afterValue != "" {
		after = &afterValue
	}
	if beforeValue != "" {
		before = &beforeValue
	}
	return limit, after, before, nil
}

// AddLegacyCursorAlias keeps --cursor, the flag older versions paged with,
// as a hidden, deprecated alias of --after.
func AddLegacyCursorAlias(cmd *cobra.Command) {
	AddFlagAlias(cmd, "cursor", "after")
}

// ReadPaginationParams returns the pagination flags as the SDK's pagination
// params.
func ReadPaginationParams(cmd *cobra.Command) (common.PaginationParams, error) {
	limit, after, before, err := ReadPagination(cmd)
	if err != nil {
		return common.PaginationParams{}, err
	}
	return common.PaginationParams{Limit: limit, After: after, Before: before}, nil
}

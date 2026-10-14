package printer

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/common"
)

// successMessage returns the API's message, or the command's success message
// when the API sent none.
func successMessage(response *common.SuccessResponse, config ResultConfig) string {
	if response == nil || response.Message == "" {
		return config.SuccessMessage
	}
	return response.Message
}

func (h *jsonHandler) HandleSuccessResponse(response *common.SuccessResponse, config ResultConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleSuccessResponse(response *common.SuccessResponse, config ResultConfig) error {
	_, err := fmt.Fprintf(h.writer, "%s\n", successMessage(response, config))
	return err
}

func (h *plainHandler) HandleSuccessResponse(response *common.SuccessResponse, config ResultConfig) error {
	_, err := fmt.Fprintf(h.writer, "%s\n", successMessage(response, config))
	return err
}

func (h *csvHandler) HandleSuccessResponse(response *common.SuccessResponse, config ResultConfig) error {
	return h.writeRows([]string{"message"}, []string{successMessage(response, config)})
}

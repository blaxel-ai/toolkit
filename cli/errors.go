package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type ErrorModel struct {
	Error string   `json:"error"`
	Code  int      `json:"code"`
	Stack []string `json:"stack"`
}

func ErrorHandler(request *http.Request, kind string, name string, body string) error {
	var errorModel ErrorModel
	if err := json.Unmarshal([]byte(body), &errorModel); err != nil {
		return fmt.Errorf("failed to unmarshal error: %w", err)
	}

	var err error

	workspace := request.Header.Get("X-Blaxel-Workspace")
	workspace = strings.ReplaceAll(workspace, "\n", "")
	workspace = strings.ReplaceAll(workspace, "\r", "")
	if workspace != "" {
		if errorModel.Code == 401 {
			resourceFullName := fmt.Sprintf("%s:%s", kind, name)
			if name == "" {
				resourceFullName = kind
			}
			err = fmt.Errorf("You are not authorized to access the resource %s on workspace %s. Please login again.", resourceFullName, workspace)
		} else {
			err = fmt.Errorf("Resource %s:%s:%s: %s (Code: %d)", kind, workspace, name, errorModel.Error, errorModel.Code)
		}
	} else {
		err = fmt.Errorf("Resource %s:%s: %s (Code: %d)", kind, name, errorModel.Error, errorModel.Code)
	}

	if verbose && len(errorModel.Stack) > 0 {
		errMsg := err.Error() + "\nStack trace:"
		for _, line := range errorModel.Stack {
			errMsg += fmt.Sprintf("\n  %s", line)
		}
		err = fmt.Errorf("%s", errMsg)
	}
	return err
}

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type ErrorModel struct {
	Error string   `json:"error"`
	Code  int      `json:"code"`
	Stack []string `json:"stack"`
}

func ErrorHandler(request *http.Request, kind string, name string, body string) {
	var error ErrorModel
	if err := json.Unmarshal([]byte(body), &error); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// Afficher l'erreur et le code

	workspace := request.Header.Get("X-Blaxel-Workspace")
	workspace = strings.ReplaceAll(workspace, "\n", "")
	workspace = strings.ReplaceAll(workspace, "\r", "")
	if workspace != "" {
		if error.Code == 401 {
			resourceFullName := fmt.Sprintf("%s:%s", kind, name)
			if name == "" {
				resourceFullName = kind
			}
			fmt.Printf("You are not authorized to access the resource %s on workspace %s. Please login again.\n", resourceFullName, workspace)
		} else {
			fmt.Printf("Resource %s:%s:%s: %s (Code: %d)\n", kind, workspace, name, error.Error, error.Code)
		}
	} else {
		fmt.Printf("Resource %s:%s: %s (Code: %d)\n", kind, name, error.Error, error.Code)
	}

	// Afficher le stack trace seulement s'il existe
	if verbose && len(error.Stack) > 0 {
		fmt.Println("Stack trace:")
		for _, line := range error.Stack {
			fmt.Printf("  %s\n", line)
		}
	}
}

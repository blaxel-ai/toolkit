package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	core.RegisterCommand("drive", func() *cobra.Command {
		return DriveCmd()
	})
}

// driveMountRequest is the request body for mounting a drive.
type driveMountRequest struct {
	DriveName string `json:"driveName"`
	MountPath string `json:"mountPath"`
	DrivePath string `json:"drivePath,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
	UIDMap    string `json:"uidMap,omitempty"`
	GIDMap    string `json:"gidMap,omitempty"`
}

// driveMountResponse is the response from mounting a drive.
type driveMountResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	DriveName string `json:"driveName"`
	MountPath string `json:"mountPath"`
	DrivePath string `json:"drivePath"`
	ReadOnly  bool   `json:"readOnly"`
	UIDMap    string `json:"uidMap"`
	GIDMap    string `json:"gidMap"`
}

// driveUnmountResponse is the response from unmounting a drive.
type driveUnmountResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	MountPath string `json:"mountPath"`
}

// driveMountInfo describes a single mounted drive.
type driveMountInfo struct {
	DriveName string `json:"driveName"`
	MountPath string `json:"mountPath"`
	DrivePath string `json:"drivePath"`
	ReadOnly  bool   `json:"readOnly"`
	UIDMap    string `json:"uidMap"`
	GIDMap    string `json:"gidMap"`
}

// driveListResponse is the response from listing mounted drives.
type driveListResponse struct {
	Mounts []driveMountInfo `json:"mounts"`
}

// sandboxAPIError is an error returned by the sandbox API.
type sandboxAPIError struct {
	Error string `json:"error"`
}

func DriveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drive",
		Short: "Manage drives and drive mounts on sandboxes",
		Long: `Manage drives and drive mounts on sandboxes.

` + "```" + `
Drive CRUD:
  bl drive list                       List all drives in the workspace
  bl drive get <name>                 Get details of a specific drive
  bl drive create                     Create a new drive
  bl drive delete <name>              Delete a drive

Sandbox mount operations:
  bl drive mount --sandbox <s> ...    Mount a drive to a running sandbox
  bl drive unmount --sandbox <s> ...  Unmount a drive from a running sandbox
  bl drive mounts --sandbox <s>       List drives mounted in a running sandbox
` + "```",
		Example: `  # List all drives
  bl drive list

  # Get details of a drive
  bl drive get my-drive

  # Create a new drive
  bl drive create --name my-drive --region us-pdx-1

  # Delete a drive
  bl drive delete my-drive

  # Mount a drive to a sandbox
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data

  # Unmount a drive from a sandbox
  bl drive unmount --sandbox my-sandbox --mount-path /mnt/data

  # List mounted drives in a sandbox
  bl drive mounts --sandbox my-sandbox`,
	}

	// CRUD subcommands
	cmd.AddCommand(DriveListCmd())
	cmd.AddCommand(DriveGetCmd())
	cmd.AddCommand(DriveCreateCmd())
	cmd.AddCommand(DriveDeleteCmd())

	// Mount subcommands
	cmd.AddCommand(DriveMountCmd())
	cmd.AddCommand(DriveUnmountCmd())
	cmd.AddCommand(DriveMountsCmd())
	return cmd
}

func DriveMountCmd() *cobra.Command {
	var sandboxName, driveName, mountPath, drivePath, uidMap, gidMap string
	var readOnly bool

	cmd := &cobra.Command{
		Use:   "mount",
		Short: "Mount a drive to a sandbox",
		Long: `Mount or re-mount a drive to a sandbox environment.

This command attaches an agent drive to a local path inside the sandbox using
the blfs filesystem. It can be used as a recovery tool when mounts are lost.`,
		Example: `  # Mount a drive with default settings
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data

  # Mount a subdirectory of the drive
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data --drive-path /subdir

  # Mount as read-only
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data --read-only

  # Mount with UID/GID mapping
  bl drive mount --sandbox my-sandbox --drive my-drive --mount-path /mnt/data --uid-map 1000 --gid-map 1000`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			sandboxURL, token := resolveSandbox(ctx, sandboxName)

			body := driveMountRequest{
				DriveName: driveName,
				MountPath: mountPath,
				DrivePath: drivePath,
				ReadOnly:  readOnly,
				UIDMap:    uidMap,
				GIDMap:    gidMap,
			}

			jsonBody, err := json.Marshal(body)
			if err != nil {
				core.PrintError("Drive mount", fmt.Errorf("failed to marshal request: %w", err))
				core.ExitWithError(err)
			}

			resp, err := sandboxRequest(ctx, http.MethodPost, sandboxURL, "/drives/mount", token, bytes.NewReader(jsonBody))
			if err != nil {
				core.PrintError("Drive mount", err)
				core.ExitWithError(err)
			}
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				core.PrintError("Drive mount", fmt.Errorf("failed to read response: %w", err))
				core.ExitWithError(err)
			}

			if resp.StatusCode != http.StatusOK {
				handleSandboxAPIError(respBody, resp.StatusCode, "mount drive")
			}

			var mountResp driveMountResponse
			if err := json.Unmarshal(respBody, &mountResp); err != nil {
				core.PrintError("Drive mount", fmt.Errorf("failed to parse response: %w", err))
				core.ExitWithError(err)
			}

			outputFormat := core.GetOutputFormat()
			if outputFormat == "json" || outputFormat == "yaml" {
				outputDriveData(&mountResp, outputFormat)
				return
			}

			core.PrintInfo(fmt.Sprintf("Drive '%s' mounted to '%s' in sandbox '%s'", driveName, mountPath, sandboxName))
		},
	}

	cmd.Flags().StringVar(&sandboxName, "sandbox", "", "Name of the sandbox")
	cmd.Flags().StringVar(&driveName, "drive", "", "Name of the drive to mount")
	cmd.Flags().StringVar(&mountPath, "mount-path", "", "Local path inside the sandbox to mount the drive")
	cmd.Flags().StringVar(&drivePath, "drive-path", "", "Subdirectory within the drive to mount (optional, defaults to /)")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Mount the drive as read-only")
	cmd.Flags().StringVar(&uidMap, "uid-map", "", "Local UID to map (filer UID is always 0)")
	cmd.Flags().StringVar(&gidMap, "gid-map", "", "Local GID to map (filer GID is always 0)")
	_ = cmd.MarkFlagRequired("sandbox")
	_ = cmd.MarkFlagRequired("drive")
	_ = cmd.MarkFlagRequired("mount-path")

	return cmd
}

func DriveUnmountCmd() *cobra.Command {
	var sandboxName, mountPath string

	cmd := &cobra.Command{
		Use:   "unmount",
		Short: "Unmount a drive from a sandbox",
		Long:  `Unmount a previously mounted drive from the specified local path inside a sandbox.`,
		Example: `  # Unmount a drive
  bl drive unmount --sandbox my-sandbox --mount-path /mnt/data`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			sandboxURL, token := resolveSandbox(ctx, sandboxName)

			// URL-encode the mount path for safe inclusion in the request path
			encodedPath := url.PathEscape(strings.TrimPrefix(mountPath, "/"))
			apiPath := fmt.Sprintf("/drives/mount/%s", encodedPath)

			resp, err := sandboxRequest(ctx, http.MethodDelete, sandboxURL, apiPath, token, nil)
			if err != nil {
				core.PrintError("Drive unmount", err)
				core.ExitWithError(err)
			}
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				core.PrintError("Drive unmount", fmt.Errorf("failed to read response: %w", err))
				core.ExitWithError(err)
			}

			if resp.StatusCode != http.StatusOK {
				handleSandboxAPIError(respBody, resp.StatusCode, "unmount drive")
			}

			var unmountResp driveUnmountResponse
			if err := json.Unmarshal(respBody, &unmountResp); err != nil {
				core.PrintError("Drive unmount", fmt.Errorf("failed to parse response: %w", err))
				core.ExitWithError(err)
			}

			outputFormat := core.GetOutputFormat()
			if outputFormat == "json" || outputFormat == "yaml" {
				outputDriveData(&unmountResp, outputFormat)
				return
			}

			core.PrintInfo(fmt.Sprintf("Drive unmounted from '%s' in sandbox '%s'", mountPath, sandboxName))
		},
	}

	cmd.Flags().StringVar(&sandboxName, "sandbox", "", "Name of the sandbox")
	cmd.Flags().StringVar(&mountPath, "mount-path", "", "Mount path to detach (must start with /)")
	_ = cmd.MarkFlagRequired("sandbox")
	_ = cmd.MarkFlagRequired("mount-path")

	return cmd
}

func DriveMountsCmd() *cobra.Command {
	var sandboxName string

	cmd := &cobra.Command{
		Use:     "mounts",
		Short:   "List mounted drives in a sandbox",
		Long:    `List all currently mounted drives in a sandbox environment.`,
		Example: `  # List all mounted drives
  bl drive mounts --sandbox my-sandbox`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			sandboxURL, token := resolveSandbox(ctx, sandboxName)

			resp, err := sandboxRequest(ctx, http.MethodGet, sandboxURL, "/drives/mount", token, nil)
			if err != nil {
				core.PrintError("Drive mounts", err)
				core.ExitWithError(err)
			}
			defer resp.Body.Close()

			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				core.PrintError("Drive mounts", fmt.Errorf("failed to read response: %w", err))
				core.ExitWithError(err)
			}

			if resp.StatusCode != http.StatusOK {
				handleSandboxAPIError(respBody, resp.StatusCode, "list mounted drives")
			}

			var listResp driveListResponse
			if err := json.Unmarshal(respBody, &listResp); err != nil {
				core.PrintError("Drive mounts", fmt.Errorf("failed to parse response: %w", err))
				core.ExitWithError(err)
			}

			if len(listResp.Mounts) == 0 {
				core.PrintInfo(fmt.Sprintf("No drives mounted in sandbox '%s'", sandboxName))
				return
			}

			outputFormat := core.GetOutputFormat()
			if outputFormat == "json" || outputFormat == "yaml" {
				outputDriveData(&listResp, outputFormat)
				return
			}

			// Convert to generic slices for table output
			jsonData, err := json.Marshal(listResp.Mounts)
			if err != nil {
				core.PrintError("Drive mounts", fmt.Errorf("failed to marshal mounts: %w", err))
				core.ExitWithError(err)
			}

			var slices []interface{}
			if err := json.Unmarshal(jsonData, &slices); err != nil {
				core.PrintError("Drive mounts", fmt.Errorf("failed to unmarshal mounts: %w", err))
				core.ExitWithError(err)
			}

			resource := core.Resource{
				Kind:     "DriveMount",
				Plural:   "mounts",
				Singular: "mount",
				Fields: []core.Field{
					{Key: "DRIVE", Value: "driveName"},
					{Key: "MOUNT_PATH", Value: "mountPath"},
					{Key: "DRIVE_PATH", Value: "drivePath"},
					{Key: "READ_ONLY", Value: "readOnly"},
					{Key: "UID_MAP", Value: "uidMap"},
					{Key: "GID_MAP", Value: "gidMap"},
				},
			}

			core.Output(resource, slices, outputFormat)
		},
	}

	cmd.Flags().StringVar(&sandboxName, "sandbox", "", "Name of the sandbox")
	_ = cmd.MarkFlagRequired("sandbox")

	return cmd
}

// driveResource returns the Drive resource definition from the resource registry.
func driveResource() *core.Resource {
	for _, r := range core.GetResources() {
		if r.Kind == "Drive" {
			return r
		}
	}
	return nil
}

func DriveListCmd() *cobra.Command {
	var pageLimit int
	var pageCursor string
	var fetchAll bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all drives in the workspace",
		Long:  `List all drives in the current workspace.`,
		Example: `  # List all drives
  bl drive list

  # List drives in JSON format
  bl drive list -o json

  # Fetch the next page
  bl drive list --cursor <cursor>

  # Fetch every drive
  bl drive list --all`,
		Aliases: []string{"ls"},
		Run: func(cmd *cobra.Command, args []string) {
			r := driveResource()
			if r == nil {
				core.PrintError("Drive", fmt.Errorf("drive resource not found"))
				core.ExitWithError(fmt.Errorf("drive resource not found"))
			}
			ListFnPaginated(r, pageLimit, pageCursor, fetchAll)
		},
	}

	cmd.Flags().IntVar(&pageLimit, "limit", core.DefaultPageLimit, "Maximum number of items to return (auto-paginates when above 200)")
	cmd.Flags().StringVar(&pageCursor, "cursor", "", "Cursor from a previous page to fetch the next page of results")
	cmd.Flags().BoolVar(&fetchAll, "all", false, "Fetch all pages (may be slow for large collections)")

	return cmd
}

func DriveGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Get details of a specific drive",
		Long:  `Get detailed information about a drive in the current workspace.`,
		Example: `  # Get drive details
  bl drive get my-drive

  # Get drive details in JSON format
  bl drive get my-drive -o json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			r := driveResource()
			if r == nil {
				core.PrintError("Drive", fmt.Errorf("drive resource not found"))
				core.ExitWithError(fmt.Errorf("drive resource not found"))
			}
			GetFn(r, args[0])
		},
	}
}

func DriveCreateCmd() *cobra.Command {
	var name, region string
	var size int64

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new drive",
		Long:  `Create a new drive in the current workspace.`,
		Example: `  # Create a drive in a specific region
  bl drive create --name my-drive --region us-pdx-1

  # Create a drive with a size limit (in GB)
  bl drive create --name my-drive --region us-pdx-1 --size 10`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			client := core.GetClient()

			params := blaxel.DriveNewParams{
				Metadata: blaxel.MetadataParam{
					Name: name,
				},
				Spec: blaxel.DriveSpecParam{
					Region: blaxel.String(region),
				},
			}
			if size > 0 {
				params.Spec.Size = blaxel.Int(size)
			}

			resp, err := client.Drives.New(ctx, params)
			if err != nil {
				core.PrintError("Drive create", fmt.Errorf("failed to create drive: %w", err))
				core.ExitWithError(err)
			}

			outputFormat := core.GetOutputFormat()
			if outputFormat == "json" || outputFormat == "yaml" {
				outputDriveData(resp, outputFormat)
				return
			}

			core.PrintInfo(fmt.Sprintf("Drive '%s' created", name))
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Name of the drive")
	cmd.Flags().StringVar(&region, "region", "", "Deployment region (e.g., us-pdx-1, eu-lon-1)")
	cmd.Flags().Int64Var(&size, "size", 0, "Size limit in GB (optional, 0 for unlimited)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("region")

	return cmd
}

func DriveDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a drive",
		Long:  `Delete a drive from the current workspace.`,
		Example: `  # Delete a drive
  bl drive delete my-drive`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			r := driveResource()
			if r == nil {
				core.PrintError("Drive", fmt.Errorf("drive resource not found"))
				core.ExitWithError(fmt.Errorf("drive resource not found"))
			}
			if err := DeleteFn(r, args[0]); err != nil {
				core.ExitWithError(err)
			}
		},
	}
}

// resolveSandbox retrieves the sandbox URL and auth token for the given sandbox name.
func resolveSandbox(ctx context.Context, sandboxName string) (sandboxURL, token string) {
	currentContext, _ := blaxel.CurrentContext()
	workspace := currentContext.Workspace
	if workspace == "" {
		err := fmt.Errorf("no workspace found in current context. Please run 'bl login' first")
		core.PrintError("Drive", err)
		core.ExitWithError(err)
	}

	credentials, _ := blaxel.LoadCredentials(workspace)
	if !credentials.IsValid() {
		err := fmt.Errorf("no valid credentials found. Please run 'bl login' first")
		core.PrintError("Drive", err)
		core.ExitWithError(err)
	}

	token = credentials.AccessToken
	if token == "" {
		token = credentials.APIKey
	}
	if token == "" {
		err := fmt.Errorf("no access token or Blaxel API key found. Please run 'bl login' first")
		core.PrintError("Drive", err)
		core.ExitWithError(err)
	}

	client := core.GetClient()
	sbx, err := client.Sandboxes.Get(ctx, sandboxName, blaxel.SandboxGetParams{})
	if err != nil {
		var apiErr *blaxel.Error
		if isBlaxelError(err, &apiErr) && apiErr.StatusCode == 404 {
			err = fmt.Errorf("sandbox '%s' not found", sandboxName)
			core.PrintError("Drive", err)

			sandboxes, listErr := client.Sandboxes.List(ctx, blaxel.SandboxListParams{})
			if listErr == nil && sandboxes != nil && len(sandboxes.Data) > 0 {
				names := make([]string, 0, len(sandboxes.Data))
				for _, sb := range sandboxes.Data {
					if sb.Metadata.Name != "" {
						names = append(names, sb.Metadata.Name)
					}
				}
				if len(names) > 0 {
					core.Print(fmt.Sprintf("Available sandboxes: %s\n", strings.Join(names, ", ")))
				}
			}
			core.ExitWithError(err)
		}
		err = fmt.Errorf("failed to get sandbox '%s': %w", sandboxName, err)
		core.PrintError("Drive", err)
		core.ExitWithError(err)
	}

	sandboxURL = sbx.Metadata.URL
	if sandboxURL == "" {
		sandboxURL = blaxel.BuildSandboxURL(workspace, sandboxName)
	}

	return sandboxURL, token
}

// sandboxRequest makes an authenticated HTTP request to the sandbox API.
func sandboxRequest(ctx context.Context, method, sandboxURL, path, token string, body io.Reader) (*http.Response, error) {
	url := strings.TrimSuffix(sandboxURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to sandbox failed: %w", err)
	}

	return resp, nil
}

// handleSandboxAPIError extracts an error message from a sandbox API response and exits.
func handleSandboxAPIError(body []byte, statusCode int, operation string) {
	var apiErr sandboxAPIError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Error != "" {
		err := fmt.Errorf("failed to %s (HTTP %d): %s", operation, statusCode, apiErr.Error)
		core.PrintError("Drive", err)
		core.ExitWithError(err)
	}
	err := fmt.Errorf("failed to %s (HTTP %d): %s", operation, statusCode, string(body))
	core.PrintError("Drive", err)
	core.ExitWithError(err)
}

// outputDriveData marshals the given data to JSON or YAML format and prints it.
func outputDriveData(data interface{}, format string) {
	var output []byte
	var err error

	switch format {
	case "json":
		output, err = json.MarshalIndent(data, "", "  ")
	case "yaml":
		output, err = yaml.Marshal(data)
	}
	if err != nil {
		core.PrintError("Drive", fmt.Errorf("failed to marshal output: %w", err))
		os.Exit(1)
	}
	fmt.Println(string(output))
}

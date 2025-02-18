package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func (r *Operations) LoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login [workspace]",
		Short: "Login to Blaxel",
		Args:  cobra.MaximumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			workspace := "" // Default workspace
			if len(args) > 0 {
				workspace = args[0]
			}
			if workspace == "" {
				fmt.Println("Error: Enter a workspace")
				os.Exit(1)
			}

			if os.Getenv("BL_AUTHENTICATION_CLIENT_CREDENTIALS") != "" {
				r.ClientCredentialsLogin(workspace, os.Getenv("BL_AUTHENTICATION_CLIENT_CREDENTIALS"))
				return
			}

			if os.Getenv("BL_API_KEY") != "" {
				r.ApiKeyLogin(workspace)
				return
			}

			options := []string{
				"Login with your browser",
				"Login with API key",
			}

			// Initialize variables for keyboard input handling
			selectedIndex := 0
			fmt.Print("\033[?25l")       // Hide cursor
			defer fmt.Print("\033[?25h") // Show cursor when done

			// Set terminal to raw mode
			setRawMode()
			defer resetRawMode()

			// Handle Ctrl+C
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigChan
				resetRawMode()
				fmt.Print("\033[?25h") // Show cursor
				os.Exit(0)
			}()

			// Print initial menu
			printMenu := func() {
				fmt.Print("\033[H\033[2J") // Clear screen
				fmt.Println("Choose a login method (use arrow keys and press Enter:")
				for i, opt := range options {
					if i == selectedIndex {
						fmt.Printf("> %s\n", opt)
					} else {
						fmt.Printf("  %s\n", opt)
					}
				}
			}

			printMenu()

			// Read keyboard input
			for {
				// Read a single byte without blocking
				var b [3]byte
				_, err := os.Stdin.Read(b[:])
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}

				if b[0] == 3 { // Ctrl+C
					resetRawMode()
					fmt.Print("\033[?25h") // Show cursor
					os.Exit(0)
				} else if b[0] == 27 && b[1] == 91 { // ESC [ sequence
					switch b[2] {
					case 65: // Up arrow
						if selectedIndex > 0 {
							selectedIndex--
							printMenu()
						}
					case 66: // Down arrow
						if selectedIndex < len(options)-1 {
							selectedIndex++
							printMenu()
						}
					}
				} else if b[0] == 10 { // Enter
					switch selectedIndex {
					case 0:
						r.DeviceModeLogin(workspace)
					case 1:
						r.ApiKeyLogin(workspace)
					}
					return
				}
			}
		},
	}
}

func setRawMode() {
	cmd := exec.Command("stty", "cbreak", "-echo")
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func resetRawMode() {
	cmd := exec.Command("stty", "-raw", "echo")
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

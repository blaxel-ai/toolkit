// Command preview runs the bl setup screens with simulated coding agents, to
// check their layout with many agents. It changes nothing on this machine.
//
//	go run ./cli/ui/preview -agents 60 -set-up 10 -fail 3
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/blaxel-ai/toolkit/cli/ui"
)

var knownAgents = []string{
	"Claude Code", "Codex", "Cursor", "Gemini CLI", "GitHub Copilot", "OpenCode", "Amp", "Cline", "Windsurf", "Goose",
	"Kiro CLI", "Roo Code", "Continue", "Augment", "Junie", "Trae", "Qwen Code", "OpenHands", "Pi", "Crush", "Devin", "OpenClaw",
}

func main() {
	agents := flag.Int("agents", 60, "simulated coding agents")
	setUp := flag.Int("set-up", 10, "how many of them an earlier setup already set up")
	failures := flag.Int("fail", 3, "how many of them fail to set up")
	yes := flag.Bool("yes", false, "install without showing the plan")
	loggedIn := flag.Bool("logged-in", false, "start logged in")
	flag.Parse()

	names := make([]string, *agents)
	for i := range names {
		if i < len(knownAgents) {
			names[i] = knownAgents[i]
		} else {
			names[i] = fmt.Sprintf("Agent %02d", i+1)
		}
	}
	random := rand.New(rand.NewSource(1))
	var mu sync.Mutex
	delay := func(low, high int) time.Duration {
		mu.Lock()
		defer mu.Unlock()
		return time.Duration(low+random.Intn(high-low)) * time.Millisecond
	}

	group := fmt.Sprintf("Coding agents · %d found", *agents)
	if *setUp > 0 {
		group = fmt.Sprintf("Coding agents · %d to set up", max(*agents-*setUp, 0))
	}
	var items []*ui.Item
	for i, name := range names {
		item := &ui.Item{ID: fmt.Sprint("agent:", i), Group: group, Label: name, Detail: "skills · MCP", On: true}
		switch {
		case i < *setUp:
			item.Done, item.On = "set up · skills · MCP", false
		case i%9 == 8:
			item.Detail = "skills"
		}
		items = append(items, item)
	}
	items = append(items,
		&ui.Item{ID: "skills", Group: "Blaxel", Label: "Agent skills", Detail: "teach your agents bl and the SDKs", On: true, Update: *setUp > 0},
		&ui.Item{ID: "mcp", Group: "Blaxel", Label: "Blaxel MCP", Detail: "your workspace, from your agents", On: true},
		&ui.Item{ID: "docs", Group: "Blaxel", Label: "Docs MCP", Detail: "search docs.blaxel.ai", On: true},
		&ui.Item{ID: "shell", Group: "This machine", Label: "Shell", Done: "bl on PATH · zsh completions"},
	)
	if *loggedIn {
		items = append(items, &ui.Item{ID: "account", Group: "This machine", Label: "Logged in", Done: "main"})
	} else {
		items = append(items, &ui.Item{ID: "login", Group: "This machine", Label: "Log in", Detail: "opens your browser", On: true})
	}
	items = append(items, &ui.Item{ID: "tracking", Group: "This machine", Label: "Error reports", Detail: "anonymous, helps us fix bugs faster", On: true})

	var failing sync.Map
	for i := 0; i < *failures; i++ {
		failing.Store(fmt.Sprint("agent:", *agents-1-i*3), true)
	}
	var workspace string
	if *loggedIn {
		workspace = "main"
	}
	setup := &ui.Setup{Subtitle: "v0.1.999-preview · simulated", Items: items}
	setup.Tasks = func(chosen map[string]bool) []ui.Task {
		var tasks []ui.Task
		if chosen["skills"] {
			tasks = append(tasks, ui.Task{ID: "skills", Label: "Agent skills", Run: func(ctx context.Context, c *ui.Control) (string, error) {
				c.Progress("downloading from GitHub")
				return "blaxel-cli, blaxel-sdk", sleep(ctx, delay(500, 900))
			}})
		}
		for i, name := range names {
			id := fmt.Sprint("agent:", i)
			if !chosen[id] || (!chosen["mcp"] && !chosen["docs"]) || i%9 == 8 {
				continue
			}
			tasks = append(tasks, ui.Task{ID: "mcp:" + id, Label: name, Group: "MCP servers", Run: func(ctx context.Context, c *ui.Control) (string, error) {
				c.Progress("adding the MCP servers")
				if err := sleep(ctx, delay(150, 2400)); err != nil {
					return "", err
				}
				if _, fail := failing.Load(id); fail {
					return "", errors.New("could not edit its config: not plain JSON")
				}
				return "added blaxel and blaxel-docs", nil
			}})
		}
		var others []string
		for _, task := range tasks {
			others = append(others, task.ID)
		}
		if _, offered := chosen["tracking"]; offered {
			tasks = append(tasks, ui.Task{ID: "tracking", Label: "Error reports", Run: func(context.Context, *ui.Control) (string, error) {
				if chosen["tracking"] {
					return "on · anonymous", nil
				}
				return "off", nil
			}})
		}
		if chosen["login"] {
			tasks = append(tasks, ui.Task{ID: "login", Label: "Log in", After: others, Skippable: true, Run: func(ctx context.Context, c *ui.Control) (string, error) {
				c.Progress("waiting for you in the browser")
				c.Note("Simulated: nothing opens. Esc skips, or wait to pick a workspace.")
				if err := sleep(ctx, 2500*time.Millisecond); err != nil {
					return "", err
				}
				options := []string{"main", "staging", "sandbox-lab"}
				index, err := c.Choose("Choose a workspace", options)
				if err != nil {
					return "", err
				}
				workspace = options[index]
				return "workspace " + workspace, nil
			}})
		}
		return tasks
	}
	setup.Summary = func(results map[string]ui.Result) ui.Summary {
		summary := ui.Summary{Group: "Installed into"}
		chosen := map[string]bool{}
		for _, item := range setup.Items {
			chosen[item.ID] = item.On
		}
		for i, name := range names {
			id := fmt.Sprint("agent:", i)
			line := ui.Line{Label: name, Group: "agents"}
			switch result, ran := results["mcp:"+id]; {
			case i < *setUp:
				line.Detail = "already set up · skills · Blaxel MCP · docs MCP"
			case !chosen[id]:
				continue
			case ran && result.Err != nil:
				line.Detail, line.Failed = result.Err.Error(), true
			case ran:
				line.Detail = "skills · Blaxel MCP · docs MCP"
			default:
				line.Detail = "skills"
			}
			summary.Lines = append(summary.Lines, line)
		}
		summary.Lines = append(summary.Lines, ui.Line{Label: "Shell", Detail: "bl on PATH · zsh completions"})
		account := []string{}
		if workspace != "" {
			account = append(account, "logged in to "+workspace)
		}
		if result, ok := results["tracking"]; ok {
			account = append(account, "error reports "+strings.TrimSuffix(result.Detail, " · anonymous"))
		}
		if len(account) > 0 {
			summary.Lines = append(summary.Lines, ui.Line{Label: "Blaxel", Detail: strings.Join(account, " · ")})
		}
		for _, line := range summary.Lines {
			if line.Failed {
				summary.Problems++
			}
		}
		summary.Title = "Blaxel is ready"
		if summary.Problems > 0 {
			summary.Title = fmt.Sprintf("Blaxel is set up, with %d problems", summary.Problems)
		}
		summary.Next = [][2]string{{"source ~/.zshrc", "use bl in this terminal"}, {"Restart your agents", `and ask: "Create a Blaxel sandbox"`}}
		if workspace == "" {
			summary.Next = append(summary.Next, [2]string{"bl login", "log in to Blaxel"})
		}
		return summary
	}

	_, err := setup.Run(context.Background(), ui.Options{Out: os.Stdout, Interactive: !*yes, Yes: *yes})
	if errors.Is(err, ui.ErrCancelled) {
		fmt.Println("  Preview cancelled.")
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

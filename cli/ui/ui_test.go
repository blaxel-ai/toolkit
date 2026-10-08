package ui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collect(t *testing.T, ctx context.Context, tasks []Task, skips *skipper) map[string]Result {
	t.Helper()
	events := make(chan event)
	go runTasks(ctx, tasks, events, skips)
	results := map[string]Result{}
	for e := range events {
		if e.kind == eventFinish {
			results[e.id] = e.result
		}
		if e.kind == eventChoose {
			e.reply <- 1
		}
		if e.kind == eventAllDone {
			return results
		}
	}
	return results
}

func TestRunTasksRunsInParallelAndInOrder(t *testing.T) {
	var running, peak atomic.Int32
	work := func(context.Context, *Control) (string, error) {
		now := running.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		running.Add(-1)
		return "done", nil
	}
	var lastStarted atomic.Bool
	results := collect(t, context.Background(), []Task{
		{ID: "a", Label: "A", Run: work},
		{ID: "b", Label: "B", Run: work},
		{ID: "last", Label: "Last", After: []string{"a", "b"}, Run: func(context.Context, *Control) (string, error) {
			lastStarted.Store(running.Load() == 0)
			return "after", nil
		}},
	}, nil)
	assert.Equal(t, int32(2), peak.Load(), "independent tasks run at the same time")
	assert.True(t, lastStarted.Load(), "a task waits for the tasks it comes after")
	assert.Equal(t, "after", results["last"].Detail)
	assert.Equal(t, "A", results["a"].Label)
}

func TestRunTasksSkipAndChoose(t *testing.T) {
	skips := &skipper{}
	started := make(chan struct{})
	go func() {
		<-started
		for !skips.any() {
			time.Sleep(time.Millisecond)
		}
		skips.skip()
	}()
	results := collect(t, context.Background(), []Task{
		{ID: "login", Skippable: true, Run: func(ctx context.Context, c *Control) (string, error) {
			close(started)
			<-ctx.Done()
			return "", ctx.Err()
		}},
		{ID: "pick", Run: func(_ context.Context, c *Control) (string, error) {
			index, err := c.Choose("Pick", []string{"a", "b"})
			return []string{"a", "b"}[index], err
		}},
		{ID: "fails", Run: func(context.Context, *Control) (string, error) { return "", errors.New("boom") }},
	}, skips)
	assert.ErrorIs(t, results["login"].Err, ErrSkipped)
	assert.Equal(t, "b", results["pick"].Detail)
	assert.EqualError(t, results["fails"].Err, "boom")

	// Stopping everything is not a skip.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results = collect(t, ctx, []Task{{ID: "x", Skippable: true, Run: func(ctx context.Context, _ *Control) (string, error) { return "", ctx.Err() }}}, skips)
	assert.ErrorIs(t, results["x"].Err, context.Canceled)
}

func TestPlainOutputAsksOnTheTerminal(t *testing.T) {
	setup := func(picked *string) *Setup {
		return &Setup{
			Tasks: func(map[string]bool) []Task {
				return []Task{{ID: "login", Label: "Log in", Run: func(_ context.Context, c *Control) (string, error) {
					index, err := c.Choose("Choose a workspace", []string{"main", "other"})
					if err != nil {
						return "", err
					}
					*picked = []string{"main", "other"}[index]
					return *picked, nil
				}}}
			},
			Summary: func(map[string]Result) Summary { return Summary{Title: "done"} },
		}
	}
	var out strings.Builder
	var picked string
	setup(&picked).runPlain(context.Background(), &out, bufio.NewReader(strings.NewReader("x\n2\n")))
	assert.Equal(t, "other", picked, "a wrong answer is asked again")
	assert.Contains(t, out.String(), "2  other")

	// Without a terminal, there is no one to ask.
	out.Reset()
	picked = ""
	setup(&picked).runPlain(context.Background(), &out, nil)
	assert.Empty(t, picked)
	assert.Contains(t, out.String(), "no choice is possible without a terminal")
}

func TestWrapBreaksLongWords(t *testing.T) {
	lines := wrap("Open ABCDEFGHIJKLMNOPQRSTABCDEFGHIJKLMNOPQRST now", 20, 80, false)
	for _, line := range lines {
		assert.LessOrEqual(t, lipgloss.Width(line), 20, line)
	}
	assert.Equal(t, "Open", lines[0])
	assert.Equal(t, "now", lines[len(lines)-1])
}

func TestWrapKeepsURLsWhole(t *testing.T) {
	const url = "https://app.blaxel.ai/device?code=ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	text := "Confirm the login in your browser. Not open? " + url

	// A URL longer than the column runs past it on a line of its own.
	assert.Equal(t, []string{"Confirm the login in your", "browser. Not open?", url}, wrap(text, 25, 80, false))
	assert.Equal(t, []string{"Open", url, "now"}, wrap("Open "+url+" now", 20, 80, false))

	// As a hyperlink, the text is still the whole URL.
	link := "\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"
	assert.Equal(t, []string{"Open", link, "now"}, wrap("Open "+url+" now", 20, 80, true))
	assert.Equal(t, len(url), lipgloss.Width(link))

	// Wider than the terminal, a URL breaks like any word, and is no hyperlink:
	// a link to a piece would open the wrong page.
	lines := wrap("Open "+url, 20, 30, true)
	assert.Equal(t, []string{"Open", url[:20], url[20:40], url[40:]}, lines)
}

func TestAppKeepsTheLoginLinkWhole(t *testing.T) {
	const url = "https://api.blaxel.ai/v0/login/device/finalize?user_code=ABCDEF"
	require.Equal(t, 63, len(url), "one more than the 62 cells of the column")
	ansi := regexp.MustCompile("\x1b\\[[0-9;]*m")
	for _, size := range [][2]int{{80, 24}, {110, 42}, {63, 24}} {
		for _, links := range []bool{false, true} {
			m := testModel(t, termenv.TrueColor)
			m.links = links
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.phase = running
			m.notes = []string{"Confirm the login in your browser. Not open? " + url}
			view := m.View()
			var found []string
			for _, line := range strings.Split(view, "\n") {
				assert.LessOrEqual(t, lipgloss.Width(line), size[0], "size %v: %q", size, line)
				if strings.Contains(line, "https://") {
					found = append(found, strings.TrimSpace(ansi.ReplaceAllString(line, "")))
				}
			}
			label := fmt.Sprintf("size %v links %v", size, links)
			if links {
				assert.Equal(t, []string{"\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"}, found, label)
			} else {
				assert.Equal(t, []string{url}, found, label)
				assert.NotContains(t, view, "\x1b]8", label)
			}
		}
	}

	// On a terminal narrower than the URL it breaks into pieces of the column,
	// without losing a character, and no hyperlink points at a piece.
	for _, columns := range []int{24, 30, 32, 40} {
		m := testModel(t, termenv.TrueColor)
		m.links = true
		m.Update(tea.WindowSizeMsg{Width: columns, Height: 24})
		m.phase = running
		m.notes = []string{"Open this page to log in: " + url}
		view := ansi.ReplaceAllString(m.View(), "")
		assert.NotContains(t, view, "\x1b]8")
		var pieces string
		for _, line := range strings.Split(view, "\n") {
			line = strings.TrimSpace(line)
			if pieces != "" && line == "" {
				break
			}
			if pieces != "" || strings.Contains(line, "https://") {
				assert.LessOrEqual(t, len(line), columns-2, "columns %d: %q", columns, line)
				pieces += line
			}
		}
		assert.Equal(t, url, pieces, "columns %d", columns)
	}
}

func testModel(t *testing.T, profile termenv.Profile) *model {
	t.Helper()
	renderer := lipgloss.NewRenderer(nil)
	renderer.SetColorProfile(profile)
	setup := &Setup{
		Subtitle: "v1.2.3 · macOS arm64",
		Items: []*Item{
			{ID: "agent:claude", Group: "Coding agents · 1 found", Label: "Claude Code", Detail: "skills · MCP", On: true},
			{ID: "skills", Group: "Blaxel", Label: "Agent skills", Detail: "update to the latest", On: true, Update: true},
			{ID: "shell", Group: "This machine", Label: "Shell", Done: "bl on PATH · zsh completions"},
		},
		Tasks: func(map[string]bool) []Task {
			return []Task{{ID: "skills", Label: "Agent skills", Run: func(context.Context, *Control) (string, error) { return "blaxel-cli", nil }}}
		},
		Summary: func(map[string]Result) Summary {
			return Summary{Title: "Blaxel is ready", Group: "Installed into",
				Lines: []Line{{Label: "Claude Code", Detail: strings.Repeat("very long detail ", 10)}},
				Next:  [][2]string{{"source ~/.zshrc", "use bl in this terminal"}}}
		},
	}
	m := &model{setup: setup, st: newStyles(renderer), pal: palette{profile: profile}, glyph: glyphsFor(true), results: map[string]Result{}}
	for _, item := range setup.Items {
		if item.Done == "" {
			m.toggles = append(m.toggles, item)
		}
	}
	m.cursor = len(m.toggles)
	m.ctx, m.cancel = context.WithCancel(context.Background())
	t.Cleanup(m.cancel)
	return m
}

func TestAppRendersAtEverySize(t *testing.T) {
	for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.ANSI, termenv.Ascii} {
		for _, size := range [][2]int{{20, 8}, {40, 16}, {80, 24}, {110, 42}, {120, 40}, {240, 70}} {
			m := testModel(t, profile)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			for _, phase := range []phase{planning, running} {
				m.phase = phase
				view := m.View()
				for _, line := range strings.Split(view, "\n") {
					assert.LessOrEqual(t, lipgloss.Width(line), max(size[0], 23), "profile %v size %v phase %v: %q", profile, size, phase, line)
				}
				if size[0] >= 80 && size[1] >= 24 {
					assert.Contains(t, view, "v1.2.3 · macOS arm64")
				}
			}
		}
	}
}

func TestAppQuitsAfterTasksWithoutAnotherKey(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {110, 42}} {
		for _, taskErr := range []error{nil, ErrSkipped, errors.New("failed"), context.Canceled} {
			m := testModel(t, termenv.Ascii)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.phase = running
			m.rows = []*row{{id: "skills", label: "Agent skills", state: rowRunning}}
			m.cancelled = errors.Is(taskErr, context.Canceled)
			result := Result{ID: "skills", Label: "Agent skills", Detail: "done", Err: taskErr}
			m.Update(eventMsg{kind: eventFinish, id: "skills", result: result})
			_, cmd := m.Update(eventMsg{kind: eventAllDone})
			require.NotNil(t, cmd)
			assert.IsType(t, tea.QuitMsg{}, cmd(), "completion quits without a key in every outcome")
			assert.Equal(t, finished, m.phase)
			assert.Equal(t, result, m.results["skills"])
			assert.Equal(t, m.setup.Summary(m.results), m.summary)
			assert.Empty(t, m.View(), "there is no final full-screen summary")
			for _, profile := range []termenv.Profile{termenv.TrueColor, termenv.Ascii} {
				renderer := lipgloss.NewRenderer(nil)
				renderer.SetColorProfile(profile)
				var out strings.Builder
				printSummary(&out, newStyles(renderer), m.glyph, m.summary, 2, true, size[0])
				assert.Equal(t, 1, strings.Count(out.String(), m.summary.Title))
				assert.NotContains(t, out.String(), "enter close")
				assert.Equal(t, profile != termenv.Ascii, strings.Contains(out.String(), "\x1b["))
			}
		}
	}
	// With no tasks (everything already done or deselected), completion also quits.
	m := testModel(t, termenv.Ascii)
	m.phase = running
	_, cmd := m.Update(eventMsg{kind: eventAllDone})
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}

func TestAppProgramExitsAfterAcceptingPlan(t *testing.T) {
	for _, taskErr := range []error{nil, ErrSkipped, errors.New("failed")} {
		m := testModel(t, termenv.Ascii)
		m.setup.Tasks = func(map[string]bool) []Task {
			return []Task{{ID: "skills", Label: "Agent skills", Run: func(context.Context, *Control) (string, error) {
				return "done", taskErr
			}}}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var out strings.Builder
		// Only the Enter accepting the plan is available. No completion key.
		final, err := tea.NewProgram(m, tea.WithInput(strings.NewReader("\r")), tea.WithOutput(&out),
			tea.WithoutRenderer(), tea.WithoutSignalHandler(), tea.WithContext(ctx)).Run()
		require.NoError(t, err)
		require.Equal(t, finished, final.(*model).phase)
		assert.Equal(t, taskErr, m.results["skills"].Err)
	}
}

func TestAppPlanKeys(t *testing.T) {
	m := testModel(t, termenv.Ascii)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	assert.Equal(t, "Install", m.action(), "a new agent is installed")
	// Up from the button reaches the last toggle; space turns it off.
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	assert.False(t, m.setup.Items[1].On)
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	assert.Equal(t, "Done", m.action(), "nothing selected")
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	assert.Equal(t, "Update", m.action(), "only refreshing what is installed")
	assert.Contains(t, m.View(), "Update")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, running, m.phase)
	assert.Len(t, m.rows, 2, "the shell row and the skills task")
}

func TestWordmarkProfiles(t *testing.T) {
	assert.Equal(t, []string{"B L A X E L"}, palette{profile: termenv.Ascii}.renderWordmark(80, -1))
	big := palette{profile: termenv.TrueColor}.renderWordmark(80, -1)
	assert.Len(t, big, 6)
	assert.Contains(t, big[0], "\x1b[38;2;239;65;54m", "the gradient starts at the logo red")
	small := palette{profile: termenv.ANSI256}.renderWordmark(30, 3)
	assert.Len(t, small, 2)
	assert.Contains(t, small[0], "\x1b[38;5;223m", "the sweep highlights a band")
	for _, line := range small {
		assert.Equal(t, wordmarkSmallWidth, lipgloss.Width(line))
	}
}

// manyAgents is a plan with n coding agents, the first done of them already set up.
func manyAgents(t *testing.T, n, done int) *model {
	t.Helper()
	m := testModel(t, termenv.TrueColor)
	var items []*Item
	for i := 0; i < n; i++ {
		item := &Item{ID: fmt.Sprint("agent:", i), Group: "Coding agents", Label: fmt.Sprintf("Agent %02d", i+1), Detail: "skills · MCP", On: true}
		if i < done {
			item.Done, item.On = "set up", false
		}
		items = append(items, item)
	}
	m.setup.Items = append(items, m.setup.Items...)
	m.setup.Tasks = func(chosen map[string]bool) []Task {
		var tasks []Task
		for i := 0; i < n; i++ {
			if chosen[fmt.Sprint("agent:", i)] {
				tasks = append(tasks, Task{ID: fmt.Sprint("mcp:", i), Label: fmt.Sprintf("Agent %02d", i+1), Group: "MCP servers",
					Run: func(context.Context, *Control) (string, error) { return "added", nil }})
			}
		}
		return tasks
	}
	m.toggles = nil
	for _, item := range m.setup.Items {
		if item.Done == "" {
			m.toggles = append(m.toggles, item)
		}
	}
	m.cursor = len(m.toggles)
	return m
}

var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// plain drops colors, so tests can look for text.
func plain(view string) string { return ansiCodes.ReplaceAllString(view, "") }

func assertFits(t *testing.T, view string, width, height int, context string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	assert.LessOrEqual(t, len(lines), height, context)
	for _, line := range lines {
		assert.LessOrEqual(t, lipgloss.Width(line), width, "%s: %q", context, line)
	}
}

func TestPlanWithManyAgentsFitsAndScrolls(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {110, 42}} {
		for _, n := range []int{22, 60} {
			m := manyAgents(t, n, 10)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			label := fmt.Sprintf("%d agents at %dx%d", n, size[0], size[1])
			view := m.View()
			assertFits(t, view, size[0], size[1], label)
			assert.Contains(t, plain(view), "Install", "%s: the button stays on screen", label)
			assert.Contains(t, plain(view), "enter install", label)
			// Walk the cursor through every item: it must always be on screen.
			for step := 0; step < len(m.toggles)+2; step++ {
				m.Update(tea.KeyMsg{Type: tea.KeyLeft})
				view = m.View()
				assertFits(t, view, size[0], size[1], label)
				if m.cursor < len(m.toggles) {
					assert.Contains(t, plain(view), "› ● "+m.toggles[m.cursor].Label, "%s: the cursor is visible on %s", label, m.toggles[m.cursor].Label)
				}
			}
		}
	}
}

func TestPlanGridMovesByRows(t *testing.T) {
	m := manyAgents(t, 22, 0)
	m.Update(tea.WindowSizeMsg{Width: 110, Height: 42})
	_, columns, _ := m.gridOf(m.toggles[0])
	require.Equal(t, 4, columns, "a wide terminal shows four columns")
	m.cursor = 0
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, columns, m.cursor, "down moves one row")
	m.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, columns+1, m.cursor, "right moves one item")
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, m.cursor)
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, len(m.toggles), m.cursor, "up from the first row reaches the Install button")
	// From the last row, down leaves the grid for the next group.
	m.cursor = 21
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, "agent:claude", m.toggles[m.cursor].ID, "the next group")
	assert.Contains(t, legend(m.setup.Items[:22]), "skills · MCP for each")
	assert.Equal(t, "skills · MCP for each new one · 10 already set up", legend(manyAgents(t, 22, 10).setup.Items[:22]))
}

func TestRunningCollapsesManyTasks(t *testing.T) {
	m := manyAgents(t, 60, 0)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.start()
	for i, r := range m.rows {
		if r.group == "" {
			continue
		}
		r.state, r.finished = rowDone, time.Now().Add(time.Duration(i)*time.Millisecond)
		if i%7 == 0 {
			r.state, r.detail = rowFailed, "boom"
		}
		if i > 50 {
			r.state = rowRunning
		}
	}
	rows, failures := m.displayRows()
	var collapsed *row
	for _, r := range rows {
		if r.label == "MCP servers" {
			collapsed = r
		}
	}
	require.NotNil(t, collapsed, "the 60 agent tasks collapse into one row")
	assert.Equal(t, rowRunning, collapsed.state)
	assert.Equal(t, "50 of 60 · Agent 50", collapsed.detail, "the count and the latest agent done")
	assert.Len(t, failures[collapsed], 7)
	view := m.View()
	assertFits(t, view, 80, 24, "running")
	assert.Contains(t, plain(view), "and 2 more failed")
}

func TestShellSummaryGroupsManyAgents(t *testing.T) {
	var lines []Line
	for i := 0; i < 60; i++ {
		line := Line{Label: fmt.Sprintf("Agent %02d", i+1), Group: "agents", Detail: "skills · MCP"}
		switch {
		case i < 10:
			line.Detail = "skills · MCP · already set up"
		case i%11 == 0:
			line.Detail, line.Failed = "could not edit it", true
		case i == 59:
			line.Detail = "skills"
		}
		lines = append(lines, line)
	}
	lines = append(lines, Line{Label: "Shell", Detail: "bl on PATH"})
	rows := summaryRows(lines, true, 40)
	var labels []string
	for _, r := range rows {
		labels = append(labels, r.label)
		for _, names := range r.names {
			assert.LessOrEqual(t, lipgloss.Width(names), 40, names)
		}
	}
	assert.Equal(t, []string{"10 agents", "44 agents", "Agent 60", "Agent 12", "Agent 23", "Agent 34", "Agent 45", "Agent 56", "Shell"}, labels)
	assert.Len(t, rows[1].names, 2, "names wrap to two lines")
	assert.Contains(t, rows[1].names[1], "more")
	assert.Len(t, summaryRows(lines, false, 40), 61, "logs keep every line")

	for _, size := range [][2]int{{80, 24}, {110, 42}} {
		m := manyAgents(t, 60, 10)
		var out strings.Builder
		summary := Summary{Title: "2 steps left: bl login, bl setup", Group: "Installed into", Lines: lines,
			Next: [][2]string{{"source ~/.zshrc", "use bl in this terminal"}, {"bl login", "log in"}}}
		printSummary(&out, m.st, m.glyph, summary, 2, true, size[0])
		assertFits(t, out.String(), size[0], size[1], "shell summary")
		assert.Contains(t, plain(out.String()), "44 agents")
		assert.NotContains(t, plain(out.String()), "enter close")
	}
}

func TestNameList(t *testing.T) {
	assert.Equal(t, []string{"A, B and C"}, []string{joinNames([]string{"A", "B", "C"})})
	names := []string{"Claude Code", "Codex", "Cursor", "Gemini CLI", "Agent 05", "Agent 06", "Agent 07", "Agent 08"}
	lines := nameList(names, 30, 2)
	assert.Len(t, lines, 2)
	assert.Equal(t, "Claude Code, Codex, Cursor,", lines[0])
	assert.Regexp(t, `\+\d+ more$`, lines[1])
	assert.Equal(t, []string{"Codex, Cursor"}, nameList([]string{"Codex", "Cursor"}, 30, 2))
}

func TestPlanOpensAtTheTop(t *testing.T) {
	m := manyAgents(t, 60, 10)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	view := plain(m.View())
	assert.Contains(t, view, "CODING AGENTS", "the plan opens at its first group")
	assert.Regexp(t, `↓ \d+ more`, view, "with a marker for what is below")
	assert.NotRegexp(t, `↑ \d+ more`, view, "and nothing above")
}

package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// Item is one line of the plan. Toggleable items start On or off; items
// with Done set are already in place and only shown.
type Item struct {
	ID, Group, Label, Detail string
	On                       bool
	// Update marks an item that refreshes something already installed.
	Update bool
	Done   string
}

// Task is one piece of work, shown as a row while setup runs. Tasks run
// concurrently once the tasks they come After have finished.
type Task struct {
	ID, Label string
	// Group collapses many similar tasks, such as one per agent, into one
	// progress row when there are more than a few.
	Group string
	After []string
	// Skippable tasks can be skipped with Esc while they run, such as a login.
	Skippable bool
	Run       func(ctx context.Context, c *Control) (detail string, err error)
}

// ErrSkipped is the result of a task the user skipped.
var ErrSkipped = errors.New("skipped")

// Result is how a task ended.
type Result struct {
	ID, Label, Detail string
	Err               error
	Elapsed           time.Duration
}

// Line is a row of the final screen. Many lines of the same Group (such as
// "agents") with the same Detail merge into one row that lists their labels.
type Line struct {
	Label, Detail, Group string
	Failed               bool
}

// Summary is the final screen: where Blaxel went and what to do next.
type Summary struct {
	Title    string
	Group    string
	Lines    []Line
	Next     [][2]string // command or action, and what it does
	Problems int
}

// Setup is a setup screen: a plan of items, the tasks for the chosen items,
// and the summary of their results.
type Setup struct {
	Subtitle string
	Items    []*Item
	// Tasks receives every item that can be toggled, mapped to whether it is on.
	Tasks   func(chosen map[string]bool) []Task
	Summary func(results map[string]Result) Summary
}

// Options control how a setup runs.
type Options struct {
	Out *os.File
	// Interactive shows the plan and waits for Enter at the end.
	Interactive bool
	// Yes accepts the plan without showing it and closes when done.
	Yes bool
}

// ErrCancelled is returned when the user leaves before installing.
var ErrCancelled = errors.New("setup cancelled")

// Run shows the setup on a color terminal, or prints it as plain lines.
func (s *Setup) Run(ctx context.Context, options Options) (Summary, error) {
	out := options.Out
	if out == nil {
		out = os.Stdout
	}
	if fancyTerminal(out) && (options.Interactive || options.Yes) {
		return s.runApp(ctx, out, options)
	}
	return s.runPlain(ctx, out), nil
}

func fancyTerminal(file *os.File) bool {
	return term.IsTerminal(int(file.Fd())) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
}

// chosen maps every item that can be toggled to whether it is on.
func (s *Setup) chosen() map[string]bool {
	chosen := map[string]bool{}
	for _, item := range s.Items {
		if item.Done == "" {
			chosen[item.ID] = item.On
		}
	}
	return chosen
}

// Control lets a running task report progress and ask the user to choose.
type Control struct {
	id     string
	events chan<- event
	ctx    context.Context
}

// Progress replaces the detail of the task's row.
func (c *Control) Progress(detail string) {
	c.send(event{kind: eventProgress, id: c.id, detail: detail})
}

// Note shows a line under the rows, such as a URL to open.
func (c *Control) Note(text string) { c.send(event{kind: eventNote, id: c.id, detail: text}) }

// Choose asks the user to pick one option. Plain output cannot ask, so it
// returns an error there.
func (c *Control) Choose(title string, options []string) (int, error) {
	reply := make(chan int, 1)
	c.send(event{kind: eventChoose, id: c.id, detail: title, options: options, reply: reply})
	select {
	case index := <-reply:
		if index < 0 {
			return 0, errors.New("no choice is possible without a terminal")
		}
		return index, nil
	case <-c.ctx.Done():
		return 0, c.ctx.Err()
	}
}

func (c *Control) send(e event) {
	select {
	case c.events <- e:
	case <-c.ctx.Done():
	}
}

type eventKind int

const (
	eventStart eventKind = iota
	eventProgress
	eventNote
	eventChoose
	eventFinish
	eventAllDone
)

type event struct {
	kind    eventKind
	id      string
	detail  string
	options []string
	reply   chan int
	result  Result
}

// runTasks runs the tasks, each as soon as the tasks it comes after finish,
// and reports everything on events. A task whose prerequisite failed still runs.
func runTasks(ctx context.Context, tasks []Task, events chan<- event, skips *skipper) {
	finished := map[string]chan struct{}{}
	for _, task := range tasks {
		finished[task.ID] = make(chan struct{})
	}
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func(task Task) {
			defer wg.Done()
			defer close(finished[task.ID])
			for _, id := range task.After {
				if done, ok := finished[id]; ok {
					select {
					case <-done:
					case <-ctx.Done():
					}
				}
			}
			taskCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			if task.Skippable && skips != nil {
				skips.set(task.ID, cancel)
				defer skips.set(task.ID, nil)
			}
			control := &Control{id: task.ID, events: events, ctx: ctx}
			// Starts and results are always delivered, even when stopping;
			// the reader drains events until eventAllDone.
			events <- event{kind: eventStart, id: task.ID}
			started := time.Now()
			detail, err := "", ctx.Err()
			if err == nil {
				control.ctx = taskCtx
				detail, err = task.Run(taskCtx, control)
				control.ctx = ctx
			}
			if err != nil && ctx.Err() == nil && taskCtx.Err() != nil {
				err = ErrSkipped
			}
			events <- event{kind: eventFinish, id: task.ID, result: Result{
				ID: task.ID, Label: task.Label, Detail: detail, Err: err, Elapsed: time.Since(started),
			}}
		}(task)
	}
	wg.Wait()
	events <- event{kind: eventAllDone}
}

// skipper cancels the running skippable tasks on request.
type skipper struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func (s *skipper) set(id string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancels == nil {
		s.cancels = map[string]context.CancelFunc{}
	}
	if cancel == nil {
		delete(s.cancels, id)
		return
	}
	s.cancels[id] = cancel
}

// skip cancels the running skippable tasks and reports whether there were any.
func (s *skipper) skip() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, cancel := range s.cancels {
		cancel()
	}
	return len(s.cancels) > 0
}

func (s *skipper) any() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.cancels) > 0
}

// runPlain prints the setup as plain lines, for logs, CI and coding agents.
func (s *Setup) runPlain(ctx context.Context, out io.Writer) Summary {
	st := newStyles(plainRenderer(out))
	glyph := glyphsFor(true)
	if s.Subtitle != "" {
		_, _ = fmt.Fprintln(out, "  Blaxel setup · "+s.Subtitle)
	}
	for _, item := range s.Items {
		if item.Done != "" {
			_, _ = fmt.Fprintln(out, plainLine(st, glyph.ok, item.Label, item.Done))
		}
	}
	events := make(chan event)
	tasks := s.Tasks(s.chosen())
	go runTasks(ctx, tasks, events, nil)
	results := map[string]Result{}
	for e := range events {
		switch e.kind {
		case eventNote:
			_, _ = fmt.Fprintln(out, "  "+e.detail)
		case eventChoose:
			e.reply <- -1
		case eventFinish:
			results[e.id] = e.result
			marker, detail := glyph.ok, e.result.Detail
			if e.result.Err != nil {
				marker, detail = glyph.fail, e.result.Err.Error()
			}
			_, _ = fmt.Fprintln(out, plainLine(st, marker, e.result.Label, detail))
		}
		if e.kind == eventAllDone {
			break
		}
	}
	summary := s.Summary(results)
	// Logs keep every line, for people and agents reading them later.
	printSummary(out, st, glyph, summary, 0, false, 120)
	return summary
}

// plainLine prints one step, lined up with the installer's own lines.
func plainLine(st styles, marker, label, detail string) string {
	return strings.TrimRight("  "+marker+" "+padRight(label, labelWidth)+" "+detail, " ")
}

// printSummary prints the final rows and next steps, indented by pad.
// compact merges large groups, as on the final screen.
func printSummary(out io.Writer, st styles, glyph glyphs, summary Summary, pad int, compact bool, width int) {
	margin := strings.Repeat(" ", pad)
	_, _ = fmt.Fprintln(out)
	_, _ = fmt.Fprintln(out, margin+st.accent.Render(glyph.spark)+" "+st.title.Render(summary.Title))
	for _, row := range summaryRows(summary.Lines, compact, width-pad-2-labelWidth-3) {
		marker := st.ok.Render(glyph.ok)
		label, detail := row.label, st.muted.Render(row.detail)
		switch {
		case row.failed:
			marker = st.fail.Render(glyph.fail)
		case row.more:
			marker, label, detail = " ", "", st.muted.Render(row.label)
		}
		_, _ = fmt.Fprintln(out, strings.TrimRight(margin+"  "+marker+" "+padRight(label, labelWidth)+" "+detail, " "))
		for _, names := range row.names {
			_, _ = fmt.Fprintln(out, margin+"  "+strings.Repeat(" ", labelWidth+3)+st.muted.Render(names))
		}
	}
	if len(summary.Next) == 0 {
		return
	}
	_, _ = fmt.Fprintln(out)
	commandWidth := 0
	for _, next := range summary.Next {
		commandWidth = max(commandWidth, lipgloss.Width(next[0]))
	}
	for _, next := range summary.Next {
		line := margin + "  " + st.accent.Render(glyph.pointer) + " " + st.key.Render(padRight(next[0], commandWidth))
		if next[1] != "" {
			line += "  " + st.muted.Render(next[1])
		}
		_, _ = fmt.Fprintln(out, strings.TrimRight(line, " "))
	}
}

// compactAbove is how many items a group can have before it becomes a grid
// in the plan, one progress row while running, and merged final rows.
const compactAbove = 6

// maxFailureRows bounds the failures listed one by one.
const maxFailureRows = 5

// summaryRow is a row of the final screen, possibly standing for many lines.
type summaryRow struct {
	label, detail string
	failed, more  bool
	names         []string // the labels a merged row stands for, wrapped
}

// summaryRows lays out the final lines. When compact, a group with more than
// compactAbove lines merges lines with the same detail into one row followed
// by their labels; failures stay one per row, up to maxFailureRows.
func summaryRows(lines []Line, compact bool, namesWidth int) []summaryRow {
	counts := map[string]int{}
	for _, line := range lines {
		if line.Group != "" {
			counts[line.Group]++
		}
	}
	var rows []summaryRow
	merged := map[string]bool{}
	for _, line := range lines {
		if !compact || line.Group == "" || counts[line.Group] <= compactAbove {
			rows = append(rows, summaryRow{label: line.Label, detail: line.Detail, failed: line.Failed})
			continue
		}
		if merged[line.Group] {
			continue
		}
		merged[line.Group] = true
		var details []string
		labels := map[string][]string{}
		var failures []Line
		for _, member := range lines {
			switch {
			case member.Group != line.Group:
			case member.Failed:
				failures = append(failures, member)
			default:
				if _, seen := labels[member.Detail]; !seen {
					details = append(details, member.Detail)
				}
				labels[member.Detail] = append(labels[member.Detail], member.Label)
			}
		}
		for _, detail := range details {
			names := labels[detail]
			if len(names) == 1 {
				rows = append(rows, summaryRow{label: names[0], detail: detail})
				continue
			}
			rows = append(rows, summaryRow{label: fmt.Sprintf("%d %s", len(names), line.Group), detail: detail,
				names: nameList(names, namesWidth, 2)})
		}
		for i, failure := range failures {
			if i == maxFailureRows {
				rows = append(rows, summaryRow{label: fmt.Sprintf("and %d more failed", len(failures)-i), more: true})
				break
			}
			rows = append(rows, summaryRow{label: failure.Label, detail: failure.Detail, failed: true})
		}
	}
	return rows
}

// nameList joins names into at most maxLines lines of width cells, ending
// with "+N more" when they do not all fit.
func nameList(names []string, width, maxLines int) []string {
	width = max(width, 16)
	var lines [][]string
	var current []string
	for i, name := range names {
		item := name
		if i < len(names)-1 {
			item += ","
		}
		if len(current) > 0 && lipgloss.Width(strings.Join(current, " ")+" "+item) > width {
			lines = append(lines, current)
			current = nil
		}
		current = append(current, item)
	}
	lines = append(lines, current)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		shown := 0
		for _, line := range lines {
			shown += len(line)
		}
		last := lines[maxLines-1]
		for {
			more := fmt.Sprintf("+%d more", len(names)-shown)
			if lipgloss.Width(strings.Join(last, " ")+" "+more) <= width || len(last) == 1 {
				lines[maxLines-1] = append(last, more)
				break
			}
			last, shown = last[:len(last)-1], shown-1
		}
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.Join(line, " ")
	}
	return out
}

func plainRenderer(out io.Writer) *lipgloss.Renderer {
	renderer := lipgloss.NewRenderer(out)
	renderer.SetColorProfile(termenv.Ascii)
	return renderer
}

// labelWidth aligns details in a second column.
const labelWidth = 14

type glyphs struct {
	ok, fail, warn, pending, pointer, on, off, spark, done, todo string
	frames                                                       []string
}

// glyphsFor returns the symbols for a terminal. The Linux text console lacks
// most of them.
func glyphsFor(unicode bool) glyphs {
	if !unicode {
		return glyphs{ok: "+", fail: "x", warn: "!", pending: "-", pointer: ">", on: "[x]", off: "[ ]", spark: "*", done: "#", todo: "-",
			frames: []string{"|", "/", "-", "\\"}}
	}
	return glyphs{ok: "✓", fail: "✗", warn: "!", pending: "○", pointer: "›", on: "●", off: "○", spark: "✦", done: "━", todo: "─",
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}}
}

func padRight(text string, width int) string {
	if gap := width - lipgloss.Width(text); gap > 0 {
		return text + strings.Repeat(" ", gap)
	}
	return text
}

func formatElapsed(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

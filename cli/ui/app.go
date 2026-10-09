package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type phase int

const (
	planning phase = iota
	running
	finished
)

// bodyWidth is the widest the centered column gets.
const bodyWidth = 62

type rowState int

const (
	rowPending rowState = iota
	rowRunning
	rowDone
	rowFailed
	rowSkipped
)

type row struct {
	id, label, group, detail string
	state                    rowState
	elapsed                  time.Duration
	finished                 time.Time
}

type tickMsg struct{}

type eventMsg event

type model struct {
	setup *Setup
	st    styles
	pal   palette
	glyph glyphs

	width, height int
	links         bool // link URLs with OSC 8; off on the Linux console
	phase         phase
	cursor        int // index in toggles; len(toggles) is the button
	toggles       []*Item
	rows          []*row
	notes         []string
	choice        *event
	choiceCursor  int
	frame         int
	offset        int // first line shown when the body scrolls
	events        chan event
	skips         skipper
	ctx           context.Context
	cancel        context.CancelFunc
	results       map[string]Result
	summary       Summary
	cancelled     bool
}

func (s *Setup) runApp(ctx context.Context, out *os.File, options Options) (Summary, error) {
	renderer := lipgloss.NewRenderer(out)
	// runApp only runs on a color terminal. The Linux console may not skip an
	// OSC 8 sequence it does not know, so it gets plain text.
	console := os.Getenv("TERM") == "linux"
	m := newModel(s, renderer, !console)
	m.links = !console
	m.ctx, m.cancel = context.WithCancel(ctx)
	defer m.cancel()
	if options.Yes {
		m.start()
	}
	program := tea.NewProgram(m, tea.WithOutput(out), tea.WithAltScreen(), tea.WithContext(ctx))
	final, err := program.Run()
	if err != nil && m.phase == planning {
		return Summary{}, err
	}
	if fm, ok := final.(*model); ok {
		m = fm
	}
	if m.phase == planning || (m.cancelled && m.phase != finished) {
		return Summary{}, ErrCancelled
	}
	// The alternate screen is gone; leave the summary in the scrollback.
	printSummary(out, m.st, m.glyph, m.summary, 2, true, max(m.width, 80))
	_, _ = fmt.Fprintln(out)
	return m.summary, nil
}

func newModel(s *Setup, renderer *lipgloss.Renderer, unicode bool) *model {
	m := &model{
		setup: s, st: newStyles(renderer), pal: palette{profile: renderer.ColorProfile()},
		glyph: glyphsFor(unicode), width: 80, height: 24, results: map[string]Result{},
	}
	for _, item := range s.Items {
		if item.Done == "" {
			m.toggles = append(m.toggles, item)
		}
	}
	m.cursor = len(m.toggles)
	return m
}

func (m *model) Init() tea.Cmd {
	if m.phase == running {
		return tea.Batch(m.tick(), m.waitForEvent())
	}
	return m.tick()
}

func (m *model) tick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *model) waitForEvent() tea.Cmd {
	return func() tea.Msg { return eventMsg(<-m.events) }
}

// start leaves the plan and runs the tasks for the chosen items.
func (m *model) start() {
	m.phase, m.offset = running, 0
	// What is already done shows as rows, except in grids, whose legend counts it.
	_, groups := groupItems(m.setup.Items)
	for _, item := range m.setup.Items {
		if item.Done != "" && len(groups[item.Group]) <= compactAbove {
			m.rows = append(m.rows, &row{id: item.ID, label: item.Label, detail: item.Done, state: rowDone})
		}
	}
	tasks := m.setup.Tasks(m.setup.chosen())
	for _, task := range tasks {
		m.rows = append(m.rows, &row{id: task.ID, label: task.Label, group: task.Group, detail: "waiting", state: rowPending})
	}
	m.events = make(chan event)
	go runTasks(m.ctx, tasks, m.events, &m.skips)
}

func (m *model) row(id string) *row {
	for _, r := range m.rows {
		if r.id == id {
			return r
		}
	}
	return &row{}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		m.frame++
		return m, m.tick()
	case eventMsg:
		return m.handleEvent(event(msg))
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *model) handleEvent(e event) (tea.Model, tea.Cmd) {
	switch e.kind {
	case eventStart:
		r := m.row(e.id)
		r.state, r.detail = rowRunning, ""
	case eventProgress:
		m.row(e.id).detail = e.detail
	case eventNote:
		m.notes = append(m.notes, e.detail)
	case eventChoose:
		m.choice, m.choiceCursor = &e, 0
	case eventFinish:
		r := m.row(e.id)
		r.elapsed, r.finished = e.result.Elapsed, time.Now()
		r.state, r.detail = rowDone, e.result.Detail
		switch {
		case errors.Is(e.result.Err, ErrSkipped):
			r.state, r.detail = rowSkipped, "skipped"
		case e.result.Err != nil:
			r.state, r.detail = rowFailed, e.result.Err.Error()
		}
		m.results[e.id] = e.result
	case eventAllDone:
		m.phase, m.offset = finished, 0
		m.notes = nil
		m.summary = m.setup.Summary(m.results)
		return m, tea.Quit
	}
	return m, m.waitForEvent()
}

func (m *model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.phase {
	case planning:
		switch key.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		case "up", "k", "down", "j", "left", "h", "right", "l", "tab", "shift+tab":
			m.move(key.String())
		case " ", "x":
			if m.cursor < len(m.toggles) {
				m.toggles[m.cursor].On = !m.toggles[m.cursor].On
			}
		case "enter":
			m.start()
			return m, m.waitForEvent()
		}
	case running:
		if m.choice != nil {
			switch key.String() {
			case "up", "k":
				m.choiceCursor = (m.choiceCursor + len(m.choice.options) - 1) % len(m.choice.options)
			case "down", "j":
				m.choiceCursor = (m.choiceCursor + 1) % len(m.choice.options)
			case "enter":
				m.choice.reply <- m.choiceCursor
				m.choice = nil
			}
		}
		switch key.String() {
		case "esc":
			m.skips.skip()
		case "ctrl+c":
			m.cancelled = true
			m.cancel()
		}
	}
	return m, nil
}

// move steps the plan cursor. In a grid, up and down move by rows and leave
// the grid at its edges; elsewhere every arrow steps through the items, and
// the Install button sits between the last item and the first.
func (m *model) move(key string) {
	if vim, ok := map[string]string{"k": "up", "j": "down", "h": "left", "l": "right"}[key]; ok {
		key = vim
	}
	n := len(m.toggles)
	if n == 0 {
		return
	}
	backward := key == "up" || key == "left" || key == "shift+tab"
	if m.cursor < n && (key == "up" || key == "down") {
		item := m.toggles[m.cursor]
		if group, columns, _ := m.gridOf(item); columns > 0 {
			cell := indexOf(group, item)
			step := columns
			if backward {
				step = -columns
			}
			for c := cell + step; c >= 0 && c < len(group); c += step {
				if i := m.toggleIndex(group[c]); i >= 0 {
					m.cursor = i
					return
				}
			}
			// Leave the grid past its first or last toggle.
			first, last := n, -1
			for _, member := range group {
				if i := m.toggleIndex(member); i >= 0 {
					first, last = min(first, i), max(last, i)
				}
			}
			if backward {
				m.cursor = (first + n) % (n + 1)
			} else {
				m.cursor = last + 1
			}
			return
		}
	}
	if backward {
		m.cursor = (m.cursor + n) % (n + 1)
	} else {
		m.cursor = (m.cursor + 1) % (n + 1)
	}
}

func (m *model) toggleIndex(item *Item) int {
	for i, toggle := range m.toggles {
		if toggle == item {
			return i
		}
	}
	return -1
}

func indexOf(items []*Item, item *Item) int {
	for i, candidate := range items {
		if candidate == item {
			return i
		}
	}
	return -1
}

// groupItems splits items into their groups, in order.
func groupItems(items []*Item) (titles []string, groups map[string][]*Item) {
	groups = map[string][]*Item{}
	for _, item := range items {
		if _, ok := groups[item.Group]; !ok {
			titles = append(titles, item.Group)
		}
		groups[item.Group] = append(groups[item.Group], item)
	}
	return titles, groups
}

// gridOf returns the group of an item and, when the group shows as a grid,
// its number of columns and the width of a cell.
func (m *model) gridOf(item *Item) ([]*Item, int, int) {
	_, groups := groupItems(m.setup.Items)
	group := groups[item.Group]
	columns, cell := gridColumns(group, m.columnWidth())
	return group, columns, cell
}

// gridColumns lays a large group out in up to four columns. Small groups,
// and groups that would get a single column, stay a list.
func gridColumns(items []*Item, width int) (columns, cell int) {
	if len(items) <= compactAbove {
		return 0, 0
	}
	label := 0
	for _, item := range items {
		label = max(label, lipgloss.Width(item.Label))
	}
	cell = label + 6 // pointer, marker, space, gap
	columns = min(width/cell, 4)
	if columns < 2 {
		return 0, 0
	}
	return columns, cell
}

func (m *model) columnWidth() int {
	return max(min(bodyWidth, m.width-2), 10)
}

func (m *model) View() string {
	// Completion goes straight to the shell summary, never another full screen.
	if m.phase == finished {
		return ""
	}
	width := m.columnWidth()
	body, anchor, pinned := m.screen(width)
	headers := [][]string{m.header(m.width - 2), m.header(wordmarkSmallWidth), nil}
	for _, header := range headers {
		view := m.compose(header, body, pinned, m.footer(m.width-2), width)
		if lipgloss.Height(view) <= m.height {
			return m.place(view)
		}
	}
	// Too tall even without the wordmark: scroll the body, keeping the
	// pinned lines and the keys in view.
	footer := m.footer(m.width - 2)
	room := m.height - len(pinned) - 3
	if pinned == nil {
		room += 1
	}
	if room < 3 {
		footer, room = "", room+2
	}
	return m.place(m.compose(nil, m.window(body, anchor, max(room, 1)), pinned, footer, width))
}

// screen is what the current phase shows: a body that may scroll, the body
// line to keep in view (-1 to scroll freely), and lines pinned under it.
func (m *model) screen(width int) (body []string, anchor int, pinned []string) {
	switch m.phase {
	case planning:
		// On the pinned button the list stays where it is (the top at first).
		body, anchor = m.planLines(width)
		button := m.st.buttonIdle.Render(m.action())
		if m.cursor == len(m.toggles) {
			button = m.st.button.Render(m.action())
		}
		return body, anchor, []string{center(button, width)}
	case running:
		body, anchor = m.runningLines(width)
		return body, anchor, []string{m.progressBar(width)}
	}
	return nil, -1, nil
}

// compose stacks the header, the column of body and pinned lines, and the
// keys. The column is as wide as its widest line, so it centers as a block.
func (m *model) compose(header, body, pinned []string, footer string, width int) string {
	lines := append([]string{}, body...)
	if len(pinned) > 0 {
		lines = append(append(lines, ""), pinned...)
	}
	// A line with a URL may run past the column, to the edge of the terminal.
	limit := func(line string) int {
		if hasURL(line) {
			return max(m.width, width)
		}
		return width
	}
	column := 0
	for _, line := range lines {
		column = max(column, min(lipgloss.Width(line), limit(line)))
	}
	for i, line := range lines {
		lines[i] = padRight(truncateStyled(line, min(column, limit(line))), column)
	}
	parts := header
	if len(parts) > 0 {
		parts = append(parts, "")
	}
	parts = append(parts, strings.Join(lines, "\n"))
	if footer != "" {
		parts = append(parts, "", footer)
	}
	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}

// place centers the view, and clips lines on terminals too small for it.
func (m *model) place(view string) string {
	vertical := lipgloss.Center
	if lipgloss.Height(view) >= m.height {
		vertical = lipgloss.Top
	}
	lines := strings.Split(view, "\n")
	if len(lines) > m.height && m.height > 0 {
		lines = lines[:m.height]
	}
	for i, line := range lines {
		lines[i] = truncateStyled(line, m.width)
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, vertical, strings.Join(lines, "\n"))
}

// window shows height lines of body around the anchor line, with markers
// for the lines above and below.
func (m *model) window(body []string, anchor, height int) []string {
	if len(body) <= height {
		m.offset = 0
		return body
	}
	visible := max(height-2, 1)
	if anchor >= 0 {
		if anchor < m.offset {
			m.offset = anchor
		} else if anchor >= m.offset+visible {
			m.offset = anchor - visible + 1
		}
	}
	m.offset = max(min(m.offset, len(body)-visible), 0)
	marker := func(count int, arrow string) string {
		if count == 0 {
			return ""
		}
		return m.st.muted.Render(fmt.Sprintf("%s %d more", arrow, count))
	}
	up, down := "↑", "↓"
	if m.glyph.ok == "+" {
		up, down = "^", "v"
	}
	lines := []string{marker(m.offset, up)}
	lines = append(lines, body[m.offset:m.offset+visible]...)
	return append(lines, marker(len(body)-m.offset-visible, down))
}

func (m *model) header(width int) []string {
	sweep := -1
	if m.phase == running {
		sweep = m.frame % 18
	}
	lines := m.pal.renderWordmark(width, sweep)
	return []string{strings.Join(lines, "\n"), "", m.st.muted.Render(m.setup.Subtitle)}
}

func (m *model) heading(title string) string {
	return m.st.muted.Bold(true).Render(strings.ToUpper(title))
}

// planLines is the plan, and the line of the cursor (-1 on the button).
func (m *model) planLines(width int) (lines []string, cursorLine int) {
	cursorLine = -1
	var current *Item
	if m.cursor < len(m.toggles) {
		current = m.toggles[m.cursor]
	}
	titles, groups := groupItems(m.setup.Items)
	for i, title := range titles {
		group := groups[title]
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, m.heading(title))
		if columns, cell := gridColumns(group, width); columns > 0 {
			for start := 0; start < len(group); start += columns {
				var cells []string
				for _, item := range group[start:min(start+columns, len(group))] {
					if item == current {
						cursorLine = len(lines)
					}
					cells = append(cells, padRight(m.itemCell(item, item == current), cell))
				}
				lines = append(lines, strings.TrimRight(strings.Join(cells, ""), " "))
			}
			for _, part := range wrap(legend(group), width-2, width-2, false) {
				lines = append(lines, "  "+m.st.muted.Render(part))
			}
			continue
		}
		for _, item := range group {
			if item == current {
				cursorLine = len(lines)
			}
			lines = append(lines, m.itemLine(item, item == current, width))
		}
	}
	return lines, cursorLine
}

// itemCell is an item in a grid: its marker and label.
func (m *model) itemCell(item *Item, focused bool) string {
	pointer := "  "
	if focused {
		pointer = m.st.accent.Render(m.glyph.pointer) + " "
	}
	marker, label := m.marker(item, focused)
	return pointer + marker + " " + label
}

// itemLine is an item in a list: its marker, label and detail.
func (m *model) itemLine(item *Item, focused bool, width int) string {
	if item.Done != "" {
		marker, label := m.marker(item, false)
		return "  " + marker + " " + padRight(label, labelWidth+lipgloss.Width(label)-lipgloss.Width(item.Label)) + " " + m.st.muted.Render(item.Done)
	}
	pointer := "  "
	if focused {
		pointer = m.st.accent.Render(m.glyph.pointer) + " "
	}
	marker, label := m.marker(item, focused)
	return pointer + marker + " " + padRight(label, labelWidth) + " " + m.st.muted.Render(truncate(item.Detail, width-labelWidth-5))
}

func (m *model) marker(item *Item, focused bool) (marker, label string) {
	switch {
	case item.Done != "":
		return m.st.ok.Render(m.glyph.ok), m.st.muted.Render(item.Label)
	case focused:
		marker = m.st.muted.Render(m.glyph.off)
		if item.On {
			marker = m.st.accent.Render(m.glyph.on)
		}
		return marker, m.st.title.Render(item.Label)
	case item.On:
		return m.st.accent.Render(m.glyph.on), item.Label
	}
	return m.st.muted.Render(m.glyph.off), m.st.muted.Render(item.Label)
}

// legend says what the items of a grid get, since cells have no room for it.
func legend(items []*Item) string {
	var details []string
	labels := map[string][]string{}
	done := 0
	for _, item := range items {
		if item.Done != "" {
			done++
			continue
		}
		if _, seen := labels[item.Detail]; !seen {
			details = append(details, item.Detail)
		}
		labels[item.Detail] = append(labels[item.Detail], item.Label)
	}
	sort.SliceStable(details, func(i, j int) bool { return len(labels[details[i]]) > len(labels[details[j]]) })
	var parts []string
	for i, detail := range details {
		switch {
		case len(details) == 1 && done == 0:
			parts = append(parts, detail+" for each")
		case len(details) == 1:
			parts = append(parts, detail+" for each new one")
		case i == 0:
			parts = append(parts, detail+" for most")
		default:
			parts = append(parts, detail+" for "+shortList(labels[detail], 3))
		}
	}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("%d already set up", done))
	}
	return strings.Join(parts, " · ")
}

// shortList names up to limit items, then how many more.
func shortList(names []string, limit int) string {
	if len(names) <= limit {
		return joinNames(names)
	}
	return strings.Join(names[:limit], ", ") + fmt.Sprintf(" and %d more", len(names)-limit)
}

func joinNames(names []string) string {
	if len(names) <= 1 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// action names what Enter does: install anything new, update what is
// already installed, or finish when nothing is selected.
func (m *model) action() string {
	action := "Done"
	for _, item := range m.toggles {
		switch {
		case item.On && !item.Update:
			return "Install"
		case item.On:
			action = "Update"
		}
	}
	return action
}

// displayRows are the rows while running: tasks of a large group collapse
// into one progress row, with their failures listed after it.
func (m *model) displayRows() (rows []*row, failures map[*row][]*row) {
	counts := map[string]int{}
	for _, r := range m.rows {
		if r.group != "" {
			counts[r.group]++
		}
	}
	failures = map[*row][]*row{}
	collapsed := map[string]bool{}
	for _, r := range m.rows {
		if r.group == "" || counts[r.group] <= compactAbove {
			rows = append(rows, r)
			continue
		}
		if collapsed[r.group] {
			continue
		}
		collapsed[r.group] = true
		summary, failed := m.collapse(r.group)
		rows = append(rows, summary)
		failures[summary] = failed
	}
	return rows, failures
}

// collapse sums up the tasks of a group in one row.
func (m *model) collapse(group string) (*row, []*row) {
	total, done, started := 0, 0, false
	var latest *row
	var failed []*row
	var elapsed time.Duration
	for _, r := range m.rows {
		if r.group != group {
			continue
		}
		total++
		switch r.state {
		case rowRunning:
			started = true
		case rowDone, rowFailed, rowSkipped:
			done, started = done+1, true
			elapsed = max(elapsed, r.elapsed)
			if latest == nil || r.finished.After(latest.finished) {
				latest = r
			}
			if r.state == rowFailed {
				failed = append(failed, r)
			}
		}
	}
	summary := &row{label: group, state: rowPending, detail: "waiting"}
	switch {
	case done < total && started:
		summary.state, summary.detail = rowRunning, fmt.Sprintf("%d of %d", done, total)
		if latest != nil {
			summary.detail += " · " + latest.label
		}
	case done == total && len(failed) > 0:
		summary.state, summary.elapsed = rowFailed, elapsed
		summary.detail = fmt.Sprintf("%d of %d · %d failed", total-len(failed), total, len(failed))
	case done == total:
		summary.state, summary.elapsed, summary.detail = rowDone, elapsed, fmt.Sprintf("all %d", total)
	}
	return summary, failed
}

// runningLines are the rows, notes and any choice, and the line to keep in view.
func (m *model) runningLines(width int) (lines []string, anchor int) {
	rows, failures := m.displayRows()
	for _, r := range rows {
		lines = append(lines, m.rowLine(r, width))
		for i, failure := range failures[r] {
			if i == maxFailureRows {
				lines = append(lines, "  "+m.st.muted.Render(fmt.Sprintf("  and %d more failed", len(failures[r])-i)))
				break
			}
			lines = append(lines, "  "+m.rowLine(failure, width-2))
		}
	}
	anchor = -1
	if m.choice != nil {
		lines = append(lines, "", m.st.title.Render(m.choice.detail))
		for i, option := range m.choice.options {
			if i == m.choiceCursor {
				anchor = len(lines)
				lines = append(lines, m.st.accent.Render(m.glyph.pointer)+" "+m.st.title.Render(option))
			} else {
				lines = append(lines, "  "+option)
			}
		}
	}
	for _, note := range m.notes {
		lines = append(lines, "")
		for _, part := range wrap(note, width, m.width, m.links) {
			lines = append(lines, m.st.muted.Render(part))
		}
	}
	return lines, anchor
}

func (m *model) rowLine(r *row, width int) string {
	var marker string
	detail := m.st.muted.Render(r.detail)
	switch r.state {
	case rowPending:
		marker = m.st.rail.Render(m.glyph.pending)
		detail = m.st.rail.Render(r.detail)
	case rowRunning:
		marker = m.st.accent.Render(m.glyph.frames[m.frame%len(m.glyph.frames)])
		if r.detail == "" {
			detail = m.st.muted.Render("working…")
		}
	case rowDone:
		marker = m.st.ok.Render(m.glyph.ok)
	case rowFailed:
		marker = m.st.fail.Render(m.glyph.fail)
		detail = m.st.fail.Render(r.detail)
	case rowSkipped:
		marker = m.st.muted.Render(m.glyph.pending)
	}
	elapsed := ""
	if r.elapsed > 0 {
		elapsed = formatElapsed(r.elapsed)
	}
	room := width - labelWidth - 3 - len(elapsed) - 1
	line := marker + " " + padRight(r.label, labelWidth) + " " + truncateStyled(detail, room)
	if elapsed != "" {
		gap := width - lipgloss.Width(line) - len(elapsed)
		line += strings.Repeat(" ", max(gap, 1)) + m.st.rail.Render(elapsed)
	}
	return line
}

func (m *model) progressBar(width int) string {
	done := 0
	for _, r := range m.rows {
		if r.state >= rowDone {
			done++
		}
	}
	total := len(m.rows)
	count := fmt.Sprintf(" %d/%d", done, total)
	room := max(width-len(count), 1)
	filled := 0
	if total > 0 {
		filled = room * done / total
	}
	return m.st.accent.Render(strings.Repeat(m.glyph.done, filled)) + m.st.rail.Render(strings.Repeat(m.glyph.todo, room-filled)) + m.st.muted.Render(count)
}

// footer shows the keys of the current screen, dropping the least important
// ones when the terminal is narrow.
func (m *model) footer(width int) string {
	key := func(k, what string) string { return m.st.key.Render(k) + " " + m.st.muted.Render(what) }
	var keys []string
	switch m.phase {
	case planning:
		keys = []string{key("enter", strings.ToLower(m.action())), key("space", "toggle"), key("↑↓", "move"), key("esc", "quit")}
	case running:
		switch {
		case m.choice != nil:
			keys = []string{key("enter", "choose"), key("↑↓", "move")}
		case m.skips.any():
			keys = []string{key("esc", "skip"), key("ctrl+c", "stop")}
		default:
			keys = []string{key("ctrl+c", "stop")}
		}
	}
	gap := m.st.rail.Render("  ·  ")
	for len(keys) > 1 && lipgloss.Width(strings.Join(keys, gap)) > width {
		keys = keys[:len(keys)-1]
	}
	return strings.Join(keys, gap)
}

func center(text string, width int) string {
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, text)
}

func truncate(text string, width int) string {
	if width <= 1 || lipgloss.Width(text) <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 && lipgloss.Width(string(runes))+1 > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// truncateStyled shortens styled text to width cells.
func truncateStyled(text string, width int) string {
	if width <= 1 || lipgloss.Width(text) <= width {
		return text
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(text)
}

// wrap breaks text into lines of at most width cells at spaces, and breaks
// words longer than a line. A URL that fits in room cells, the width of the
// terminal, is kept whole so that it stays clickable, and one wider than that
// breaks like any word. A whole URL is a hyperlink when links is set, with the
// URL as its text, so that a terminal that ignores hyperlinks still shows it.
func wrap(text string, width, room int, links bool) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		limit := width
		if isURL(word) && lipgloss.Width(word) <= max(room, width) {
			limit = max(room, width)
			if links {
				word = "\x1b]8;;" + word + "\x1b\\" + word + "\x1b]8;;\x1b\\"
			}
		}
		for limit > 0 && lipgloss.Width(word) > limit {
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			runes := []rune(word)
			lines = append(lines, string(runes[:limit]))
			word = string(runes[limit:])
		}
		switch {
		case line == "":
			line = word
		case lipgloss.Width(line)+1+lipgloss.Width(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	return append(lines, line)
}

func isURL(word string) bool {
	return strings.HasPrefix(word, "https://") || strings.HasPrefix(word, "http://")
}

func hasURL(line string) bool {
	return strings.Contains(line, "https://") || strings.Contains(line, "http://")
}

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/edwinupegui/arsenal/internal/calendar"
)

// calendarViewState selects which sub-view the Calendar area renders.
type calendarViewState int

const (
	calendarStateList calendarViewState = iota
	calendarStateDetail
	calendarStateTrash
	calendarStateConfirmDelete
	calendarStateConfirmPurge
)

// calendarItem adapts a *calendar.Event to bubbles/list's Item interface.
type calendarItem struct {
	e *calendar.Event
}

func newCalendarItem(e *calendar.Event) calendarItem { return calendarItem{e: e} }

// Title returns the event title.
func (i calendarItem) Title() string {
	return i.e.Row.Title
}

// Description returns formatted start_at + location + recurrence + tag count.
func (i calendarItem) Description() string {
	parts := []string{formatCalendarStartAt(i.e.Row.StartAt, i.e.Row.AllDay == 1)}
	if i.e.Row.Location != "" {
		parts = append(parts, i.e.Row.Location)
	}
	if i.e.Row.Recurrence != "none" {
		parts = append(parts, i.e.Row.Recurrence)
	}
	if len(i.e.Tags) > 0 {
		parts = append(parts, fmt.Sprintf("%d tag(s)", len(i.e.Tags)))
	}
	return strings.Join(parts, " · ")
}

// FilterValue exposes fields searched by the built-in bubbles fuzzy filter.
func (i calendarItem) FilterValue() string {
	return strings.Join([]string{
		i.e.Row.Title,
		i.e.Row.StartAt,
		i.e.Row.Location,
		i.e.Row.Recurrence,
		strings.Join(i.e.Tags, " "),
	}, " ")
}

// formatCalendarStartAt formats start_at for list display. All-day events show
// just the date; timed events show the time portion (HH:MM).
func formatCalendarStartAt(startAt string, allDay bool) string {
	if allDay {
		// Date-only: YYYY-MM-DD — return as-is
		if len(startAt) >= 10 {
			return startAt[:10]
		}
		return startAt
	}
	// Timed: YYYY-MM-DDTHH:MM:SS — extract HH:MM
	if len(startAt) >= 16 {
		return startAt[11:16]
	}
	return startAt
}

// formatCalendarTimeRange builds "HH:MM–HH:MM" or "HH:MM" when end is empty.
func formatCalendarTimeRange(startAt string, endAt string, allDay bool) string {
	if allDay {
		return "All day"
	}
	start := ""
	if len(startAt) >= 16 {
		start = startAt[11:16]
	} else {
		start = startAt
	}
	if endAt == "" {
		return start
	}
	end := ""
	if len(endAt) >= 16 {
		end = endAt[11:16]
	} else {
		end = endAt
	}
	return start + "–" + end
}

// asCalendarItems converts a slice of *calendar.Event to []list.Item.
func asCalendarItems(rows []*calendar.Event) []list.Item {
	out := make([]list.Item, 0, len(rows))
	for _, e := range rows {
		out = append(out, newCalendarItem(e))
	}
	return out
}

// --- calendarDetailModel ------------------------------------------------------

// calendarDetailModel renders a single event's full info in a scrollable viewport.
type calendarDetailModel struct {
	vp        viewport.Model
	event     *calendar.Event
	hasItem   bool
	width     int
	maxHeight int
}

func newCalendarDetailModel() calendarDetailModel {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	return calendarDetailModel{vp: vp}
}

// SetEvent replaces the displayed event.
func (m *calendarDetailModel) SetEvent(e *calendar.Event) {
	m.event = e
	m.hasItem = true
	m.vp.SetContent(m.renderBody())
	m.vp.GotoTop()
}

// SetSize keeps the viewport in step with terminal resizes.
func (m *calendarDetailModel) SetSize(width, height int) {
	m.width = width
	m.maxHeight = height
	m.vp.Width = width
	m.vp.Height = height
	if m.hasItem {
		m.vp.SetContent(m.renderBody())
	}
}

// Update forwards key/scroll events to the viewport.
func (m calendarDetailModel) Update(msg tea.Msg) (calendarDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the detail panel.
func (m calendarDetailModel) View() string {
	if !m.hasItem {
		return mutedStyle.Render("no event selected")
	}
	return m.vp.View()
}

func (m calendarDetailModel) renderBody() string {
	e := m.event
	var b strings.Builder

	row := func(label, value string) {
		fmt.Fprintf(&b, "%s %s\n", detailLabelStyle.Render(label+":"), detailValueStyle.Render(value))
	}

	row("ID", fmt.Sprintf("%d", e.Row.ID))
	row("Title", e.Row.Title)

	// Time display: All day / timed range / open-ended
	endAt := ""
	if e.Row.EndAt.Valid {
		endAt = e.Row.EndAt.String
	}
	timeDisplay := formatCalendarTimeRange(e.Row.StartAt, endAt, e.Row.AllDay == 1)
	row("Time", timeDisplay)
	row("Start", e.Row.StartAt)

	if e.Row.EndAt.Valid && e.Row.EndAt.String != "" {
		row("End", e.Row.EndAt.String)
	} else {
		row("End", "—")
	}

	row("Recurrence", e.Row.Recurrence)

	if e.Row.Location != "" {
		row("Location", e.Row.Location)
	}

	if e.Row.Description.Valid && e.Row.Description.String != "" {
		row("Description", e.Row.Description.String)
	}

	if e.Row.CategoryID.Valid {
		row("Category ID", fmt.Sprintf("%d", e.Row.CategoryID.Int64))
	} else {
		row("Category", "—")
	}

	if len(e.Tags) > 0 {
		row("Tags", "#"+strings.Join(e.Tags, " #"))
	} else {
		row("Tags", "—")
	}

	row("Created", e.Row.CreatedAt)
	row("Updated", e.Row.UpdatedAt)

	if e.Row.DeletedAt.Valid {
		row("Trashed at", e.Row.DeletedAt.String)
	}

	if e.Row.Notes.Valid && e.Row.Notes.String != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, detailLabelStyle.Render("Notes"))
		fmt.Fprintln(&b, indentLines(e.Row.Notes.String, "  "))
	}

	return b.String()
}

// --- updateCalendar -----------------------------------------------------------

// updateCalendar is the top-level Update dispatcher for areaCalendar.
func (a App) updateCalendar(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		listH := msg.Height - 2
		if listH < 5 {
			listH = 5
		}
		a.calendarList.SetSize(msg.Width, listH)
		a.calendarDetail.SetSize(msg.Width, listH)
		return a, nil

	case calendarLoadedMsg:
		if msg.err != nil {
			a.statusErr = msg.err
			return a, nil
		}
		a.calendarList.SetItems(asCalendarItems(msg.items))
		a.statusErr = nil
		title := "Calendar"
		if a.calendarShowTrashed {
			title = "Calendar — Trash"
		}
		a.calendarList.Title = title
		return a, nil

	case calendarMutatedMsg:
		if msg.err != nil {
			a.statusErr = msg.err
		} else {
			a.statusErr = nil
			a.statusMsg = msg.status
		}
		return a, loadCalendarCmd(a.calendarService, a.calendarShowTrashed)
	}

	switch a.calendarState {
	case calendarStateDetail:
		return a.updateCalendarDetail(msg)
	case calendarStateConfirmDelete:
		return a.updateCalendarConfirmDelete(msg)
	case calendarStateConfirmPurge:
		return a.updateCalendarConfirmPurge(msg)
	default:
		return a.updateCalendarList(msg)
	}
}

// updateCalendarList handles key events while the list sub-view is active.
func (a App) updateCalendarList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && a.calendarList.FilterState() != list.Filtering {
		switch {
		case key.Matches(km, a.keys.Quit):
			return a, tea.Quit

		case key.Matches(km, a.keys.Detail):
			if it, ok := a.selectedCalendarItem(); ok {
				a.calendarDetail.SetEvent(it.e)
				a.calendarState = calendarStateDetail
				return a, nil
			}

		case key.Matches(km, a.keys.Trash):
			a.calendarShowTrashed = !a.calendarShowTrashed
			a.statusMsg = ""
			return a, loadCalendarCmd(a.calendarService, a.calendarShowTrashed)

		case key.Matches(km, a.keys.SoftDelete):
			if a.calendarShowTrashed {
				return a, nil
			}
			if it, ok := a.selectedCalendarItem(); ok {
				a.calendarConfirm = newConfirmModel(
					fmt.Sprintf("Move %q to trash?", it.e.Row.Title),
					confirmActionDelete, it.e.Row.ID, it.e.Row.Title,
				)
				a.calendarState = calendarStateConfirmDelete
				return a, nil
			}

		case key.Matches(km, a.keys.Restore):
			if !a.calendarShowTrashed {
				return a, nil
			}
			if it, ok := a.selectedCalendarItem(); ok {
				return a, restoreCalendarCmd(a.calendarService, it.e)
			}

		case km.String() == "x":
			if !a.calendarShowTrashed {
				return a, nil
			}
			if it, ok := a.selectedCalendarItem(); ok {
				a.calendarConfirm = newConfirmModel(
					fmt.Sprintf("Permanently delete event %q?", it.e.Row.Title),
					confirmActionDelete, it.e.Row.ID, it.e.Row.Title,
				)
				a.calendarState = calendarStateConfirmPurge
				return a, nil
			}
		}
	}

	var cmd tea.Cmd
	a.calendarList, cmd = a.calendarList.Update(msg)
	return a, cmd
}

// updateCalendarDetail handles key events while the detail sub-view is active.
func (a App) updateCalendarDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, a.keys.Quit), key.Matches(km, a.keys.Back):
			a.calendarState = calendarStateList
			return a, nil
		}
	}
	var cmd tea.Cmd
	a.calendarDetail, cmd = a.calendarDetail.Update(msg)
	return a, cmd
}

// updateCalendarConfirmDelete handles the soft-delete confirmation prompt.
func (a App) updateCalendarConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			id := a.calendarConfirm.id
			title := a.calendarConfirm.title
			a.calendarState = calendarStateList
			return a, softDeleteCalendarCmd(a.calendarService, id, title)
		case "n", "N", "esc":
			a.calendarState = calendarStateList
			a.statusMsg = "canceled"
			return a, nil
		}
	}
	return a, nil
}

// updateCalendarConfirmPurge handles the hard-delete (purge) confirmation prompt.
func (a App) updateCalendarConfirmPurge(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			id := a.calendarConfirm.id
			a.calendarState = calendarStateList
			return a, purgeCalendarCmd(a.calendarService, id)
		case "n", "N", "esc":
			a.calendarState = calendarStateList
			a.statusMsg = "canceled"
			return a, nil
		}
	}
	return a, nil
}

// selectedCalendarItem returns the currently highlighted list item.
func (a App) selectedCalendarItem() (calendarItem, bool) {
	cur := a.calendarList.SelectedItem()
	if cur == nil {
		return calendarItem{}, false
	}
	it, ok := cur.(calendarItem)
	return it, ok
}

// --- calendar view rendering --------------------------------------------------

// viewCalendar composes the active calendar sub-view.
func (a App) viewCalendar() string {
	switch a.calendarState {
	case calendarStateDetail:
		return a.calendarDetail.View()
	case calendarStateConfirmDelete, calendarStateConfirmPurge:
		return a.calendarConfirm.view(a.width, a.height)
	default:
		header := ""
		if a.calendarShowTrashed {
			header = trashBannerStyle.Render(" TRASH ") + "\n"
		}
		return header + a.calendarList.View()
	}
}

// calendarStatusHints returns the key-hint portion of the status bar for the
// Calendar area.
func (a App) calendarStatusHints() string {
	parts := []string{
		keyStyle.Render("Calendar"),
		keyStyle.Render("tab") + " next",
		keyStyle.Render("shift+tab") + " prev",
		keyStyle.Render("1-5") + " jump",
		keyStyle.Render("n") + " new",
		keyStyle.Render("e") + " edit",
		keyStyle.Render("d") + " del",
		keyStyle.Render("Tab") + " switch",
	}
	if a.calendarShowTrashed {
		parts = append(parts,
			keyStyle.Render("r")+" restore",
			keyStyle.Render("x")+" purge",
		)
	}
	return mutedStyle.Render(strings.Join(parts, "  "))
}

// --- commands -----------------------------------------------------------------

// calendarLoadedMsg arrives after a (re)load of the calendar list completes.
type calendarLoadedMsg struct {
	items []*calendar.Event
	err   error
}

// calendarMutatedMsg fires after a mutation (delete, restore, purge).
type calendarMutatedMsg struct {
	status string
	err    error
}

// loadCalendarCmd queries the calendar list (active or trashed) and returns it
// as a calendarLoadedMsg. Runs off the UI goroutine.
func loadCalendarCmd(svc *calendar.Service, trashed bool) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return calendarLoadedMsg{}
		}
		items, err := svc.List(context.Background(), calendar.Filter{
			Trashed: trashed,
			Limit:   500,
		})
		return calendarLoadedMsg{items: items, err: err}
	}
}

func softDeleteCalendarCmd(svc *calendar.Service, id int64, title string) tea.Cmd {
	return func() tea.Msg {
		if err := svc.SoftDelete(context.Background(), id); err != nil {
			return calendarMutatedMsg{err: err}
		}
		return calendarMutatedMsg{status: fmt.Sprintf("moved to trash: %s", title)}
	}
}

func restoreCalendarCmd(svc *calendar.Service, e *calendar.Event) tea.Cmd {
	return func() tea.Msg {
		if err := svc.Restore(context.Background(), e.Row.ID); err != nil {
			return calendarMutatedMsg{err: err}
		}
		return calendarMutatedMsg{status: fmt.Sprintf("restored: %s", e.Row.Title)}
	}
}

func purgeCalendarCmd(svc *calendar.Service, id int64) tea.Cmd {
	return func() tea.Msg {
		if err := svc.Purge(context.Background(), id); err != nil {
			return calendarMutatedMsg{err: err}
		}
		return calendarMutatedMsg{status: "purged"}
	}
}

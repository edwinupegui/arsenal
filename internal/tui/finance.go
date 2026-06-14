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

	"github.com/edwinupegui/arsenal/internal/finance"
)

// financeViewState selects which sub-view the Finance area renders.
type financeViewState int

const (
	financeStateList financeViewState = iota
	financeStateDetail
	financeStateTrash
	financeStateConfirmDelete
	financeStateConfirmPurge
)

// financeItem adapts a *finance.Transaction to bubbles/list's Item interface.
type financeItem struct {
	t *finance.Transaction
}

func newFinanceItem(t *finance.Transaction) financeItem { return financeItem{t: t} }

// Title returns the account name and formatted amount (with sign for kind).
func (i financeItem) Title() string {
	sign := "-"
	if i.t.Row.Kind == "income" {
		sign = "+"
	}
	return fmt.Sprintf("%s %s%.2f %s", i.t.Row.Account, sign, i.t.Row.Amount, i.t.Row.Currency)
}

// Description returns kind, date, and tags on one line.
func (i financeItem) Description() string {
	parts := []string{i.t.Row.Kind, i.t.Row.Date}
	if len(i.t.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(i.t.Tags, " #"))
	}
	return strings.Join(parts, " · ")
}

// FilterValue exposes fields searched by the built-in bubbles fuzzy filter.
func (i financeItem) FilterValue() string {
	notes := ""
	if i.t.Row.Notes.Valid {
		notes = i.t.Row.Notes.String
	}
	return strings.Join([]string{
		i.t.Row.Account,
		i.t.Row.Kind,
		i.t.Row.Date,
		notes,
		strings.Join(i.t.Tags, " "),
	}, " ")
}

// asFinanceItems converts a slice of *finance.Transaction to []list.Item.
func asFinanceItems(rows []*finance.Transaction) []list.Item {
	out := make([]list.Item, 0, len(rows))
	for _, t := range rows {
		out = append(out, newFinanceItem(t))
	}
	return out
}

// --- financeDetailModel -----------------------------------------------------

// financeDetailModel renders a single transaction's full info in a scrollable viewport.
type financeDetailModel struct {
	vp          viewport.Model
	transaction *finance.Transaction
	hasItem     bool
	width       int
	maxHeight   int
}

func newFinanceDetailModel() financeDetailModel {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	return financeDetailModel{vp: vp}
}

// SetTransaction replaces the displayed transaction.
func (m *financeDetailModel) SetTransaction(t *finance.Transaction) {
	m.transaction = t
	m.hasItem = true
	m.vp.SetContent(m.renderBody())
	m.vp.GotoTop()
}

// SetSize keeps the viewport in step with terminal resizes.
func (m *financeDetailModel) SetSize(width, height int) {
	m.width = width
	m.maxHeight = height
	m.vp.Width = width
	m.vp.Height = height
	if m.hasItem {
		m.vp.SetContent(m.renderBody())
	}
}

// Update forwards key/scroll events to the viewport.
func (m financeDetailModel) Update(msg tea.Msg) (financeDetailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the detail panel.
func (m financeDetailModel) View() string {
	if !m.hasItem {
		return mutedStyle.Render("no transaction selected")
	}
	return m.vp.View()
}

func (m financeDetailModel) renderBody() string {
	t := m.transaction
	var b strings.Builder

	row := func(label, value string) {
		fmt.Fprintf(&b, "%s %s\n", detailLabelStyle.Render(label+":"), detailValueStyle.Render(value))
	}

	row("ID", fmt.Sprintf("%d", t.Row.ID))
	row("Date", t.Row.Date)
	row("Amount", fmt.Sprintf("%.2f", t.Row.Amount))
	row("Kind", t.Row.Kind)
	row("Account", t.Row.Account)
	row("Currency", t.Row.Currency)
	row("Recurrence", t.Row.Recurrence)

	if t.Row.CategoryID.Valid {
		row("Category ID", fmt.Sprintf("%d", t.Row.CategoryID.Int64))
	} else {
		row("Category", "—")
	}

	if len(t.Tags) > 0 {
		row("Tags", "#"+strings.Join(t.Tags, " #"))
	} else {
		row("Tags", "—")
	}

	row("Created", t.Row.CreatedAt)
	row("Updated", t.Row.UpdatedAt)

	if t.Row.DeletedAt.Valid {
		row("Trashed at", t.Row.DeletedAt.String)
	}

	if t.Row.Notes.Valid && t.Row.Notes.String != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, detailLabelStyle.Render("Notes"))
		fmt.Fprintln(&b, indentLines(t.Row.Notes.String, "  "))
	}

	return b.String()
}

// --- updateFinance ----------------------------------------------------------

// updateFinance is the top-level Update dispatcher for areaFinance. It handles
// common messages (window resize, data load, mutations) and then delegates to
// the active sub-state handler.
func (a App) updateFinance(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		listH := msg.Height - 2
		if listH < 5 {
			listH = 5
		}
		a.financeList.SetSize(msg.Width, listH)
		a.financeDetail.SetSize(msg.Width, listH)
		return a, nil

	case financeLoadedMsg:
		if msg.err != nil {
			a.statusErr = msg.err
			return a, nil
		}
		a.financeList.SetItems(asFinanceItems(msg.items))
		a.statusErr = nil
		title := "Finance"
		if a.financeShowTrashed {
			title = "Finance — Trash"
		}
		a.financeList.Title = title
		return a, nil

	case financeMutatedMsg:
		if msg.err != nil {
			a.statusErr = msg.err
		} else {
			a.statusErr = nil
			a.statusMsg = msg.status
		}
		return a, loadFinanceCmd(a.financeService, a.financeShowTrashed)
	}

	switch a.financeState {
	case financeStateDetail:
		return a.updateFinanceDetail(msg)
	case financeStateConfirmDelete:
		return a.updateFinanceConfirmDelete(msg)
	case financeStateConfirmPurge:
		return a.updateFinanceConfirmPurge(msg)
	default:
		return a.updateFinanceList(msg)
	}
}

// updateFinanceList handles key events while the list sub-view is active.
func (a App) updateFinanceList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok && a.financeList.FilterState() != list.Filtering {
		switch {
		case key.Matches(km, a.keys.Quit):
			return a, tea.Quit

		case key.Matches(km, a.keys.Detail):
			if it, ok := a.selectedFinanceItem(); ok {
				a.financeDetail.SetTransaction(it.t)
				a.financeState = financeStateDetail
				return a, nil
			}

		case key.Matches(km, a.keys.Trash):
			a.financeShowTrashed = !a.financeShowTrashed
			a.statusMsg = ""
			return a, loadFinanceCmd(a.financeService, a.financeShowTrashed)

		case key.Matches(km, a.keys.SoftDelete):
			if a.financeShowTrashed {
				return a, nil
			}
			if it, ok := a.selectedFinanceItem(); ok {
				a.financeConfirm = newConfirmModel(
					fmt.Sprintf("Move %q to trash?", it.t.Row.Account),
					confirmActionDelete, it.t.Row.ID, it.t.Row.Account,
				)
				a.financeState = financeStateConfirmDelete
				return a, nil
			}

		case key.Matches(km, a.keys.Restore):
			if !a.financeShowTrashed {
				return a, nil
			}
			if it, ok := a.selectedFinanceItem(); ok {
				return a, restoreFinanceCmd(a.financeService, it.t)
			}

		case km.String() == "x":
			if !a.financeShowTrashed {
				return a, nil
			}
			if it, ok := a.selectedFinanceItem(); ok {
				a.financeConfirm = newConfirmModel(
					fmt.Sprintf("Permanently delete transaction %q?", it.t.Row.Account),
					confirmActionDelete, it.t.Row.ID, it.t.Row.Account,
				)
				a.financeState = financeStateConfirmPurge
				return a, nil
			}
		}
	}

	var cmd tea.Cmd
	a.financeList, cmd = a.financeList.Update(msg)
	return a, cmd
}

// updateFinanceDetail handles key events while the detail sub-view is active.
func (a App) updateFinanceDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(km, a.keys.Quit), key.Matches(km, a.keys.Back):
			a.financeState = financeStateList
			return a, nil
		}
	}
	var cmd tea.Cmd
	a.financeDetail, cmd = a.financeDetail.Update(msg)
	return a, cmd
}

// updateFinanceConfirmDelete handles the soft-delete confirmation prompt.
func (a App) updateFinanceConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			id := a.financeConfirm.id
			title := a.financeConfirm.title
			a.financeState = financeStateList
			return a, softDeleteFinanceCmd(a.financeService, id, title)
		case "n", "N", "esc":
			a.financeState = financeStateList
			a.statusMsg = "canceled"
			return a, nil
		}
	}
	return a, nil
}

// updateFinanceConfirmPurge handles the hard-delete (purge) confirmation prompt.
func (a App) updateFinanceConfirmPurge(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			id := a.financeConfirm.id
			a.financeState = financeStateList
			return a, purgeFinanceCmd(a.financeService, id)
		case "n", "N", "esc":
			a.financeState = financeStateList
			a.statusMsg = "canceled"
			return a, nil
		}
	}
	return a, nil
}

// selectedFinanceItem returns the currently highlighted list item.
func (a App) selectedFinanceItem() (financeItem, bool) {
	cur := a.financeList.SelectedItem()
	if cur == nil {
		return financeItem{}, false
	}
	it, ok := cur.(financeItem)
	return it, ok
}

// --- finance view rendering -------------------------------------------------

// viewFinance composes the active finance sub-view.
func (a App) viewFinance() string {
	switch a.financeState {
	case financeStateDetail:
		return a.financeDetail.View()
	case financeStateConfirmDelete, financeStateConfirmPurge:
		return a.financeConfirm.view(a.width, a.height)
	default:
		header := ""
		if a.financeShowTrashed {
			header = trashBannerStyle.Render(" TRASH ") + "\n"
		}
		return header + a.financeList.View()
	}
}

// financeStatusHints returns the key-hint portion of the status bar for the
// Finance area.
func (a App) financeStatusHints() string {
	parts := []string{
		keyStyle.Render("Finance"),
		keyStyle.Render("tab") + " next",
		keyStyle.Render("shift+tab") + " prev",
		keyStyle.Render("1-5") + " jump",
		keyStyle.Render("n") + " new",
		keyStyle.Render("e") + " edit",
		keyStyle.Render("d") + " del",
		keyStyle.Render("r") + " restore",
	}
	if a.financeShowTrashed {
		parts = append(parts, keyStyle.Render("x")+" purge")
	}
	return mutedStyle.Render(strings.Join(parts, "  "))
}

// --- commands ---------------------------------------------------------------

// financeLoadedMsg arrives after a (re)load of the finance list completes.
type financeLoadedMsg struct {
	items []*finance.Transaction
	err   error
}

// financeMutatedMsg fires after a mutation (delete, restore, purge).
type financeMutatedMsg struct {
	status string
	err    error
}

// loadFinanceCmd queries the finance list (active or trashed) and returns it
// as a financeLoadedMsg. Runs off the UI goroutine.
func loadFinanceCmd(svc *finance.Service, trashed bool) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return financeLoadedMsg{}
		}
		items, err := svc.List(context.Background(), finance.Filter{
			Trashed: trashed,
			Limit:   500,
		})
		return financeLoadedMsg{items: items, err: err}
	}
}

func softDeleteFinanceCmd(svc *finance.Service, id int64, account string) tea.Cmd {
	return func() tea.Msg {
		if err := svc.SoftDelete(context.Background(), id); err != nil {
			return financeMutatedMsg{err: err}
		}
		return financeMutatedMsg{status: fmt.Sprintf("moved to trash: %s", account)}
	}
}

func restoreFinanceCmd(svc *finance.Service, t *finance.Transaction) tea.Cmd {
	return func() tea.Msg {
		if err := svc.Restore(context.Background(), t.Row.ID); err != nil {
			return financeMutatedMsg{err: err}
		}
		return financeMutatedMsg{status: fmt.Sprintf("restored: %s", t.Row.Account)}
	}
}

func purgeFinanceCmd(svc *finance.Service, id int64) tea.Cmd {
	return func() tea.Msg {
		if err := svc.Purge(context.Background(), id); err != nil {
			return financeMutatedMsg{err: err}
		}
		return financeMutatedMsg{status: "purged"}
	}
}

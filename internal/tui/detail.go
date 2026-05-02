package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/edwinupegui/arsenal/internal/store"
)

// detailModel renders a single resource's full info in a scrollable viewport.
// It is stateless beyond the viewport: every time the parent activates it,
// SetResource is called with the row to display.
type detailModel struct {
	vp        viewport.Model
	res       store.ListedResource
	hasRes    bool
	width     int
	maxHeight int
}

func newDetailModel() detailModel {
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	return detailModel{vp: vp}
}

// SetResource replaces what the view shows.
func (m *detailModel) SetResource(r store.ListedResource) {
	m.res = r
	m.hasRes = true
	m.vp.SetContent(m.renderBody())
	m.vp.GotoTop()
}

// SetSize keeps the viewport in step with terminal resizes.
func (m *detailModel) SetSize(width, height int) {
	m.width = width
	m.maxHeight = height
	m.vp.Width = width
	m.vp.Height = height
	if m.hasRes {
		m.vp.SetContent(m.renderBody())
	}
}

// Update forwards key/scroll events to the viewport.
func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the detail panel; the parent composes header/footer around it.
func (m detailModel) View() string {
	if !m.hasRes {
		return mutedStyle.Render("no resource selected")
	}
	return m.vp.View()
}

func (m detailModel) renderBody() string {
	r := m.res
	var b strings.Builder

	row := func(label, value string) {
		fmt.Fprintf(&b, "%s %s\n", detailLabelStyle.Render(label+":"), detailValueStyle.Render(value))
	}

	row("ID", fmt.Sprintf("%d", r.Resource.ID))
	row("Title", r.Resource.Title)
	row("URL", r.Resource.Url)
	row("Type", r.Resource.Type)
	row("Language", r.Resource.Language)
	if r.Resource.Favorite == 1 {
		row("Favorite", "★ yes")
	} else {
		row("Favorite", "no")
	}
	if r.CategoryName.Valid {
		slug := ""
		if r.CategorySlug.Valid {
			slug = " (" + r.CategorySlug.String + ")"
		}
		row("Category", r.CategoryName.String+slug)
	} else {
		row("Category", "—")
	}
	if len(r.Tags) > 0 {
		row("Tags", "#"+strings.Join(r.Tags, " #"))
	} else {
		row("Tags", "—")
	}
	row("Created", r.Resource.CreatedAt)
	row("Updated", r.Resource.UpdatedAt)
	if r.Resource.DeletedAt.Valid {
		row("Trashed at", r.Resource.DeletedAt.String)
	}
	if r.Resource.Description.Valid && r.Resource.Description.String != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, detailLabelStyle.Render("Description"))
		fmt.Fprintln(&b, indentLines(r.Resource.Description.String, "  "))
	}
	if r.Resource.Notes.Valid && r.Resource.Notes.String != "" {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, detailLabelStyle.Render("Notes"))
		fmt.Fprintln(&b, indentLines(r.Resource.Notes.String, "  "))
	}
	return b.String()
}

func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

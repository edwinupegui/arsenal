package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/edwinupegui/arsenal/internal/finance"
	"github.com/edwinupegui/arsenal/internal/migrations"
	"github.com/edwinupegui/arsenal/internal/store"
)

// newFinanceTestDB opens a temp SQLite DB, runs migrations, and returns a
// *finance.Service backed by it.
func newFinanceTestDB(t *testing.T) (*finance.Service, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "arsenal.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := store.Migrate(db, migrations.FS, "."); err != nil {
		_ = db.Close()
		t.Fatalf("migrate: %v", err)
	}
	svc := finance.New(db)
	return svc, func() { _ = db.Close() }
}

// seedFinanceTransaction creates a transaction in the DB and returns it.
func seedFinanceTransaction(t *testing.T, svc *finance.Service, account string) *finance.Transaction {
	t.Helper()
	tx, err := svc.Create(t.Context(), finance.CreateInput{
		Date:    "2026-06-10",
		Amount:  1500.00,
		Kind:    finance.KindExpense,
		Account: account,
	})
	if err != nil {
		t.Fatalf("create finance transaction: %v", err)
	}
	return tx
}

// --- Scenario: Finance area renders transaction list -------------------------

func TestFinanceAreaRendersTransactionList(t *testing.T) {
	svc, cleanup := newFinanceTestDB(t)
	defer cleanup()

	seedFinanceTransaction(t, svc, "banco nación")

	app := New(nil)
	app.financeService = svc
	app.currentArea = areaFinance

	// Set terminal size.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	// Load finance data.
	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	view := app.View()
	if !strings.Contains(view, "banco nación") {
		t.Errorf("finance list should show 'banco nación', got:\n%s", view)
	}
}

// --- Scenario: Placeholder message no longer appears ------------------------

func TestFinancePlaceholderGone(t *testing.T) {
	app := New(nil)
	app.currentArea = areaFinance

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	view := app.View()
	if strings.Contains(view, "coming soon") {
		t.Errorf("finance placeholder should be gone, got:\n%s", view)
	}
}

// --- Scenario: Tab cycles to Finance ----------------------------------------

func TestTabCyclesToFinance(t *testing.T) {
	app := App{currentArea: areaTodos, keys: defaultKeys()}
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got := model.(App).currentArea; got != areaFinance {
		t.Errorf("Tab from Todos = %d, want %d (areaFinance)", got, areaFinance)
	}
}

// --- Scenario: j/k navigate -------------------------------------------------

func TestFinanceJKeyMovesDown(t *testing.T) {
	svc, cleanup := newFinanceTestDB(t)
	defer cleanup()

	seedFinanceTransaction(t, svc, "banco nación")
	seedFinanceTransaction(t, svc, "banco colombia")

	app := New(nil)
	app.financeService = svc
	app.currentArea = areaFinance

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	initialIdx := app.financeList.Index()

	// Press j to move down.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	app = model.(App)

	if app.financeList.Index() <= initialIdx {
		t.Errorf("j key should move selection down: was %d, now %d", initialIdx, app.financeList.Index())
	}
}

// --- Scenario: enter opens detail -------------------------------------------

func TestFinanceEnterOpensDetail(t *testing.T) {
	svc, cleanup := newFinanceTestDB(t)
	defer cleanup()

	seedFinanceTransaction(t, svc, "banco nación")

	app := New(nil)
	app.financeService = svc
	app.currentArea = areaFinance

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	app.financeList.Select(0)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(App)

	if app.financeState != financeStateDetail {
		t.Errorf("enter should switch to financeStateDetail, got %d", app.financeState)
	}
}

// --- Scenario: detail view shows all fields ---------------------------------

func TestFinanceDetailViewShowsAllFields(t *testing.T) {
	svc, cleanup := newFinanceTestDB(t)
	defer cleanup()

	tx, err := svc.Create(t.Context(), finance.CreateInput{
		Date:    "2026-06-10",
		Amount:  1500.00,
		Kind:    finance.KindExpense,
		Account: "banco nación",
		Notes:   "supermercado",
		Tags:    []string{"comida"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	app := New(nil)
	app.financeService = svc
	app.currentArea = areaFinance
	app.financeState = financeStateDetail

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	app.financeDetail.SetTransaction(tx)

	view := app.View()
	if !strings.Contains(view, "banco nación") {
		t.Errorf("detail should show account 'banco nación', got:\n%s", view)
	}
	if !strings.Contains(view, "1500") {
		t.Errorf("detail should show amount '1500', got:\n%s", view)
	}
	if !strings.Contains(view, "expense") {
		t.Errorf("detail should show kind 'expense', got:\n%s", view)
	}
	if !strings.Contains(view, "comida") {
		t.Errorf("detail should show tag 'comida', got:\n%s", view)
	}
}

// --- Scenario: d key triggers soft-delete confirm ---------------------------

func TestFinanceSoftDeleteKey(t *testing.T) {
	svc, cleanup := newFinanceTestDB(t)
	defer cleanup()

	seedFinanceTransaction(t, svc, "banco nación")

	app := New(nil)
	app.financeService = svc
	app.currentArea = areaFinance

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	app.financeList.Select(0)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	app = model.(App)

	if app.financeState != financeStateConfirmDelete {
		t.Errorf("d key should enter financeStateConfirmDelete, got %d", app.financeState)
	}
}

// --- Scenario: r key in trash view restores ---------------------------------

func TestFinanceRestoreKeyInTrashView(t *testing.T) {
	svc, cleanup := newFinanceTestDB(t)
	defer cleanup()

	tx := seedFinanceTransaction(t, svc, "banco nación")
	if err := svc.SoftDelete(t.Context(), tx.Row.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	app := New(nil)
	app.financeService = svc
	app.currentArea = areaFinance
	app.financeShowTrashed = true

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	app.financeList.Select(0)

	model, cmd = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	app = model.(App)

	// r in trashed view should emit a restore command (not nil).
	if cmd == nil {
		t.Errorf("r in trash view should return a restore command, got nil")
	}
}

// --- Scenario: x key in trash view triggers purge confirm -------------------

func TestFinancePurgeKeyInTrashView(t *testing.T) {
	svc, cleanup := newFinanceTestDB(t)
	defer cleanup()

	tx := seedFinanceTransaction(t, svc, "banco nación")
	if err := svc.SoftDelete(t.Context(), tx.Row.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	app := New(nil)
	app.financeService = svc
	app.currentArea = areaFinance
	app.financeShowTrashed = true

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	cmd := app.loadCurrentAreaCmd()
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	app.financeList.Select(0)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	app = model.(App)

	if app.financeState != financeStateConfirmPurge {
		t.Errorf("x in trash view should enter financeStateConfirmPurge, got %d", app.financeState)
	}
}

// --- Scenario: status bar shows Finance hints -------------------------------

func TestFinanceStatusBarHints(t *testing.T) {
	app := New(nil)
	app.currentArea = areaFinance

	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	app = model.(App)

	view := app.View()
	if !strings.Contains(view, "Finance") {
		t.Errorf("status bar should show 'Finance', got:\n%s", view)
	}
}

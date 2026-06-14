package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"

	"github.com/edwinupegui/arsenal/internal/finance"
	"github.com/edwinupegui/arsenal/internal/migrations"
)

// financeTestHomes caches per-test ARSENAL_HOME dirs so multiple
// runFinanceExec calls within the same test share the same SQLite database.
var (
	financeTestHomesMu sync.Mutex
	financeTestHomes   = map[string]string{}
)

// financeTestHome returns a stable temp directory for t, creating it once
// per test name and registering cleanup on first creation.
func financeTestHome(t *testing.T) string {
	t.Helper()
	key := t.Name()
	financeTestHomesMu.Lock()
	defer financeTestHomesMu.Unlock()
	if h, ok := financeTestHomes[key]; ok {
		return h
	}
	dir, err := os.MkdirTemp("", "arsenal-finance-test-*")
	if err != nil {
		t.Fatalf("create test home: %v", err)
	}
	t.Cleanup(func() {
		financeTestHomesMu.Lock()
		delete(financeTestHomes, key)
		financeTestHomesMu.Unlock()
		_ = os.RemoveAll(dir)
	})
	h := filepath.Join(dir, "home")
	financeTestHomes[key] = h
	return h
}

// runFinanceExec builds a fresh `arsenal finance` subcommand tree, points
// ARSENAL_HOME at a stable per-test directory, wires the embedded migrations,
// and executes args. Multiple calls within the same test share the same DB.
func runFinanceExec(t *testing.T, args ...string) (string, error) {
	t.Helper()
	migrationsFS = migrations.FS
	t.Setenv("ARSENAL_HOME", financeTestHome(t))

	root := &cobra.Command{Use: "arsenal"}
	root.AddCommand(newFinanceCmd())

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"finance"}, args...))

	err := root.Execute()
	return buf.String(), err
}

func TestFinanceCmd_HelpListsSubcommands(t *testing.T) {
	// The finance parent command has no RunE, so cobra prints help and returns
	// nil. The spec requires the help to list all subcommands.
	out, _ := runFinanceExec(t)
	for _, sub := range []string{"add", "list", "show", "edit", "rm", "restore", "purge", "export"} {
		if !strings.Contains(out, sub) {
			t.Errorf("help output missing subcommand %q:\n%s", sub, out)
		}
	}
}

func TestFinanceAdd_AllFlags(t *testing.T) {
	out, err := runFinanceExec(t,
		"add",
		"--date", "2026-06-13",
		"--amount", "42.50",
		"--kind", "expense",
		"--account", "checking",
		"--notes", "lunch",
		"--recurrence", "weekly",
	)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("expected 'created' in output, got:\n%s", out)
	}
}

func TestFinanceAdd_MissingAmount(t *testing.T) {
	_, err := runFinanceExec(t, "add", "--kind", "expense")
	if err == nil {
		t.Fatal("expected error for missing amount")
	}
}

func TestFinanceAdd_InvalidKind(t *testing.T) {
	out, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "transfer")
	if err == nil {
		t.Fatal("expected error for invalid kind")
	}
	if !strings.Contains(out, "kind must be expense or income") {
		t.Errorf("expected kind validation error, got:\n%s", out)
	}
}

func TestFinanceAdd_JSON(t *testing.T) {
	out, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "income", "--json")
	if err != nil {
		t.Fatalf("add --json: %v\n%s", err, out)
	}
	var tx map[string]any
	if err := json.Unmarshal([]byte(out), &tx); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
}

func TestFinanceList_WithFilter(t *testing.T) {
	if _, err := runFinanceExec(t, "add", "--date", "2026-06-10", "--amount", "10", "--kind", "expense"); err != nil {
		t.Fatalf("seed add: %v", err)
	}
	if _, err := runFinanceExec(t, "add", "--date", "2026-06-11", "--amount", "20", "--kind", "income"); err != nil {
		t.Fatalf("seed add: %v", err)
	}

	out, err := runFinanceExec(t, "list", "--from", "2026-06-01", "--to", "2026-06-30", "--kind", "income")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "income") {
		t.Errorf("expected income transaction in output:\n%s", out)
	}
}

func TestFinanceList_JSON(t *testing.T) {
	if _, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "expense"); err != nil {
		t.Fatalf("seed add: %v", err)
	}

	out, err := runFinanceExec(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v\n%s", err, out)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("output is not valid JSON array: %v\n%s", err, out)
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(arr))
	}
}

func TestFinanceShow(t *testing.T) {
	out, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "expense", "--account", "a")
	if err != nil {
		t.Fatalf("seed add: %v\n%s", err, out)
	}
	// Extract the created id from the text output.
	id := extractID(t, out)

	out, err = runFinanceExec(t, "show", id)
	if err != nil {
		t.Fatalf("show: %v\n%s", err, out)
	}
	if !strings.Contains(out, "a") {
		t.Errorf("expected account in show output:\n%s", out)
	}
}

func TestFinanceShow_JSON(t *testing.T) {
	// Seed via text output so we get a clean id string without JSON path issues.
	out, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "expense")
	if err != nil {
		t.Fatalf("seed add: %v\n%s", err, out)
	}
	id := extractID(t, out)

	out, err = runFinanceExec(t, "show", "--json", id)
	if err != nil {
		t.Fatalf("show --json: %v\n%s", err, out)
	}
	// show --json encodes *finance.Transaction which has a "Row" field.
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("show output is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := got["Row"]; !ok {
		t.Errorf("expected 'Row' field in show --json output:\n%s", out)
	}
}

func TestFinanceEdit(t *testing.T) {
	out, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "expense")
	if err != nil {
		t.Fatalf("seed add: %v\n%s", err, out)
	}
	id := extractID(t, out)

	out, err = runFinanceExec(t, "edit", id, "--amount", "99", "--kind", "income")
	if err != nil {
		t.Fatalf("edit: %v\n%s", err, out)
	}
	if !strings.Contains(out, "updated") {
		t.Errorf("expected 'updated' in output:\n%s", out)
	}
}

func TestFinanceRmAndRestore(t *testing.T) {
	out, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "expense")
	if err != nil {
		t.Fatalf("seed add: %v\n%s", err, out)
	}
	id := extractID(t, out)

	out, err = runFinanceExec(t, "rm", id)
	if err != nil {
		t.Fatalf("rm: %v\n%s", err, out)
	}
	if !strings.Contains(out, "trash") {
		t.Errorf("expected trash confirmation:\n%s", out)
	}

	out, err = runFinanceExec(t, "restore", id)
	if err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}
	if !strings.Contains(out, "restored") {
		t.Errorf("expected restore confirmation:\n%s", out)
	}
}

func TestFinancePurge_RequiresYesInNonInteractive(t *testing.T) {
	out, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "expense")
	if err != nil {
		t.Fatalf("seed add: %v\n%s", err, out)
	}
	id := extractID(t, out)

	// Without --yes, purge either returns an error (non-TTY: "--yes required")
	// or prints "aborted" (TTY stdin returning EOF). Either way the transaction
	// must NOT be deleted.
	purgeOut, purgeErr := runFinanceExec(t, "purge", id)
	if purgeErr == nil && strings.Contains(purgeOut, "purged") {
		t.Fatal("purge without --yes must not permanently delete the transaction")
	}

	// Verify the transaction still exists.
	showOut, showErr := runFinanceExec(t, "show", id)
	if showErr != nil {
		t.Fatalf("transaction should still exist after aborted purge: %v\n%s", showErr, showOut)
	}
}

func TestFinancePurge_WithYes(t *testing.T) {
	out, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "expense")
	if err != nil {
		t.Fatalf("seed add: %v\n%s", err, out)
	}
	id := extractID(t, out)

	out, err = runFinanceExec(t, "purge", "--yes", id)
	if err != nil {
		t.Fatalf("purge: %v\n%s", err, out)
	}
	if !strings.Contains(out, "purged") {
		t.Errorf("expected purged confirmation:\n%s", out)
	}
}

// --- CSV Export tests ---

// TestFinanceExport_HeaderCorrect verifies the CSV header row matches the spec
// (finance-csv-export scenario: "Header row is correct").
func TestFinanceExport_HeaderCorrect(t *testing.T) {
	out, err := runFinanceExec(t, "export", "--format", "csv")
	if err != nil {
		t.Fatalf("export: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 1 {
		t.Fatal("expected at least one line (header)")
	}
	const wantHeader = "date,kind,amount,currency,account,category,notes,tags"
	if lines[0] != wantHeader {
		t.Errorf("header mismatch\n  got:  %s\n  want: %s", lines[0], wantHeader)
	}
}

// TestFinanceExport_EmptyProducesHeaderOnly verifies that an empty database
// still produces the header row (finance-csv-export scenario: "Empty export").
func TestFinanceExport_EmptyProducesHeaderOnly(t *testing.T) {
	out, err := runFinanceExec(t, "export", "--format", "csv")
	if err != nil {
		t.Fatalf("export: %v\n%s", err, out)
	}
	trimmed := strings.TrimSpace(out)
	const wantHeader = "date,kind,amount,currency,account,category,notes,tags"
	if trimmed != wantHeader {
		t.Errorf("empty export should be header only\n  got: %s", trimmed)
	}
}

// TestFinanceExport_OutputToFile verifies --output writes to a file and nothing
// goes to stdout (finance-csv-export scenario: "Output to file with --output").
func TestFinanceExport_OutputToFile(t *testing.T) {
	// Seed one transaction.
	if _, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "expense"); err != nil {
		t.Fatalf("seed add: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "finance.csv")
	out, err := runFinanceExec(t, "export", "--format", "csv", "--output", outFile)
	if err != nil {
		t.Fatalf("export --output: %v\n%s", err, out)
	}
	// Nothing should be on stdout when writing to file.
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty stdout when --output is set, got:\n%s", out)
	}
	// File must exist and contain the header.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.HasPrefix(string(data), "date,kind,") {
		t.Errorf("output file missing CSV header:\n%s", string(data))
	}
}

// TestFinanceExport_FilterByKind verifies --kind filters only matching rows
// (finance-csv-export scenario: "Filtered export by kind").
func TestFinanceExport_FilterByKind(t *testing.T) {
	if _, err := runFinanceExec(t, "add", "--amount", "10", "--kind", "expense", "--date", "2026-06-01"); err != nil {
		t.Fatalf("seed expense: %v", err)
	}
	if _, err := runFinanceExec(t, "add", "--amount", "20", "--kind", "income", "--date", "2026-06-02"); err != nil {
		t.Fatalf("seed income: %v", err)
	}

	out, err := runFinanceExec(t, "export", "--format", "csv", "--kind", "expense")
	if err != nil {
		t.Fatalf("export --kind expense: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// header + 1 data row
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (header + 1 expense), got %d:\n%s", len(lines), out)
	}
	if len(lines) > 1 && !strings.Contains(lines[1], "expense") {
		t.Errorf("data row should contain 'expense':\n%s", lines[1])
	}
}

// TestFinanceExport_TagsQuoted verifies multiple tags produce a comma-separated
// quoted cell (finance-csv-export scenario: "Tags column is comma-separated within
// quoted cell"). Tag order may vary; we verify the cell is quoted and contains both.
func TestFinanceExport_TagsQuoted(t *testing.T) {
	if _, err := runFinanceExec(t, "add", "--amount", "5", "--kind", "expense",
		"--tag", "work", "--tag", "urgent"); err != nil {
		t.Fatalf("seed add: %v", err)
	}

	out, err := runFinanceExec(t, "export", "--format", "csv")
	if err != nil {
		t.Fatalf("export: %v\n%s", err, out)
	}
	// encoding/csv quotes any field containing a comma, so the tags cell
	// must be quoted and contain both tag names.
	if !strings.Contains(out, "work") || !strings.Contains(out, "urgent") {
		t.Errorf("expected both tag names in output:\n%s", out)
	}
	// Verify the tags cell is quoted (the comma between tags forces quoting).
	if !strings.Contains(out, `"`) {
		t.Errorf("expected quoted tags cell in output:\n%s", out)
	}
}

// TestWriteFinanceCSV_SpecialCharsEscaped verifies RFC 4180 escaping for notes
// containing commas and quotes (finance-csv-export scenario: "Special characters
// in notes are escaped").
func TestWriteFinanceCSV_SpecialCharsEscaped(t *testing.T) {
	rows := []finance.ExportRow{
		{
			Date:     "2026-06-10",
			Kind:     "expense",
			Amount:   99.0,
			Currency: "USD",
			Account:  "test",
			Notes:    `lunch, "expensive"`,
		},
	}
	var buf bytes.Buffer
	if err := writeFinanceCSV(&buf, rows); err != nil {
		t.Fatalf("writeFinanceCSV: %v", err)
	}
	output := buf.String()
	// encoding/csv must double-quote the notes field containing commas/quotes.
	if !strings.Contains(output, `"lunch, ""expensive"""`) {
		t.Errorf("notes field not properly RFC 4180 escaped:\n%s", output)
	}
}

func extractID(t *testing.T, s string) string {
	t.Helper()
	// Text output format: "created 1: ..."
	fields := strings.Fields(s)
	if len(fields) < 2 {
		t.Fatalf("could not extract id from: %s", s)
	}
	return strings.TrimSuffix(fields[1], ":")
}

func int64ToStr(n int64) string {
	var buf [32]byte
	return string(strconvAppendInt(buf[:0], n, 10))
}

func strconvAppendInt(dst []byte, n int64, base int) []byte {
	// Minimal base-10 formatter for int64.
	if n < 0 {
		dst = append(dst, '-')
		n = -n
	}
	if n == 0 {
		return append(dst, '0')
	}
	var digits [32]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%int64(base))
		n /= int64(base)
	}
	return append(dst, digits[i:]...)
}

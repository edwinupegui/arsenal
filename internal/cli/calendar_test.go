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

	"github.com/edwinupegui/arsenal/internal/migrations"
)

// calendarTestHomes caches per-test ARSENAL_HOME dirs so multiple
// runCalendarExec calls within the same test share the same SQLite database.
var (
	calendarTestHomesMu sync.Mutex
	calendarTestHomes   = map[string]string{}
)

// calendarTestHome returns a stable temp directory for t, creating it once
// per test name and registering cleanup on first creation.
func calendarTestHome(t *testing.T) string {
	t.Helper()
	key := t.Name()
	calendarTestHomesMu.Lock()
	defer calendarTestHomesMu.Unlock()
	if h, ok := calendarTestHomes[key]; ok {
		return h
	}
	dir, err := os.MkdirTemp("", "arsenal-calendar-test-*")
	if err != nil {
		t.Fatalf("create test home: %v", err)
	}
	t.Cleanup(func() {
		calendarTestHomesMu.Lock()
		delete(calendarTestHomes, key)
		calendarTestHomesMu.Unlock()
		_ = os.RemoveAll(dir)
	})
	h := filepath.Join(dir, "home")
	calendarTestHomes[key] = h
	return h
}

// runCalendarExec builds a fresh `arsenal calendar` subcommand tree, points
// ARSENAL_HOME at a stable per-test directory, wires the embedded migrations,
// and executes args. Multiple calls within the same test share the same DB.
func runCalendarExec(t *testing.T, args ...string) (string, error) {
	t.Helper()
	migrationsFS = migrations.FS
	t.Setenv("ARSENAL_HOME", calendarTestHome(t))

	root := &cobra.Command{Use: "arsenal"}
	root.AddCommand(newCalendarCmd())

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"calendar"}, args...))

	err := root.Execute()
	return buf.String(), err
}

// extractCalendarID extracts the event id from "created <id>: ..." output.
func extractCalendarID(t *testing.T, s string) string {
	t.Helper()
	fields := strings.Fields(s)
	if len(fields) < 2 {
		t.Fatalf("could not extract id from: %s", s)
	}
	return strings.TrimSuffix(fields[1], ":")
}

// --- Help ---

func TestCalendarCmd_HelpListsSubcommands(t *testing.T) {
	out, _ := runCalendarExec(t)
	for _, sub := range []string{"add", "list", "show", "edit", "rm", "restore", "purge", "export"} {
		if !strings.Contains(out, sub) {
			t.Errorf("help output missing subcommand %q:\n%s", sub, out)
		}
	}
}

// --- Add ---

func TestCalendarAdd_TimedEvent(t *testing.T) {
	out, err := runCalendarExec(t,
		"add",
		"--title", "Team standup",
		"--start", "2026-06-15T09:00",
		"--end", "2026-06-15T09:30",
		"--recurrence", "daily",
	)
	if err != nil {
		t.Fatalf("add timed event: %v\n%s", err, out)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("expected 'created' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Team standup") {
		t.Errorf("expected title in output, got:\n%s", out)
	}
}

func TestCalendarAdd_AllDayEvent(t *testing.T) {
	out, err := runCalendarExec(t,
		"add",
		"--title", "Holiday",
		"--start", "2026-12-25",
		"--all-day",
	)
	if err != nil {
		t.Fatalf("add all-day event: %v\n%s", err, out)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("expected 'created' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "all-day:    true") {
		t.Errorf("expected all-day in output, got:\n%s", out)
	}
}

func TestCalendarAdd_AllDayInferred(t *testing.T) {
	// When --start is date-only, all-day should be inferred without --all-day flag.
	out, err := runCalendarExec(t,
		"add",
		"--title", "Birthday",
		"--start", "2026-08-10",
	)
	if err != nil {
		t.Fatalf("add inferred all-day: %v\n%s", err, out)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("expected 'created' in output, got:\n%s", out)
	}
}

func TestCalendarAdd_MissingTitle(t *testing.T) {
	_, err := runCalendarExec(t, "add", "--start", "2026-06-15T09:00")
	if err == nil {
		t.Fatal("expected error for missing --title")
	}
}

func TestCalendarAdd_MissingStart(t *testing.T) {
	_, err := runCalendarExec(t, "add", "--title", "No start")
	if err == nil {
		t.Fatal("expected error for missing --start")
	}
}

func TestCalendarAdd_InvalidRecurrence(t *testing.T) {
	out, err := runCalendarExec(t,
		"add",
		"--title", "Bad recurrence",
		"--start", "2026-06-15T09:00",
		"--recurrence", "biweekly",
	)
	if err == nil {
		t.Fatal("expected error for invalid recurrence")
	}
	if !strings.Contains(out, "invalid recurrence") {
		t.Errorf("expected invalid recurrence error, got:\n%s", out)
	}
}

func TestCalendarAdd_JSON(t *testing.T) {
	out, err := runCalendarExec(t,
		"add",
		"--title", "JSON event",
		"--start", "2026-06-15T10:00",
		"--json",
	)
	if err != nil {
		t.Fatalf("add --json: %v\n%s", err, out)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
}

// --- List ---

func TestCalendarList_DateRange(t *testing.T) {
	if _, err := runCalendarExec(t, "add", "--title", "June event", "--start", "2026-06-10T09:00"); err != nil {
		t.Fatalf("seed event 1: %v", err)
	}
	if _, err := runCalendarExec(t, "add", "--title", "July event", "--start", "2026-07-01T09:00"); err != nil {
		t.Fatalf("seed event 2: %v", err)
	}

	out, err := runCalendarExec(t, "list", "--from", "2026-06-01", "--to", "2026-06-30")
	if err != nil {
		t.Fatalf("list --from/--to: %v\n%s", err, out)
	}
	if !strings.Contains(out, "June event") {
		t.Errorf("expected 'June event' in list output:\n%s", out)
	}
	if strings.Contains(out, "July event") {
		t.Errorf("did not expect 'July event' in filtered output:\n%s", out)
	}
}

func TestCalendarList_JSON(t *testing.T) {
	if _, err := runCalendarExec(t, "add", "--title", "JSON list event", "--start", "2026-06-15T09:00"); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	out, err := runCalendarExec(t, "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v\n%s", err, out)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("output is not valid JSON array: %v\n%s", err, out)
	}
	if len(arr) == 0 {
		t.Error("expected at least one event in JSON output")
	}
}

func TestCalendarList_Trashed(t *testing.T) {
	out, err := runCalendarExec(t, "add", "--title", "To be trashed", "--start", "2026-06-15T09:00")
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	id := extractCalendarID(t, out)

	if _, err := runCalendarExec(t, "rm", id); err != nil {
		t.Fatalf("rm event: %v", err)
	}

	// Default list should not include trashed.
	out, err = runCalendarExec(t, "list")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	if strings.Contains(out, "To be trashed") {
		t.Errorf("trashed event should not appear in default list:\n%s", out)
	}

	// --trashed should include it.
	out, err = runCalendarExec(t, "list", "--trashed")
	if err != nil {
		t.Fatalf("list --trashed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "To be trashed") {
		t.Errorf("expected trashed event in --trashed list:\n%s", out)
	}
}

// --- Show ---

func TestCalendarShow(t *testing.T) {
	out, err := runCalendarExec(t,
		"add",
		"--title", "Show me",
		"--start", "2026-06-15T09:00",
		"--location", "Conference room",
	)
	if err != nil {
		t.Fatalf("seed event: %v\n%s", err, out)
	}
	id := extractCalendarID(t, out)

	out, err = runCalendarExec(t, "show", id)
	if err != nil {
		t.Fatalf("show: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Show me") {
		t.Errorf("expected title in show output:\n%s", out)
	}
	if !strings.Contains(out, "Conference room") {
		t.Errorf("expected location in show output:\n%s", out)
	}
	if !strings.Contains(out, "All-day:") {
		t.Errorf("expected All-day field in show output:\n%s", out)
	}
}

func TestCalendarShow_NonExistent(t *testing.T) {
	_, err := runCalendarExec(t, "show", "9999")
	if err == nil {
		t.Fatal("expected error for non-existent event")
	}
}

// --- Edit ---

func TestCalendarEdit(t *testing.T) {
	out, err := runCalendarExec(t, "add", "--title", "Original title", "--start", "2026-06-15T09:00")
	if err != nil {
		t.Fatalf("seed event: %v\n%s", err, out)
	}
	id := extractCalendarID(t, out)

	out, err = runCalendarExec(t, "edit", id, "--title", "Updated title", "--recurrence", "weekly")
	if err != nil {
		t.Fatalf("edit: %v\n%s", err, out)
	}
	if !strings.Contains(out, "updated") {
		t.Errorf("expected 'updated' in output:\n%s", out)
	}

	// Verify the change was applied.
	out, err = runCalendarExec(t, "show", id)
	if err != nil {
		t.Fatalf("show after edit: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Updated title") {
		t.Errorf("expected updated title in show output:\n%s", out)
	}
}

// --- Rm, Restore, Purge ---

func TestCalendarRmAndRestore(t *testing.T) {
	out, err := runCalendarExec(t, "add", "--title", "To trash", "--start", "2026-06-15T09:00")
	if err != nil {
		t.Fatalf("seed event: %v\n%s", err, out)
	}
	id := extractCalendarID(t, out)

	out, err = runCalendarExec(t, "rm", id)
	if err != nil {
		t.Fatalf("rm: %v\n%s", err, out)
	}
	if !strings.Contains(out, "trash") {
		t.Errorf("expected trash confirmation:\n%s", out)
	}

	out, err = runCalendarExec(t, "restore", id)
	if err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}
	if !strings.Contains(out, "restored") {
		t.Errorf("expected restore confirmation:\n%s", out)
	}
}

func TestCalendarPurge_RequiresYesInNonInteractive(t *testing.T) {
	out, err := runCalendarExec(t, "add", "--title", "To purge maybe", "--start", "2026-06-15T09:00")
	if err != nil {
		t.Fatalf("seed event: %v\n%s", err, out)
	}
	id := extractCalendarID(t, out)

	// Without --yes, purge either returns an error (non-TTY: "--yes required")
	// or prints "aborted" (TTY stdin returning EOF). Either way the event must NOT be deleted.
	purgeOut, purgeErr := runCalendarExec(t, "purge", id)
	if purgeErr == nil && strings.Contains(purgeOut, "purged") {
		t.Fatal("purge without --yes must not permanently delete the event")
	}

	// Verify the event still exists.
	showOut, showErr := runCalendarExec(t, "show", id)
	if showErr != nil {
		t.Fatalf("event should still exist after aborted purge: %v\n%s", showErr, showOut)
	}
}

func TestCalendarPurge_WithYes(t *testing.T) {
	out, err := runCalendarExec(t, "add", "--title", "To purge", "--start", "2026-06-15T09:00")
	if err != nil {
		t.Fatalf("seed event: %v\n%s", err, out)
	}
	id := extractCalendarID(t, out)

	out, err = runCalendarExec(t, "purge", "--yes", id)
	if err != nil {
		t.Fatalf("purge --yes: %v\n%s", err, out)
	}
	if !strings.Contains(out, "purged") {
		t.Errorf("expected 'purged' in output:\n%s", out)
	}
}

// --- Export ---

func TestCalendarExport_StdoutICal(t *testing.T) {
	if _, err := runCalendarExec(t, "add", "--title", "Export event", "--start", "2026-06-15T09:00"); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	out, err := runCalendarExec(t, "export", "--format", "ical")
	if err != nil {
		t.Fatalf("export --format ical: %v\n%s", err, out)
	}
	if !strings.Contains(out, "BEGIN:VCALENDAR") {
		t.Errorf("expected BEGIN:VCALENDAR in iCal output:\n%s", out)
	}
	if !strings.Contains(out, "END:VCALENDAR") {
		t.Errorf("expected END:VCALENDAR in iCal output:\n%s", out)
	}
	if !strings.Contains(out, "VEVENT") {
		t.Errorf("expected VEVENT in iCal output:\n%s", out)
	}
}

func TestCalendarExport_EmptyIsValidVCALENDAR(t *testing.T) {
	// Empty DB — no events seeded.
	out, err := runCalendarExec(t, "export", "--format", "ical")
	if err != nil {
		t.Fatalf("export empty: %v\n%s", err, out)
	}
	if !strings.Contains(out, "BEGIN:VCALENDAR") {
		t.Errorf("empty export should still have BEGIN:VCALENDAR:\n%s", out)
	}
	if !strings.Contains(out, "END:VCALENDAR") {
		t.Errorf("empty export should still have END:VCALENDAR:\n%s", out)
	}
	// Should NOT contain VEVENT.
	if strings.Contains(out, "VEVENT") {
		t.Errorf("empty export should not have VEVENT:\n%s", out)
	}
}

func TestCalendarExport_AllDayVSTimedDTSTART(t *testing.T) {
	if _, err := runCalendarExec(t, "add", "--title", "All day event", "--start", "2026-06-15", "--all-day"); err != nil {
		t.Fatalf("seed all-day event: %v", err)
	}
	if _, err := runCalendarExec(t, "add", "--title", "Timed event", "--start", "2026-06-15T09:00"); err != nil {
		t.Fatalf("seed timed event: %v", err)
	}

	out, err := runCalendarExec(t, "export", "--format", "ical")
	if err != nil {
		t.Fatalf("export: %v\n%s", err, out)
	}
	// All-day: DTSTART;VALUE=DATE
	if !strings.Contains(out, "DTSTART;VALUE=DATE") {
		t.Errorf("expected DTSTART;VALUE=DATE for all-day event:\n%s", out)
	}
	// Timed: DTSTART without ;VALUE=DATE
	if !strings.Contains(out, "DTSTART:20260615T") {
		t.Errorf("expected DTSTART:20260615T for timed event:\n%s", out)
	}
}

func TestCalendarExport_OutputToFile(t *testing.T) {
	if _, err := runCalendarExec(t, "add", "--title", "File export event", "--start", "2026-06-15T09:00"); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "calendar.ics")
	out, err := runCalendarExec(t, "export", "--format", "ical", "--output", outFile)
	if err != nil {
		t.Fatalf("export --output: %v\n%s", err, out)
	}
	// Nothing should be on stdout when writing to file.
	if strings.TrimSpace(out) != "" {
		t.Errorf("expected empty stdout when --output is set, got:\n%s", out)
	}
	// File must exist and contain valid VCALENDAR.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(data), "BEGIN:VCALENDAR") {
		t.Errorf("output file missing BEGIN:VCALENDAR:\n%s", string(data))
	}
}

func TestCalendarExport_UnsupportedFormat(t *testing.T) {
	_, err := runCalendarExec(t, "export", "--format", "csv")
	if err == nil {
		t.Fatal("expected error for unsupported format csv")
	}
}

func TestCalendarExport_DateRangeFilter(t *testing.T) {
	if _, err := runCalendarExec(t, "add", "--title", "June event", "--start", "2026-06-10T09:00"); err != nil {
		t.Fatalf("seed june event: %v", err)
	}
	if _, err := runCalendarExec(t, "add", "--title", "July event", "--start", "2026-07-01T09:00"); err != nil {
		t.Fatalf("seed july event: %v", err)
	}

	out, err := runCalendarExec(t, "export", "--format", "ical", "--from", "2026-06-01", "--to", "2026-06-30")
	if err != nil {
		t.Fatalf("export with date filter: %v\n%s", err, out)
	}
	if !strings.Contains(out, "June event") {
		t.Errorf("expected 'June event' in filtered export:\n%s", out)
	}
	if strings.Contains(out, "July event") {
		t.Errorf("did not expect 'July event' in date-filtered export:\n%s", out)
	}
}

// --- Completions ---

func TestCalendarCompletion_Recurrences(t *testing.T) {
	completions, _ := completeCalendarRecurrences(nil, nil, "")
	for _, want := range []string{"none", "daily", "weekly", "monthly", "yearly"} {
		found := false
		for _, c := range completions {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected recurrence %q in completions %v", want, completions)
		}
	}
}

func TestCalendarCompletion_Formats(t *testing.T) {
	completions, _ := completeCalendarFormats(nil, nil, "")
	if len(completions) != 1 || completions[0] != "ical" {
		t.Errorf("expected [ical] format completions, got %v", completions)
	}
}

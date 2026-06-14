package calendar_test

import (
	"strings"
	"testing"

	"github.com/edwinupegui/arsenal/internal/calendar"
)

// icalLines splits iCal output into unfolded logical lines.
// RFC 5545 §3.1: folded lines begin with CRLF + SPACE; this joins them back.
func icalLines(output string) []string {
	// Normalize CRLF to LF for easier splitting, then unfold.
	output = strings.ReplaceAll(output, "\r\n ", "")
	output = strings.ReplaceAll(output, "\r\n", "\n")
	lines := strings.Split(output, "\n")
	// Trim trailing empty element from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// containsLine reports whether lines contains an exact match for s.
func containsLine(lines []string, s string) bool {
	for _, l := range lines {
		if l == s {
			return true
		}
	}
	return false
}

// hasLinePrefix reports whether any line starts with prefix.
func hasLinePrefix(lines []string, prefix string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

// writeICal is a test helper that calls calendar.WriteICal and returns the
// output as a string, failing the test on error.
func writeICalStr(t *testing.T, rows []calendar.ExportRow) string {
	t.Helper()
	var sb strings.Builder
	if err := calendar.WriteICal(&sb, rows); err != nil {
		t.Fatalf("WriteICal: %v", err)
	}
	return sb.String()
}

// --- Envelope tests ---

func TestICal_EmptyExport_ValidEnvelope(t *testing.T) {
	out := writeICalStr(t, nil)
	lines := icalLines(out)

	if lines[0] != "BEGIN:VCALENDAR" {
		t.Errorf("first line = %q, want BEGIN:VCALENDAR", lines[0])
	}
	if lines[len(lines)-1] != "END:VCALENDAR" {
		t.Errorf("last line = %q, want END:VCALENDAR", lines[len(lines)-1])
	}
	if !containsLine(lines, "VERSION:2.0") {
		t.Error("missing VERSION:2.0")
	}
	if !containsLine(lines, "PRODID:-//Arsenal//Calendar//EN") {
		t.Error("missing PRODID:-//Arsenal//Calendar//EN")
	}
}

func TestICal_AllLinesCRLF(t *testing.T) {
	out := writeICalStr(t, nil)
	// Every logical line must end with CRLF.
	// Split on CRLF and verify no bare LF remains (except within folded continuations).
	// Simple check: the raw output must not contain a bare \n that is not preceded by \r.
	for i, ch := range out {
		if ch == '\n' && i > 0 && out[i-1] != '\r' {
			t.Errorf("found bare LF at offset %d (not preceded by CR)", i)
			break
		}
	}
}

// --- VEVENT required fields ---

func TestICal_VEVENTRequiredFields(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        5,
			Title:     "Team standup",
			StartAt:   "2026-06-15T09:00:00",
			EndAt:     "2026-06-15T09:30:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	out := writeICalStr(t, rows)
	lines := icalLines(out)

	if !containsLine(lines, "BEGIN:VEVENT") {
		t.Error("missing BEGIN:VEVENT")
	}
	if !containsLine(lines, "END:VEVENT") {
		t.Error("missing END:VEVENT")
	}
	if !containsLine(lines, "UID:5@arsenal") {
		t.Error("missing UID:5@arsenal")
	}
	if !containsLine(lines, "SUMMARY:Team standup") {
		t.Error("missing SUMMARY:Team standup")
	}
	if !containsLine(lines, "DTSTART:20260615T090000") {
		t.Errorf("missing DTSTART:20260615T090000; lines:\n%s", strings.Join(lines, "\n"))
	}
	if !containsLine(lines, "DTEND:20260615T093000") {
		t.Errorf("missing DTEND:20260615T093000; lines:\n%s", strings.Join(lines, "\n"))
	}
	// DTSTAMP must be present
	if !hasLinePrefix(lines, "DTSTAMP:") {
		t.Error("missing DTSTAMP")
	}
}

func TestICal_DTSTAMPFromCreatedAt(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Test",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:30:00.000Z",
		},
	}
	out := writeICalStr(t, rows)
	lines := icalLines(out)

	// DTSTAMP should be formatted as basic UTC: 20260601T083000Z
	if !containsLine(lines, "DTSTAMP:20260601T083000Z") {
		t.Errorf("DTSTAMP not found or wrong format; lines:\n%s", strings.Join(lines, "\n"))
	}
}

// --- NULL end_at ---

func TestICal_NullEndAt_NoDTEND(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Open-ended",
			StartAt:   "2026-06-15T09:00:00",
			EndAt:     "", // empty = NULL
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	out := writeICalStr(t, rows)
	lines := icalLines(out)

	if hasLinePrefix(lines, "DTEND") {
		t.Error("expected no DTEND line when end_at is empty")
	}
}

// --- All-day DATE value type ---

func TestICal_AllDay_DTSTARTValueDate(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        2,
			Title:     "Company Holiday",
			StartAt:   "2026-06-15",
			EndAt:     "2026-06-16",
			AllDay:    true,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	out := writeICalStr(t, rows)
	lines := icalLines(out)

	if !containsLine(lines, "DTSTART;VALUE=DATE:20260615") {
		t.Errorf("missing DTSTART;VALUE=DATE:20260615; lines:\n%s", strings.Join(lines, "\n"))
	}
	if !containsLine(lines, "DTEND;VALUE=DATE:20260616") {
		t.Errorf("missing DTEND;VALUE=DATE:20260616; lines:\n%s", strings.Join(lines, "\n"))
	}
	// The value portion (after the colon) must not contain a time component (HHMMSs).
	for _, l := range lines {
		if strings.HasPrefix(l, "DTSTART") {
			parts := strings.SplitN(l, ":", 2)
			if len(parts) == 2 && strings.Contains(parts[1], "T") {
				t.Errorf("DTSTART for all-day contains time component in value: %q", l)
			}
		}
	}
}

func TestICal_AllDay_NullEndAt_NoDTEND(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        3,
			Title:     "All Day Open",
			StartAt:   "2026-06-15",
			EndAt:     "",
			AllDay:    true,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	out := writeICalStr(t, rows)
	lines := icalLines(out)

	if hasLinePrefix(lines, "DTEND") {
		t.Error("expected no DTEND for all-day open-ended event")
	}
}

// --- RRULE mapping ---

func TestICal_RRULE_Daily(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Daily standup",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceDaily,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, "RRULE:FREQ=DAILY") {
		t.Errorf("missing RRULE:FREQ=DAILY; lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestICal_RRULE_Weekly(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Weekly review",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceWeekly,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, "RRULE:FREQ=WEEKLY") {
		t.Error("missing RRULE:FREQ=WEEKLY")
	}
}

func TestICal_RRULE_Monthly(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Monthly billing",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceMonthly,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, "RRULE:FREQ=MONTHLY") {
		t.Error("missing RRULE:FREQ=MONTHLY")
	}
}

func TestICal_RRULE_Yearly(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Birthday",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceYearly,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, "RRULE:FREQ=YEARLY") {
		t.Error("missing RRULE:FREQ=YEARLY")
	}
}

func TestICal_RRULE_None_Omitted(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "One-time event",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if hasLinePrefix(lines, "RRULE") {
		t.Error("expected no RRULE line for recurrence=none")
	}
}

// --- Optional fields ---

func TestICal_Description_IncludedWhenNonEmpty(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:          1,
			Title:       "Sync",
			Description: "Daily sync",
			StartAt:     "2026-06-15T09:00:00",
			AllDay:      false,
			Recurrence:  calendar.RecurrenceNone,
			CreatedAt:   "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, "DESCRIPTION:Daily sync") {
		t.Errorf("missing DESCRIPTION:Daily sync; lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestICal_Description_OmittedWhenEmpty(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Sync",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if hasLinePrefix(lines, "DESCRIPTION") {
		t.Error("expected no DESCRIPTION when empty")
	}
}

func TestICal_Location_IncludedWhenNonEmpty(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Sync",
			Location:  "Conference Room A",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, "LOCATION:Conference Room A") {
		t.Errorf("missing LOCATION line; lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestICal_Location_OmittedWhenEmpty(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Sync",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if hasLinePrefix(lines, "LOCATION") {
		t.Error("expected no LOCATION when empty")
	}
}

// --- RFC 5545 text escaping ---

func TestICal_Escaping_Backslash(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     `Back\slash`,
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, `SUMMARY:Back\\slash`) {
		t.Errorf("backslash not escaped; lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestICal_Escaping_Semicolon(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "A;B",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, `SUMMARY:A\;B`) {
		t.Errorf("semicolon not escaped; lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestICal_Escaping_Comma(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Lunch, Dinner",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, `SUMMARY:Lunch\, Dinner`) {
		t.Errorf("comma not escaped; lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestICal_Escaping_Newline(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:          1,
			Title:       "Multi",
			Description: "line1\nline2",
			StartAt:     "2026-06-15T09:00:00",
			AllDay:      false,
			Recurrence:  calendar.RecurrenceNone,
			CreatedAt:   "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, `DESCRIPTION:line1\nline2`) {
		t.Errorf("newline not escaped; lines:\n%s", strings.Join(lines, "\n"))
	}
}

// --- Line folding ---

func TestICal_LineFolding_LongSummary(t *testing.T) {
	// Title long enough to exceed 75 octets when prefixed with "SUMMARY:".
	// "SUMMARY:" is 8 chars, so 68 chars of title exceeds 75.
	longTitle := strings.Repeat("A", 80)
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     longTitle,
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	out := writeICalStr(t, rows)

	// In the raw output (with CRLF), a folded line has a CRLF + SPACE continuation.
	if !strings.Contains(out, "\r\n ") {
		t.Error("expected line folding (CRLF + SPACE) in output with long title")
	}

	// After unfolding, the full SUMMARY line must be present.
	lines := icalLines(out)
	if !containsLine(lines, "SUMMARY:"+longTitle) {
		t.Errorf("unfolded SUMMARY line not found; lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestICal_NoFolding_ShortLines(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Short",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	out := writeICalStr(t, rows)
	// Raw lines (split on CRLF) must be ≤ 75 octets each.
	rawLines := strings.Split(out, "\r\n")
	for _, l := range rawLines {
		if l == "" {
			continue
		}
		if len([]byte(l)) > 75 {
			t.Errorf("line exceeds 75 octets: %q", l)
		}
	}
}

// --- Multiple events ---

func TestICal_MultipleEvents(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Event One",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
		{
			ID:        2,
			Title:     "Event Two",
			StartAt:   "2026-06-16T10:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceWeekly,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	out := writeICalStr(t, rows)
	lines := icalLines(out)

	// Two VEVENT blocks.
	count := 0
	for _, l := range lines {
		if l == "BEGIN:VEVENT" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 VEVENT blocks, got %d", count)
	}
	if !containsLine(lines, "UID:1@arsenal") {
		t.Error("missing UID:1@arsenal")
	}
	if !containsLine(lines, "UID:2@arsenal") {
		t.Error("missing UID:2@arsenal")
	}
}

// --- CATEGORIES ---

func TestICal_Categories_CategoryAndTags(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Tagged",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			Category:  "Work",
			Tags:      []string{"urgent", "review"},
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, "CATEGORIES:Work,urgent,review") {
		t.Errorf("CATEGORIES line not found or wrong; lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestICal_Categories_TagsOnly(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "Tagged no cat",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			Tags:      []string{"alpha"},
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if !containsLine(lines, "CATEGORIES:alpha") {
		t.Errorf("CATEGORIES line not found; lines:\n%s", strings.Join(lines, "\n"))
	}
}

func TestICal_Categories_OmittedWhenEmpty(t *testing.T) {
	rows := []calendar.ExportRow{
		{
			ID:        1,
			Title:     "No cat no tags",
			StartAt:   "2026-06-15T09:00:00",
			AllDay:    false,
			Recurrence: calendar.RecurrenceNone,
			CreatedAt: "2026-06-01T08:00:00.000Z",
		},
	}
	lines := icalLines(writeICalStr(t, rows))
	if hasLinePrefix(lines, "CATEGORIES") {
		t.Error("expected no CATEGORIES line when category and tags are empty")
	}
}

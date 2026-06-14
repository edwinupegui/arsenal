package calendar

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// WriteICal writes an RFC 5545-compliant VCALENDAR document to w.
// Each ExportRow becomes one VEVENT block. An empty rows slice still produces a
// valid VCALENDAR envelope with no VEVENT blocks.
func WriteICal(w io.Writer, rows []ExportRow) error {
	crlf := "\r\n"
	write := func(s string) error {
		_, err := io.WriteString(w, foldLine(s)+crlf)
		return err
	}

	if err := write("BEGIN:VCALENDAR"); err != nil {
		return err
	}
	if err := write("VERSION:2.0"); err != nil {
		return err
	}
	if err := write("PRODID:-//Arsenal//Calendar//EN"); err != nil {
		return err
	}
	if err := write("CALSCALE:GREGORIAN"); err != nil {
		return err
	}

	for _, row := range rows {
		if err := writeVEVENT(w, row); err != nil {
			return err
		}
	}

	if err := write("END:VCALENDAR"); err != nil {
		return err
	}
	return nil
}

// writeVEVENT emits one VEVENT block for the given row.
func writeVEVENT(w io.Writer, row ExportRow) error {
	crlf := "\r\n"
	write := func(s string) error {
		_, err := io.WriteString(w, foldLine(s)+crlf)
		return err
	}

	if err := write("BEGIN:VEVENT"); err != nil {
		return err
	}

	// UID: derived from event ID.
	if err := write(fmt.Sprintf("UID:%d@arsenal", row.ID)); err != nil {
		return err
	}

	// DTSTAMP: from created_at (stored as UTC, formatted as basic UTC datetime).
	dtstamp := formatDTSTAMP(row.CreatedAt)
	if err := write("DTSTAMP:" + dtstamp); err != nil {
		return err
	}

	// SUMMARY: escaped title.
	if err := write("SUMMARY:" + escapeText(row.Title)); err != nil {
		return err
	}

	// DTSTART: value type depends on all_day.
	dtstart := formatICalDateTime(row.StartAt, row.AllDay)
	if row.AllDay {
		if err := write("DTSTART;VALUE=DATE:" + dtstart); err != nil {
			return err
		}
	} else {
		if err := write("DTSTART:" + dtstart); err != nil {
			return err
		}
	}

	// DTEND: omitted when EndAt is empty.
	if row.EndAt != "" {
		dtend := formatICalDateTime(row.EndAt, row.AllDay)
		if row.AllDay {
			if err := write("DTEND;VALUE=DATE:" + dtend); err != nil {
				return err
			}
		} else {
			if err := write("DTEND:" + dtend); err != nil {
				return err
			}
		}
	}

	// DESCRIPTION: omitted when empty.
	if row.Description != "" {
		if err := write("DESCRIPTION:" + escapeText(row.Description)); err != nil {
			return err
		}
	}

	// LOCATION: omitted when empty.
	if row.Location != "" {
		if err := write("LOCATION:" + escapeText(row.Location)); err != nil {
			return err
		}
	}

	// RRULE: omitted for recurrence=none.
	if rrule := mapRRULE(row.Recurrence); rrule != "" {
		if err := write("RRULE:" + rrule); err != nil {
			return err
		}
	}

	// CATEGORIES: comma-joined category + tags; omitted when all empty.
	cats := buildCategories(row.Category, row.Tags)
	if cats != "" {
		if err := write("CATEGORIES:" + cats); err != nil {
			return err
		}
	}

	if err := write("END:VEVENT"); err != nil {
		return err
	}
	return nil
}

// formatICalDateTime converts a stored start_at/end_at string to an iCal
// value:
//   - All-day (allDay=true): input is "YYYY-MM-DD" → output is "YYYYMMDD".
//   - Timed (allDay=false): input is "YYYY-MM-DDTHH:MM:SS" → output is
//     "YYYYMMDDTHHMMSS" (floating local time, no Z, no TZID).
func formatICalDateTime(stored string, allDay bool) string {
	if allDay {
		// Strip the single hyphen separators.
		return strings.ReplaceAll(stored, "-", "")
	}
	// Timed: "2026-06-15T09:00:00" → "20260615T090000"
	t, err := time.ParseInLocation("2006-01-02T15:04:05", stored, time.Local)
	if err != nil {
		// Fallback: strip separators manually.
		s := strings.ReplaceAll(stored, "-", "")
		s = strings.ReplaceAll(s, ":", "")
		return s
	}
	return t.Format("20060102T150405")
}

// formatDTSTAMP converts a created_at string (stored as "YYYY-MM-DDTHH:MM:SS.sssZ"
// UTC) to the iCal basic UTC form "YYYYMMDDTHHMMSSZ".
func formatDTSTAMP(createdAt string) string {
	// Try parsing with milliseconds (common SQLite strftime output).
	for _, layout := range []string{
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999999999Z",
	} {
		t, err := time.Parse(layout, createdAt)
		if err == nil {
			return t.UTC().Format("20060102T150405Z")
		}
	}
	// Fallback: strip separators and keep the Z.
	s := strings.ReplaceAll(createdAt, "-", "")
	s = strings.ReplaceAll(s, ":", "")
	// Remove the dot and milliseconds before the Z.
	if idx := strings.Index(s, "."); idx != -1 {
		if zIdx := strings.Index(s, "Z"); zIdx != -1 {
			s = s[:idx] + "Z"
		}
	}
	return s
}

// mapRRULE returns the RRULE value string for the given recurrence.
// Returns an empty string when recurrence is none (caller omits the line).
func mapRRULE(r Recurrence) string {
	switch r {
	case RecurrenceDaily:
		return "FREQ=DAILY"
	case RecurrenceWeekly:
		return "FREQ=WEEKLY"
	case RecurrenceMonthly:
		return "FREQ=MONTHLY"
	case RecurrenceYearly:
		return "FREQ=YEARLY"
	default:
		return ""
	}
}

// escapeText applies RFC 5545 text escaping to a property value:
//
//	\  → \\
//	;  → \;
//	,  → \,
//	LF → \n
func escapeText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// buildCategories assembles the CATEGORIES value from the category name and
// tags. Items are escaped individually, then joined with literal commas.
// Returns an empty string when there is nothing to emit.
func buildCategories(category string, tags []string) string {
	var parts []string
	if category != "" {
		parts = append(parts, escapeText(category))
	}
	for _, tag := range tags {
		if tag != "" {
			parts = append(parts, escapeText(tag))
		}
	}
	return strings.Join(parts, ",")
}

// foldLine folds a content line so that each raw line (before CRLF) is at most
// 75 octets, per RFC 5545 §3.1. Continuation lines begin with a single SPACE.
// The returned string does NOT include a trailing CRLF (the caller appends it).
func foldLine(line string) string {
	const maxOctets = 75
	b := []byte(line)
	if len(b) <= maxOctets {
		return line
	}

	var sb strings.Builder
	written := 0
	for i := 0; i < len(b); {
		remaining := maxOctets - written
		if remaining <= 0 {
			// Start a new folded continuation.
			sb.WriteString("\r\n ")
			written = 1 // the leading space counts
			remaining = maxOctets - 1
		}
		// Advance by up to remaining bytes, but keep multi-byte UTF-8
		// characters whole. Walk forward to find a safe split point.
		end := i + remaining
		if end >= len(b) {
			end = len(b)
		} else {
			// Back up to the last byte that starts a UTF-8 sequence so we do
			// not split mid-character.
			for end > i && b[end]&0xC0 == 0x80 {
				end--
			}
		}
		sb.Write(b[i:end])
		written += end - i
		i = end
	}
	return sb.String()
}

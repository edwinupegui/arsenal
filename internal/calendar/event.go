package calendar

import (
	"fmt"
	"strings"
	"time"
)

// Recurrence describes how a calendar event repeats.
type Recurrence string

const (
	RecurrenceNone    Recurrence = "none"
	RecurrenceDaily   Recurrence = "daily"
	RecurrenceWeekly  Recurrence = "weekly"
	RecurrenceMonthly Recurrence = "monthly"
	RecurrenceYearly  Recurrence = "yearly" // calendar-specific per ADR-0001 (birthdays/anniversaries)
)

// Valid reports whether r is a known recurrence value.
func (r Recurrence) Valid() bool {
	switch r {
	case RecurrenceNone, RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly, RecurrenceYearly:
		return true
	}
	return false
}

// String returns the underlying string value.
func (r Recurrence) String() string { return string(r) }

// AllRecurrences returns every known recurrence in declaration order.
func AllRecurrences() []Recurrence {
	return []Recurrence{
		RecurrenceNone,
		RecurrenceDaily,
		RecurrenceWeekly,
		RecurrenceMonthly,
		RecurrenceYearly,
	}
}

// CreateInput captures everything needed to insert or update an event.
type CreateInput struct {
	Title       string
	Description string
	StartAt     string     // 'YYYY-MM-DDTHH:MM:SS' (timed) or 'YYYY-MM-DD' (all-day)
	EndAt       string     // empty = NULL/open-ended
	AllDay      bool
	Location    string
	CategoryID  *int64
	Notes       string
	Recurrence  Recurrence
	Tags        []string
}

// Filter drives the dynamic listing query.
type Filter struct {
	From         *string // start_at lower bound (inclusive); ISO date or datetime
	To           *string // start_at upper bound (inclusive)
	Recurrence   *string // exact match
	CategorySlug string
	TagName      string
	Trashed      bool
	Limit        int
	Offset       int
	Search       string // FTS5 query; when set, delegates to SearchCalendar
}

// ExportRow is one resolved event for iCal export.
type ExportRow struct {
	ID          int64
	Title       string
	Description string
	StartAt     string
	EndAt       string // empty when NULL
	AllDay      bool
	Location    string
	Category    string
	Notes       string
	Recurrence  Recurrence
	Tags        []string
	CreatedAt   string // for DTSTAMP / UID
}

const (
	layoutDate     = "2006-01-02"
	layoutDatetime = "2006-01-02T15:04:05"
)

// validateCreate enforces domain invariants before any database write.
//   - Title required (non-empty after trim)
//   - Recurrence required + Valid()
//   - StartAt required + parseable as date (all_day) or datetime (timed)
//   - When AllDay: StartAt must be date-only 'YYYY-MM-DD'; EndAt (if set) date-only
//   - When timed: StartAt must be 'YYYY-MM-DDTHH:MM:SS'; EndAt (if set) datetime
//   - When EndAt set: EndAt >= StartAt (string comparison is valid for same format)
func validateCreate(in CreateInput) error {
	if strings.TrimSpace(in.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if in.Recurrence == "" {
		return fmt.Errorf("recurrence is required")
	}
	if !in.Recurrence.Valid() {
		return fmt.Errorf("invalid recurrence %q", in.Recurrence)
	}
	if in.StartAt == "" {
		return fmt.Errorf("start_at is required")
	}

	if in.AllDay {
		// all_day=1: start_at must be date-only YYYY-MM-DD
		if _, err := time.Parse(layoutDate, in.StartAt); err != nil {
			return fmt.Errorf("all_day=true requires start_at in 'YYYY-MM-DD' format, got %q", in.StartAt)
		}
		if in.EndAt != "" {
			if _, err := time.Parse(layoutDate, in.EndAt); err != nil {
				return fmt.Errorf("all_day=true requires end_at in 'YYYY-MM-DD' format, got %q", in.EndAt)
			}
		}
	} else {
		// all_day=0: start_at must be datetime YYYY-MM-DDTHH:MM:SS
		if _, err := time.Parse(layoutDatetime, in.StartAt); err != nil {
			return fmt.Errorf("all_day=false requires start_at in 'YYYY-MM-DDTHH:MM:SS' format, got %q", in.StartAt)
		}
		if in.EndAt != "" {
			if _, err := time.Parse(layoutDatetime, in.EndAt); err != nil {
				return fmt.Errorf("all_day=false requires end_at in 'YYYY-MM-DDTHH:MM:SS' format, got %q", in.EndAt)
			}
		}
	}

	// EndAt >= StartAt when both are set (string comparison is valid within same format)
	if in.EndAt != "" && in.EndAt < in.StartAt {
		return fmt.Errorf("end_at %q must not be before start_at %q", in.EndAt, in.StartAt)
	}
	return nil
}

// boolToInt converts a bool to the SQLite integer representation.
func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

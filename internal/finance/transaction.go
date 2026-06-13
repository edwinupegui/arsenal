package finance

import (
	"fmt"
	"time"
)

// Kind describes the direction of a finance transaction.
type Kind string

const (
	KindExpense Kind = "expense"
	KindIncome  Kind = "income"
)

// Valid reports whether k is a known kind value.
func (k Kind) Valid() bool {
	switch k {
	case KindExpense, KindIncome:
		return true
	}
	return false
}

// String returns the underlying string value.
func (k Kind) String() string { return string(k) }

// AllKinds returns every known kind in declaration order.
func AllKinds() []Kind { return []Kind{KindExpense, KindIncome} }

// Recurrence describes how a transaction repeats.
type Recurrence string

const (
	RecurrenceNone    Recurrence = "none"
	RecurrenceDaily   Recurrence = "daily"
	RecurrenceWeekly  Recurrence = "weekly"
	RecurrenceMonthly Recurrence = "monthly"
)

// Valid reports whether r is a known recurrence value.
func (r Recurrence) Valid() bool {
	switch r {
	case RecurrenceNone, RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly:
		return true
	}
	return false
}

// String returns the underlying string value.
func (r Recurrence) String() string { return string(r) }

// AllRecurrences returns every known recurrence in declaration order.
func AllRecurrences() []Recurrence {
	return []Recurrence{RecurrenceNone, RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly}
}

// CreateInput captures everything needed to insert or update a transaction.
type CreateInput struct {
	Date       string
	Amount     float64
	Kind       Kind
	Account    string
	CategoryID *int64
	Notes      string
	Recurrence Recurrence
	Tags       []string
}

// Filter drives the dynamic listing query.
type Filter struct {
	From         *string // ISO date lower bound (inclusive)
	To           *string // ISO date upper bound (inclusive)
	Kind         *string // exact match
	CategorySlug string
	TagName      string
	Trashed      bool
	Limit        int
	Offset       int
	Search       string // FTS5 query; when set, delegates to SearchFinance
}

// ExportRow is one line of a CSV or JSON export.
type ExportRow struct {
	Date, Kind, Currency, Account, Category, Notes string
	Amount                                         float64
	Tags                                           []string
}

// validateCreate checks the input before it reaches the store.
func validateCreate(in CreateInput) error {
	if in.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if !in.Kind.Valid() {
		return fmt.Errorf("invalid kind %q", in.Kind)
	}
	if in.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if in.Recurrence == "" {
		return fmt.Errorf("recurrence is required")
	}
	if !in.Recurrence.Valid() {
		return fmt.Errorf("invalid recurrence %q", in.Recurrence)
	}
	if in.Date != "" {
		if _, err := time.Parse("2006-01-02", in.Date); err != nil {
			return fmt.Errorf("invalid date %q: %w", in.Date, err)
		}
	}
	return nil
}

package todos

// Priority describes todo urgency.
type Priority string

const (
	PriorityLow  Priority = "low"
	PriorityMed  Priority = "med"
	PriorityHigh Priority = "high"
)

// Valid reports whether p is a known priority value.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMed, PriorityHigh:
		return true
	}
	return false
}

// String returns the underlying string value.
func (p Priority) String() string { return string(p) }

// AllPriorities returns every known priority in declaration order.
func AllPriorities() []Priority {
	return []Priority{PriorityLow, PriorityMed, PriorityHigh}
}

// Status describes whether a todo is open or done.
type Status string

const (
	StatusOpen Status = "open"
	StatusDone Status = "done"
)

// Valid reports whether s is a known status value.
func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusDone:
		return true
	}
	return false
}

// String returns the underlying string value.
func (s Status) String() string { return string(s) }

// AllStatuses returns every known status in declaration order.
func AllStatuses() []Status {
	return []Status{StatusOpen, StatusDone}
}

// Recurrence describes how a todo repeats.
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

// CreateInput captures everything needed to insert a new todo.
type CreateInput struct {
	Title       string
	Description string
	Priority    Priority
	DueDate     *string
	CategoryID  *int64
	Notes       string
	Recurrence  Recurrence
	Tags        []string
}

// ListFilter drives the dynamic listing query.
type ListFilter struct {
	CategorySlug string
	TagName      string
	Status       Status
	Priority     Priority
	OnlyOverdue  bool
	DueBefore    string
	DueAfter     string // ISO-8601 date; rows with due_date < DueAfter are excluded
	Trashed      bool
	Limit        int
	Offset       int
	Search       string // FTS5 query; when set, delegates to SearchTodos
}

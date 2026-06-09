package todos

import (
	"testing"
)

func TestPriority_Valid(t *testing.T) {
	cases := []struct {
		name  string
		value Priority
		want  bool
	}{
		{"low", PriorityLow, true},
		{"med", PriorityMed, true},
		{"high", PriorityHigh, true},
		{"empty", "", false},
		{"invalid", "critical", false},
		{"mixed case", "Low", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.value.Valid(); got != tc.want {
				t.Errorf("Valid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPriority_String(t *testing.T) {
	cases := []struct {
		value Priority
		want  string
	}{
		{PriorityLow, "low"},
		{PriorityMed, "med"},
		{PriorityHigh, "high"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.value.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAllPriorities(t *testing.T) {
	got := AllPriorities()
	want := []Priority{PriorityLow, PriorityMed, PriorityHigh}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStatus_Valid(t *testing.T) {
	cases := []struct {
		name  string
		value Status
		want  bool
	}{
		{"open", StatusOpen, true},
		{"done", StatusDone, true},
		{"empty", "", false},
		{"invalid", "in_progress", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.value.Valid(); got != tc.want {
				t.Errorf("Valid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStatus_String(t *testing.T) {
	cases := []struct {
		value Status
		want string
	}{
		{StatusOpen, "open"},
		{StatusDone, "done"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.value.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAllStatuses(t *testing.T) {
	got := AllStatuses()
	want := []Status{StatusOpen, StatusDone}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRecurrence_Valid(t *testing.T) {
	cases := []struct {
		name  string
		value Recurrence
		want  bool
	}{
		{"none", RecurrenceNone, true},
		{"daily", RecurrenceDaily, true},
		{"weekly", RecurrenceWeekly, true},
		{"monthly", RecurrenceMonthly, true},
		{"empty", "", false},
		{"invalid", "yearly", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.value.Valid(); got != tc.want {
				t.Errorf("Valid() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRecurrence_String(t *testing.T) {
	cases := []struct {
		value Recurrence
		want  string
	}{
		{RecurrenceNone, "none"},
		{RecurrenceDaily, "daily"},
		{RecurrenceWeekly, "weekly"},
		{RecurrenceMonthly, "monthly"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.value.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAllRecurrences(t *testing.T) {
	got := AllRecurrences()
	want := []Recurrence{RecurrenceNone, RecurrenceDaily, RecurrenceWeekly, RecurrenceMonthly}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

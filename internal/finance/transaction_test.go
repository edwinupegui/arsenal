package finance

import (
	"strings"
	"testing"
)

func TestKind_Valid(t *testing.T) {
	cases := []struct {
		name string
		k    Kind
		want bool
	}{
		{"expense", KindExpense, true},
		{"income", KindIncome, true},
		{"transfer", Kind("transfer"), false},
		{"empty", Kind(""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.k.Valid(); got != tc.want {
				t.Errorf("%q.Valid() = %v, want %v", tc.k, got, tc.want)
			}
		})
	}
}

func TestKind_String(t *testing.T) {
	if got := KindExpense.String(); got != "expense" {
		t.Errorf("KindExpense.String() = %q, want expense", got)
	}
}

func TestKind_AllKinds(t *testing.T) {
	got := AllKinds()
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != KindExpense || got[1] != KindIncome {
		t.Errorf("AllKinds() = %v", got)
	}
}

func TestRecurrence_Valid(t *testing.T) {
	cases := []struct {
		name string
		r    Recurrence
		want bool
	}{
		{"none", RecurrenceNone, true},
		{"daily", RecurrenceDaily, true},
		{"weekly", RecurrenceWeekly, true},
		{"monthly", RecurrenceMonthly, true},
		{"yearly", Recurrence("yearly"), false},
		{"empty", Recurrence(""), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.r.Valid(); got != tc.want {
				t.Errorf("%q.Valid() = %v, want %v", tc.r, got, tc.want)
			}
		})
	}
}

func TestRecurrence_AllRecurrences(t *testing.T) {
	got := AllRecurrences()
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
}

func TestValidateCreate(t *testing.T) {
	base := CreateInput{
		Date:       "2026-06-13",
		Amount:     10.00,
		Kind:       KindExpense,
		Account:    "checking",
		Recurrence: RecurrenceNone,
	}

	cases := []struct {
		name   string
		mutate func(*CreateInput)
	}{
		{"missing kind", func(in *CreateInput) { in.Kind = "" }},
		{"invalid kind", func(in *CreateInput) { in.Kind = "transfer" }},
		{"zero amount", func(in *CreateInput) { in.Amount = 0 }},
		{"negative amount", func(in *CreateInput) { in.Amount = -5 }},
		{"missing recurrence", func(in *CreateInput) { in.Recurrence = "" }},
		{"invalid recurrence", func(in *CreateInput) { in.Recurrence = "yearly" }},
		{"bad date", func(in *CreateInput) { in.Date = "not-a-date" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.mutate(&in)
			if err := validateCreate(in); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateCreate_Valid(t *testing.T) {
	in := CreateInput{
		Date:       "2026-06-13",
		Amount:     10.00,
		Kind:       KindExpense,
		Account:    "checking",
		Recurrence: RecurrenceNone,
	}
	if err := validateCreate(in); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestValidateCreate_EmptyDateAllowed(t *testing.T) {
	in := CreateInput{
		Amount:     10.00,
		Kind:       KindExpense,
		Recurrence: RecurrenceNone,
	}
	if err := validateCreate(in); err != nil {
		t.Fatalf("expected empty date to be allowed, got %v", err)
	}
}

func TestExportRow_Tags(t *testing.T) {
	row := ExportRow{
		Date:     "2026-06-13",
		Kind:     "expense",
		Amount:   10.00,
		Currency: "USD",
		Tags:     []string{"a", "b"},
	}
	if !strings.Contains(strings.Join(row.Tags, ","), "a") {
		t.Error("expected tags to contain a")
	}
}

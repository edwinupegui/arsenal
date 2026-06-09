package cli

import (
	"testing"

	"github.com/edwinupegui/arsenal/internal/todos"
)

func TestTodoPriorityValidation(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"low is valid", "low", false},
		{"med is valid", "med", false},
		{"high is valid", "high", false},
		{"empty is valid (default)", "", false},
		{"invalid fails", "urgent", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := todos.Priority(tt.input)
			if tt.wantErr {
				if p != "" && p.Valid() {
					t.Errorf("expected priority %q to be invalid", tt.input)
				}
				return
			}
			if !p.Valid() && tt.input != "" {
				t.Errorf("expected priority %q to be valid", tt.input)
			}
		})
	}
}

func TestTodoRecurrenceValidation(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"none is valid", "none", false},
		{"daily is valid", "daily", false},
		{"weekly is valid", "weekly", false},
		{"monthly is valid", "monthly", false},
		{"empty is valid (default)", "", false},
		{"invalid fails", "yearly", true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := todos.Recurrence(tt.input)
			if tt.wantErr {
				if r != "" && r.Valid() {
					t.Errorf("expected recurrence %q to be invalid", tt.input)
				}
				return
			}
			if !r.Valid() && tt.input != "" {
				t.Errorf("expected recurrence %q to be valid", tt.input)
			}
		})
	}
}

func TestTodoStatusValidation(t *testing.T) {
	cases := []struct {
		name  string
		input string
		valid bool
	}{
		{"open is valid", "open", true},
		{"done is valid", "done", true},
		{"empty is valid (default)", "", true},
		{"invalid fails", "pending", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			s := todos.Status(tt.input)
			if tt.valid {
				if !s.Valid() && tt.input != "" {
					t.Errorf("expected status %q to be valid", tt.input)
				}
			} else {
				if s.Valid() {
					t.Errorf("expected status %q to be invalid", tt.input)
				}
			}
		})
	}
}

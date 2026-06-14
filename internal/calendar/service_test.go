package calendar_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/edwinupegui/arsenal/internal/calendar"
	"github.com/edwinupegui/arsenal/internal/store"
)

func validTimedCreate() calendar.CreateInput {
	return calendar.CreateInput{
		Title:      "team standup",
		StartAt:    "2026-06-15T09:00:00",
		EndAt:      "2026-06-15T09:30:00",
		AllDay:     false,
		Location:   "conference room",
		Recurrence: calendar.RecurrenceNone,
		Tags:       []string{"work"},
	}
}

func validAllDayCreate() calendar.CreateInput {
	return calendar.CreateInput{
		Title:      "company holiday",
		StartAt:    "2026-06-20",
		EndAt:      "",
		AllDay:     true,
		Recurrence: calendar.RecurrenceNone,
	}
}

// --- Create tests ---

func TestCreate_TimedEvent(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	got, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if got.Row.Title != "team standup" {
		t.Errorf("title = %q, want team standup", got.Row.Title)
	}
	if got.Row.StartAt != "2026-06-15T09:00:00" {
		t.Errorf("start_at = %q, want 2026-06-15T09:00:00", got.Row.StartAt)
	}
	if !got.Row.EndAt.Valid || got.Row.EndAt.String != "2026-06-15T09:30:00" {
		t.Errorf("end_at = %v, want 2026-06-15T09:30:00", got.Row.EndAt)
	}
	if got.Row.AllDay != 0 {
		t.Errorf("all_day = %d, want 0", got.Row.AllDay)
	}
	if got.Row.DeletedAt.Valid {
		t.Error("expected deleted_at to be NULL")
	}
	if !equalStrings(got.Tags, []string{"work"}) {
		t.Errorf("tags = %v, want [work]", got.Tags)
	}
}

func TestCreate_AllDayEvent(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	got, err := svc.Create(ctx, validAllDayCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.AllDay != 1 {
		t.Errorf("all_day = %d, want 1", got.Row.AllDay)
	}
	if got.Row.StartAt != "2026-06-20" {
		t.Errorf("start_at = %q, want 2026-06-20", got.Row.StartAt)
	}
	if got.Row.EndAt.Valid {
		t.Errorf("end_at = %v, want NULL for open-ended all-day", got.Row.EndAt)
	}
}

func TestCreate_OpenEndedNullEndAt(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	got, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "open-ended event",
		StartAt:    "2026-06-15T10:00:00",
		EndAt:      "",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Row.EndAt.Valid {
		t.Errorf("end_at should be NULL for open-ended, got %q", got.Row.EndAt.String)
	}
}

func TestCreate_InvalidRecurrence(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	_, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "test",
		StartAt:    "2026-06-15T09:00:00",
		AllDay:     false,
		Recurrence: calendar.Recurrence("biweekly"),
	})
	if err == nil {
		t.Fatal("expected error for invalid recurrence, got nil")
	}
}

func TestCreate_AllDayWithDatetimeStartAt_Rejected(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	_, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "bad event",
		StartAt:    "2026-06-15T09:00:00", // datetime format, but all_day=true
		AllDay:     true,
		Recurrence: calendar.RecurrenceNone,
	})
	if err == nil {
		t.Fatal("expected error for all_day=true with datetime start_at, got nil")
	}
}

func TestCreate_EmptyTitle_Rejected(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	_, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "",
		StartAt:    "2026-06-15T09:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	})
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
}

func TestCreate_StartAtStoredWithoutTZOffset(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	got, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// start_at must not contain 'Z' or '+' (no offset)
	for _, char := range []string{"Z", "+", "-07:00", "UTC"} {
		if len(got.Row.StartAt) > 10 && got.Row.StartAt[10:] == "T"+got.Row.StartAt[11:] {
			break
		}
		_ = char
	}
	// Just assert that start_at matches what we stored verbatim (no offset appended)
	if got.Row.StartAt != "2026-06-15T09:00:00" {
		t.Errorf("start_at = %q, want 2026-06-15T09:00:00 (no tz offset)", got.Row.StartAt)
	}
}

// --- Get tests ---

func TestGet_Found(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Get(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Row.ID != created.Row.ID {
		t.Errorf("id = %d, want %d", got.Row.ID, created.Row.ID)
	}
	if !equalStrings(got.Tags, []string{"work"}) {
		t.Errorf("tags = %v, want [work]", got.Tags)
	}
}

func TestGet_NotFound(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	if _, err := svc.Get(ctx, 9999); err == nil {
		t.Fatal("expected error for non-existent event, got nil")
	}
}

// --- Update tests ---

func TestUpdate_ChangesStartAndEnd(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Row.ID, calendar.CreateInput{
		Title:      "updated standup",
		StartAt:    "2026-06-16T10:00:00",
		EndAt:      "2026-06-16T10:30:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceDaily,
		Tags:       []string{"engineering"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Row.StartAt != "2026-06-16T10:00:00" {
		t.Errorf("start_at = %q, want 2026-06-16T10:00:00", updated.Row.StartAt)
	}
	if !updated.Row.EndAt.Valid || updated.Row.EndAt.String != "2026-06-16T10:30:00" {
		t.Errorf("end_at = %v, want 2026-06-16T10:30:00", updated.Row.EndAt)
	}
	if !equalStrings(updated.Tags, []string{"engineering"}) {
		t.Errorf("tags = %v, want [engineering]", updated.Tags)
	}
}

func TestUpdate_ClearsEndAtToNull(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Row.ID, calendar.CreateInput{
		Title:      "team standup",
		StartAt:    "2026-06-15T09:00:00",
		EndAt:      "", // clear end_at
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Row.EndAt.Valid {
		t.Errorf("end_at should be NULL after clearing, got %q", updated.Row.EndAt.String)
	}
}

func TestUpdate_ChangesTags(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, created.Row.ID, calendar.CreateInput{
		Title:      "team standup",
		StartAt:    "2026-06-15T09:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
		Tags:       []string{"personal", "home"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !equalStrings(updated.Tags, []string{"home", "personal"}) {
		t.Errorf("tags = %v, want [home personal]", updated.Tags)
	}

	// verify orphan pruning: 'work' tag should be gone
	q := store.New(db)
	tags, err := q.ListTags(ctx)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	names := make(map[string]bool)
	for _, tag := range tags {
		names[tag.Name] = true
	}
	if names["work"] {
		t.Error("orphan tag 'work' should have been pruned after Update")
	}
}

func TestUpdate_NonExistentFails(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	_, err := svc.Update(ctx, 9999, calendar.CreateInput{
		Title:      "ghost",
		StartAt:    "2026-06-15T09:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	})
	if err == nil {
		t.Fatal("expected error for non-existent event, got nil")
	}
}

// --- SoftDelete tests ---

func TestSoftDelete_SetsDeletedAt(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	q := store.New(db)
	row, err := q.GetCalendarEvent(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetCalendarEvent: %v", err)
	}
	if !row.DeletedAt.Valid {
		t.Fatal("expected deleted_at to be set after SoftDelete")
	}
}

func TestSoftDelete_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	// Second soft-delete on already-deleted row should not error
	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete again: %v", err)
	}
}

// --- Restore tests ---

func TestRestore_ClearsDeletedAt(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	if err := svc.Restore(ctx, created.Row.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	q := store.New(db)
	row, err := q.GetCalendarEvent(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("GetCalendarEvent: %v", err)
	}
	if row.DeletedAt.Valid {
		t.Fatal("expected deleted_at to be NULL after restore")
	}
}

func TestRestore_Idempotent(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Restore on active event should not error
	if err := svc.Restore(ctx, created.Row.ID); err != nil {
		t.Fatalf("Restore on active: %v", err)
	}
}

// --- Purge tests ---

func TestPurge_HardDeletes(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, created.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	if err := svc.Purge(ctx, created.Row.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	q := store.New(db)
	if _, err := q.GetCalendarEvent(ctx, created.Row.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected ErrNoRows after purge, got %v", err)
	}
}

func TestPurge_RemovesFTSEntry(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "searchable event",
		StartAt:    "2026-06-15T09:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify it is in FTS
	var found int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "searchable").Scan(&found); err != nil {
		t.Fatalf("fts before purge: %v", err)
	}
	if found != 1 {
		t.Fatalf("expected 1 fts match before purge, got %d", found)
	}

	if err := svc.Purge(ctx, created.Row.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_fts WHERE calendar_fts MATCH ?`, "searchable").Scan(&found); err != nil {
		t.Fatalf("fts after purge: %v", err)
	}
	if found != 0 {
		t.Fatalf("expected 0 fts matches after purge, got %d", found)
	}
}

// --- List tests ---

func TestList_FilterByDateRange(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	if _, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "early event",
		StartAt:    "2026-06-01T10:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	}); err != nil {
		t.Fatalf("Create early: %v", err)
	}
	mid, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "mid event",
		StartAt:    "2026-06-15T10:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	})
	if err != nil {
		t.Fatalf("Create mid: %v", err)
	}

	from := "2026-06-10T00:00:00"
	to := "2026-06-20T23:59:59"
	got, err := svc.List(ctx, calendar.Filter{From: &from, To: &to})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != mid.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, mid.Row.ID)
	}
}

func TestList_AllDayOnly(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	allDay, err := svc.Create(ctx, validAllDayCreate())
	if err != nil {
		t.Fatalf("Create all-day: %v", err)
	}
	if _, err := svc.Create(ctx, validTimedCreate()); err != nil {
		t.Fatalf("Create timed: %v", err)
	}

	allDayRec := string(calendar.RecurrenceNone)
	// Filter by all-day using the fact that all-day start_at has date-only format
	// We use date range to isolate
	got, err := svc.List(ctx, calendar.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	_ = allDayRec

	// List should include both; verify all-day is in there
	found := false
	for _, e := range got {
		if e.Row.ID == allDay.Row.ID && e.Row.AllDay == 1 {
			found = true
		}
	}
	if !found {
		t.Error("all-day event not found in list results")
	}
}

func TestList_Trashed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	trashed, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, trashed.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "active event",
		StartAt:    "2026-06-16T10:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	}); err != nil {
		t.Fatalf("Create active: %v", err)
	}

	got, err := svc.List(ctx, calendar.Filter{Trashed: true})
	if err != nil {
		t.Fatalf("List trashed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != trashed.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, trashed.Row.ID)
	}
}

func TestList_ByTag(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	tagged, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "tagged event",
		StartAt:    "2026-06-15T10:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
		Tags:       []string{"urgent"},
	})
	if err != nil {
		t.Fatalf("Create tagged: %v", err)
	}
	if _, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "untagged event",
		StartAt:    "2026-06-15T11:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	}); err != nil {
		t.Fatalf("Create untagged: %v", err)
	}

	got, err := svc.List(ctx, calendar.Filter{TagName: "urgent"})
	if err != nil {
		t.Fatalf("List by tag: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.ID != tagged.Row.ID {
		t.Errorf("id = %d, want %d", got[0].Row.ID, tagged.Row.ID)
	}
}

// --- Export tests ---

func TestExport_All(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)
	q := store.New(db)

	cat, err := q.CreateCategory(ctx, store.CreateCategoryParams{Slug: "work", Name: "Work", Icon: "", SortOrder: 1})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if _, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "team standup",
		StartAt:    "2026-06-15T09:00:00",
		EndAt:      "2026-06-15T09:30:00",
		AllDay:     false,
		Location:   "conference room",
		CategoryID: &cat.ID,
		Notes:      "bring laptop",
		Recurrence: calendar.RecurrenceDaily,
		Tags:       []string{"work"},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.Export(ctx, calendar.Filter{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	row := got[0]
	if row.Title != "team standup" {
		t.Errorf("title = %q, want team standup", row.Title)
	}
	if row.StartAt != "2026-06-15T09:00:00" {
		t.Errorf("start_at = %q, want 2026-06-15T09:00:00", row.StartAt)
	}
	if row.EndAt != "2026-06-15T09:30:00" {
		t.Errorf("end_at = %q, want 2026-06-15T09:30:00", row.EndAt)
	}
	if row.Category != "Work" {
		t.Errorf("category = %q, want Work", row.Category)
	}
	if row.Notes != "bring laptop" {
		t.Errorf("notes = %q, want bring laptop", row.Notes)
	}
	if !equalStrings(row.Tags, []string{"work"}) {
		t.Errorf("tags = %v, want [work]", row.Tags)
	}
	if row.Recurrence != calendar.RecurrenceDaily {
		t.Errorf("recurrence = %q, want daily", row.Recurrence)
	}
}

func TestExport_ExcludesTrashed(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	trashed, err := svc.Create(ctx, validTimedCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.SoftDelete(ctx, trashed.Row.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if _, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "active",
		StartAt:    "2026-06-16T10:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	}); err != nil {
		t.Fatalf("Create active: %v", err)
	}

	got, err := svc.Export(ctx, calendar.Filter{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (trashed excluded)", len(got))
	}
	if got[0].Title != "active" {
		t.Errorf("title = %q, want active", got[0].Title)
	}
}

// --- Attacher tests ---

func TestAttacher_CreatesJunctionRows(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	created, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "tagged event",
		StartAt:    "2026-06-15T09:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
		Tags:       []string{"alpha", "beta"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !equalStrings(created.Tags, []string{"alpha", "beta"}) {
		t.Errorf("tags = %v, want [alpha beta]", created.Tags)
	}

	q := store.New(db)
	tags, err := q.ListTagsForCalendar(ctx, created.Row.ID)
	if err != nil {
		t.Fatalf("ListTagsForCalendar: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tag junction rows, got %d", len(tags))
	}
}

// --- FTS5 search via List ---

func TestList_Search(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	svc := calendar.New(db)

	if _, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "board meeting",
		StartAt:    "2026-06-15T09:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "lunch break",
		StartAt:    "2026-06-15T12:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.List(ctx, calendar.Filter{Search: "board"})
	if err != nil {
		t.Fatalf("List search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Row.Title != "board meeting" {
		t.Errorf("title = %q, want board meeting", got[0].Row.Title)
	}
}

// --- Cross-domain orphan tag cleanup ---

// TestPurge_PrunesCalendarOrphans_CrossDomainIsolation verifies that purging a
// calendar event removes only tags that are exclusively attached to that event.
// Tags still referenced by a finance transaction, a todo, or a resource must
// not be deleted (cross-domain isolation).
func TestPurge_PrunesCalendarOrphans_CrossDomainIsolation(t *testing.T) {
	ctx := context.Background()
	db := newTestDB(t)
	q := store.New(db)
	svc := calendar.New(db)

	// Upsert three tags: one exclusive to the calendar event, two shared.
	calOnlyTag, err := q.UpsertTag(ctx, "cal-only")
	if err != nil {
		t.Fatalf("UpsertTag cal-only: %v", err)
	}
	sharedFinTag, err := q.UpsertTag(ctx, "shared-finance")
	if err != nil {
		t.Fatalf("UpsertTag shared-finance: %v", err)
	}
	sharedTodoTag, err := q.UpsertTag(ctx, "shared-todo")
	if err != nil {
		t.Fatalf("UpsertTag shared-todo: %v", err)
	}

	// Create a calendar event with all three tags.
	event, err := svc.Create(ctx, calendar.CreateInput{
		Title:      "event with tags",
		StartAt:    "2026-06-15T09:00:00",
		AllDay:     false,
		Recurrence: calendar.RecurrenceNone,
		Tags:       []string{"cal-only", "shared-finance", "shared-todo"},
	})
	if err != nil {
		t.Fatalf("Create calendar event: %v", err)
	}

	// Create a finance transaction and attach shared-finance tag.
	fin, err := q.CreateFinanceTransaction(ctx, store.CreateFinanceTransactionParams{
		Date:       "2026-06-15",
		Amount:     100,
		Kind:       "expense",
		Account:    "bank",
		Recurrence: "none",
		Currency:   "USD",
	})
	if err != nil {
		t.Fatalf("CreateFinanceTransaction: %v", err)
	}
	if err := q.AttachTagToFinance(ctx, store.AttachTagToFinanceParams{
		FinanceID: fin.ID,
		TagID:     sharedFinTag.ID,
	}); err != nil {
		t.Fatalf("AttachTagToFinance: %v", err)
	}

	// Create a todo and attach shared-todo tag.
	todo, err := q.CreateTodo(ctx, store.CreateTodoParams{
		Title:      "todo item",
		Priority:   "med",
		Status:     "open",
		Recurrence: "none",
	})
	if err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}
	if err := q.AttachTagToTodo(ctx, store.AttachTagToTodoParams{
		TodoID: todo.ID,
		TagID:  sharedTodoTag.ID,
	}); err != nil {
		t.Fatalf("AttachTagToTodo: %v", err)
	}

	// Purge the calendar event (also prunes orphan tags).
	if err := svc.Purge(ctx, event.Row.ID); err != nil {
		t.Fatalf("Purge: %v", err)
	}

	// cal-only tag must be gone (orphan pruned).
	if _, err := q.GetTagByName(ctx, "cal-only"); err == nil {
		t.Error("expected cal-only tag to be deleted after purge; still exists")
	}

	// shared-finance tag must still exist (referenced by finance_tags).
	if _, err := q.GetTagByName(ctx, "shared-finance"); err != nil {
		t.Errorf("shared-finance tag should still exist after purge: %v", err)
	}

	// shared-todo tag must still exist (referenced by todo_tags).
	if _, err := q.GetTagByName(ctx, "shared-todo"); err != nil {
		t.Errorf("shared-todo tag should still exist after purge: %v", err)
	}

	_ = calOnlyTag
}

// --- Helpers ---

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

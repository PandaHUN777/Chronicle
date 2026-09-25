// export_session_time_roundtrip_test.go proves a session's scheduled time of
// day survives a campaign export and re-import: it fails (time lost) if
// campaigns.ExportSession drops ScheduledTime or either adapter stops
// plumbing it through (keyxmakerx/Chronicle#615).
package app

import (
	"context"
	"testing"

	"github.com/keyxmakerx/chronicle/internal/plugins/campaigns"
	"github.com/keyxmakerx/chronicle/internal/plugins/sessions"
)

// fakeSessionService is an in-memory sessions.SessionService. It embeds the
// interface so only the methods the export/import adapters actually call
// need bodies. UpdateSession applies the same absent/null/value merge the
// real service does, so the test can assert on the record the import leaves
// behind, not just the call arguments.
type fakeSessionService struct {
	sessions.SessionService

	list   []sessions.Session // What ListSessions returns.
	byID   map[string]*sessions.Session
	nextID int
}

func newFakeSessionService(list ...sessions.Session) *fakeSessionService {
	return &fakeSessionService{list: list, byID: map[string]*sessions.Session{}}
}

func (f *fakeSessionService) ListSessions(_ context.Context, _ string) ([]sessions.Session, error) {
	return f.list, nil
}

func (f *fakeSessionService) CreateSession(_ context.Context, campaignID string, in sessions.CreateSessionInput) (*sessions.Session, error) {
	f.nextID++
	s := &sessions.Session{
		ID:            string(rune('a'+f.nextID-1)) + "-new",
		CampaignID:    campaignID,
		Name:          in.Name,
		ScheduledDate: in.ScheduledDate,
		ScheduledTime: in.ScheduledTime,
		CalendarYear:  in.CalendarYear,
		CalendarMonth: in.CalendarMonth,
		CalendarDay:   in.CalendarDay,
		Status:        sessions.StatusPlanned,
	}
	f.byID[s.ID] = s
	return s, nil
}

func (f *fakeSessionService) UpdateSession(_ context.Context, id string, in sessions.UpdateSessionInput) (*sessions.Session, error) {
	s, ok := f.byID[id]
	if !ok {
		return nil, nil
	}
	s.Name = in.Name.Val(s.Name)
	s.ScheduledDate = in.ScheduledDate.Ptr(s.ScheduledDate)
	s.ScheduledTime = in.ScheduledTime.Ptr(s.ScheduledTime)
	s.CalendarYear = in.CalendarYear.Ptr(s.CalendarYear)
	s.CalendarMonth = in.CalendarMonth.Ptr(s.CalendarMonth)
	s.CalendarDay = in.CalendarDay.Ptr(s.CalendarDay)
	s.Status = in.Status.Val(s.Status)
	return s, nil
}

func (f *fakeSessionService) UpdateSessionRecap(_ context.Context, _ string, _, _ *string) error {
	return nil
}

func (f *fakeSessionService) ListAttendees(_ context.Context, _ string) ([]sessions.Attendee, error) {
	return nil, nil
}

func (f *fakeSessionService) LinkEntity(_ context.Context, _, _, _, _ string) error   { return nil }
func (f *fakeSessionService) InviteAll(_ context.Context, _ string, _ []string) error { return nil }
func (f *fakeSessionService) UpdateRSVP(_ context.Context, _, _, _ string) error      { return nil }

// TestCampaignExportImport_SessionScheduledTimeRoundTrip is the regression: a
// confirmed session's wall-clock time must survive export and re-import
// alongside its date, not just the date.
func TestCampaignExportImport_SessionScheduledTimeRoundTrip(t *testing.T) {
	date := "2028-03-08"
	clock := "19:30"
	src := newFakeSessionService(sessions.Session{
		ID:            "s1",
		Name:          "The Duke's Ball",
		Status:        sessions.StatusPlanned,
		ScheduledDate: &date,
		ScheduledTime: &clock,
	})

	a := &sessionExportAdapter{svc: src}
	out, err := a.ExportSessions(context.Background(), "c1", func(string) string { return "" })
	if err != nil {
		t.Fatalf("export sessions: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d sessions, want 1", len(out))
	}
	if out[0].ScheduledDate == nil || *out[0].ScheduledDate != date {
		t.Fatalf("exported ScheduledDate = %v, want %q", out[0].ScheduledDate, date)
	}
	if out[0].ScheduledTime == nil || *out[0].ScheduledTime != clock {
		t.Fatalf("exported ScheduledTime = %v, want %q — the session's scheduled time was dropped from the export", out[0].ScheduledTime, clock)
	}

	// --- Import side ---
	dst := newFakeSessionService()
	imp := &sessionImportAdapter{svc: dst}
	report := campaigns.NewImportReport()
	if err := imp.ImportSessions(context.Background(), "c2", "user-1", out, campaigns.NewIDMap("c2"), report); err != nil {
		t.Fatalf("import sessions: %v", err)
	}
	if report.HasFailures() {
		t.Fatalf("clean session import reported failures: %s", report.Summary())
	}
	if len(dst.byID) != 1 {
		t.Fatalf("imported %d sessions, want 1", len(dst.byID))
	}
	var imported *sessions.Session
	for _, s := range dst.byID {
		imported = s
	}
	if imported.ScheduledDate == nil || *imported.ScheduledDate != date {
		t.Errorf("imported ScheduledDate = %v, want %q", imported.ScheduledDate, date)
	}
	if imported.ScheduledTime == nil || *imported.ScheduledTime != clock {
		t.Errorf("imported ScheduledTime = %v, want %q — the session's scheduled time was lost on import", imported.ScheduledTime, clock)
	}
}

// TestSessionImportAdapter_OlderExportWithoutScheduledTime confirms an export
// file written before ScheduledTime existed (the field simply absent from its
// JSON, so it unmarshals as a nil pointer) still imports cleanly with no time
// set, rather than erroring or panicking.
func TestSessionImportAdapter_OlderExportWithoutScheduledTime(t *testing.T) {
	date := "2028-03-08"
	dst := newFakeSessionService()
	imp := &sessionImportAdapter{svc: dst}
	report := campaigns.NewImportReport()

	older := []campaigns.ExportSession{{
		Name:          "Pre-#615 Session",
		Status:        sessions.StatusPlanned,
		ScheduledDate: &date,
		// ScheduledTime intentionally omitted: an older export file has no
		// such field at all.
	}}

	if err := imp.ImportSessions(context.Background(), "c2", "user-1", older, campaigns.NewIDMap("c2"), report); err != nil {
		t.Fatalf("import sessions: %v", err)
	}
	if report.HasFailures() {
		t.Fatalf("older-export session import reported failures: %s", report.Summary())
	}
	if len(dst.byID) != 1 {
		t.Fatalf("imported %d sessions, want 1", len(dst.byID))
	}
	var imported *sessions.Session
	for _, s := range dst.byID {
		imported = s
	}
	if imported.ScheduledTime != nil {
		t.Errorf("imported ScheduledTime = %v, want nil for an older export that never carried one", *imported.ScheduledTime)
	}
	if imported.ScheduledDate == nil || *imported.ScheduledDate != date {
		t.Errorf("imported ScheduledDate = %v, want %q — unrelated to the time field", imported.ScheduledDate, date)
	}
}

package handlers

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateICS_ProducesValidCalendarStructure(t *testing.T) {
	start := time.Date(2026, 8, 1, 14, 0, 0, 0, time.UTC)
	end := start.Add(1 * time.Hour)

	ics := generateICS([]struct {
		UID         string
		Start       time.Time
		End         time.Time
		Summary     string
		Description string
		Location    string
	}{
		{
			UID:         "test-booking-id",
			Start:       start,
			End:         end,
			Summary:     "Spanish class with Jane Doe",
			Description: "Language Tutor class with Jane Doe and John Smith",
			Location:    "https://meet.example.com/xyz",
		},
	})

	requiredSubstrings := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"BEGIN:VEVENT",
		"UID:test-booking-id@languagetutor",
		"DTSTART:20260801T140000Z",
		"DTEND:20260801T150000Z",
		"SUMMARY:Spanish class with Jane Doe",
		"LOCATION:https://meet.example.com/xyz",
		"END:VEVENT",
		"END:VCALENDAR",
	}

	for _, substr := range requiredSubstrings {
		if !strings.Contains(ics, substr) {
			t.Errorf("expected ICS output to contain %q, got:\n%s", substr, ics)
		}
	}

	if !strings.HasSuffix(ics, "END:VCALENDAR\r\n") {
		t.Errorf("expected ICS output to end with END:VCALENDAR, got suffix: %q", ics[len(ics)-30:])
	}
}

func TestGenerateICS_MultipleEventsAndEmptyList(t *testing.T) {
	empty := generateICS(nil)
	if !strings.Contains(empty, "BEGIN:VCALENDAR") || strings.Contains(empty, "BEGIN:VEVENT") {
		t.Errorf("expected an empty event list to still produce a valid calendar with no events, got:\n%s", empty)
	}

	now := time.Now().UTC()
	multi := generateICS([]struct {
		UID         string
		Start       time.Time
		End         time.Time
		Summary     string
		Description string
		Location    string
	}{
		{UID: "a", Start: now, End: now.Add(time.Hour), Summary: "First"},
		{UID: "b", Start: now, End: now.Add(time.Hour), Summary: "Second"},
	})

	if strings.Count(multi, "BEGIN:VEVENT") != 2 {
		t.Errorf("expected 2 VEVENT blocks for 2 input events, got:\n%s", multi)
	}
}

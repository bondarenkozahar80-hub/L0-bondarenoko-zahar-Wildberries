package calendar

import (
	"testing"
	"time"
)

func TestCalendar_CreateEvent(t *testing.T) {
	c := New()
	date := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)

	event, err := c.CreateEvent(1, date, "New Year's Eve")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.ID != 1 {
		t.Errorf("expected event ID 1, got %d", event.ID)
	}
	if event.UserID != 1 {
		t.Errorf("expected user ID 1, got %d", event.UserID)
	}
	if event.Title != "New Year's Eve" {
		t.Errorf("expected title 'New Year's Eve', got %s", event.Title)
	}
}

func TestCalendar_CreateEvent_EmptyTitle(t *testing.T) {
	c := New()
	date := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)

	_, err := c.CreateEvent(1, date, "")
	if err == nil {
		t.Error("expected error for empty title, got nil")
	}
}

func TestCalendar_UpdateEvent(t *testing.T) {
	c := New()
	date := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)

	event, _ := c.CreateEvent(1, date, "Old Title")
	newDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	updatedEvent, err := c.UpdateEvent(event.ID, 1, newDate, "New Title")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updatedEvent.Date != newDate {
		t.Errorf("expected date %v, got %v", newDate, updatedEvent.Date)
	}
	if updatedEvent.Title != "New Title" {
		t.Errorf("expected title 'New Title', got %s", updatedEvent.Title)
	}
}

func TestCalendar_DeleteEvent(t *testing.T) {
	c := New()
	date := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)

	event, _ := c.CreateEvent(1, date, "Test Event")

	err := c.DeleteEvent(event.ID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to delete again - should fail
	err = c.DeleteEvent(event.ID, 1)
	if err == nil {
		t.Error("expected error when deleting non-existent event")
	}
}

func TestCalendar_GetEventsForDay(t *testing.T) {
	c := New()
	date := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)

	c.CreateEvent(1, date, "Event 1")
	c.CreateEvent(1, date, "Event 2")
	c.CreateEvent(2, date, "Event 3") // Different user
	c.CreateEvent(1, date.AddDate(0, 0, 1), "Event 4") // Different day

	events := c.GetEventsForDay(1, date)
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

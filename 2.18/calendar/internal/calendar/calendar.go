package calendar

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// Event представляет собой событие в календаре
type Event struct {
	ID     int       `json:"id"`
	UserID int       `json:"user_id"`
	Date   time.Time `json:"date"`
	Title  string    `json:"title"`
}

// Calendar представляет собой календарь событий
type Calendar struct {
	mu      sync.RWMutex
	events  map[int]Event // map[eventID]Event
	lastID  int
}

// New создает новый экземпляр календаря
func New() *Calendar {
	return &Calendar{
		events: make(map[int]Event),
		lastID: 0,
	}
}

// CreateEvent создает новое событие
func (c *Calendar) CreateEvent(userID int, date time.Time, title string) (Event, error) {
	if title == "" {
		return Event{}, errors.New("title cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastID++
	event := Event{
		ID:     c.lastID,
		UserID: userID,
		Date:   date,
		Title:  title,
	}

	c.events[event.ID] = event
	return event, nil
}

// UpdateEvent обновляет существующее событие
func (c *Calendar) UpdateEvent(eventID, userID int, date time.Time, title string) (Event, error) {
	if title == "" {
		return Event{}, errors.New("title cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	event, exists := c.events[eventID]
	if !exists {
		return Event{}, errors.New("event not found")
	}

	if event.UserID != userID {
		return Event{}, errors.New("user ID mismatch")
	}

	event.Date = date
	event.Title = title
	c.events[eventID] = event

	return event, nil
}

// DeleteEvent удаляет событие
func (c *Calendar) DeleteEvent(eventID, userID int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	event, exists := c.events[eventID]
	if !exists {
		return errors.New("event not found")
	}

	if event.UserID != userID {
		return errors.New("user ID mismatch")
	}

	delete(c.events, eventID)
	return nil
}

// GetEventsForDay возвращает события на указанный день
func (c *Calendar) GetEventsForDay(userID int, date time.Time) []Event {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var events []Event
	for _, event := range c.events {
		if event.UserID == userID &&
			event.Date.Year() == date.Year() &&
			event.Date.Month() == date.Month() &&
			event.Date.Day() == date.Day() {
			events = append(events, event)
		}
	}

	sortEventsByDate(events)
	return events
}

// GetEventsForWeek возвращает события на неделю, начиная с указанной даты
func (c *Calendar) GetEventsForWeek(userID int, date time.Time) []Event {
	c.mu.RLock()
	defer c.mu.RUnlock()

	endDate := date.AddDate(0, 0, 7)
	var events []Event
	for _, event := range c.events {
		if event.UserID == userID &&
			!event.Date.Before(date) &&
			event.Date.Before(endDate) {
			events = append(events, event)
		}
	}

	sortEventsByDate(events)
	return events
}

// GetEventsForMonth возвращает события на месяц
func (c *Calendar) GetEventsForMonth(userID int, date time.Time) []Event {
	c.mu.RLock()
	defer c.mu.RUnlock()

	startOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	startOfNextMonth := startOfMonth.AddDate(0, 1, 0)

	var events []Event
	for _, event := range c.events {
		if event.UserID == userID &&
			!event.Date.Before(startOfMonth) &&
			event.Date.Before(startOfNextMonth) {
			events = append(events, event)
		}
	}

	sortEventsByDate(events)
	return events
}

// Helper function to sort events by date
func sortEventsByDate(events []Event) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].Date.Before(events[j].Date)
	})
}

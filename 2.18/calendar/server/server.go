package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"calendar/internal/calendar"

	"github.com/gorilla/mux"
)

// Request structures
type CreateEventRequest struct {
	UserID int    `json:"user_id"`
	Date   string `json:"date"`
	Title  string `json:"title"`
}

type UpdateEventRequest struct {
	EventID int    `json:"event_id"`
	UserID  int    `json:"user_id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
}

type DeleteEventRequest struct {
	EventID int `json:"event_id"`
	UserID  int `json:"user_id"`
}

type Response struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// Server представляет HTTP-сервер
type Server struct {
	calendar *calendar.Calendar
	router   *mux.Router
}

// New создает новый HTTP-сервер
func New(calendar *calendar.Calendar) *Server {
	s := &Server{
		calendar: calendar,
		router:   mux.NewRouter(),
	}

	s.router.Use(loggingMiddleware)
	s.setupRoutes()

	return s
}

func (s *Server) setupRoutes() {
	s.router.HandleFunc("/create_event", s.createEventHandler).Methods("POST")
	s.router.HandleFunc("/update_event", s.updateEventHandler).Methods("POST")
	s.router.HandleFunc("/delete_event", s.deleteEventHandler).Methods("POST")
	s.router.HandleFunc("/events_for_day", s.eventsForDayHandler).Methods("GET")
	s.router.HandleFunc("/events_for_week", s.eventsForWeekHandler).Methods("GET")
	s.router.HandleFunc("/events_for_month", s.eventsForMonthHandler).Methods("GET")
}

// ServeHTTP реализует интерфейс http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) createEventHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest
	if err := decodeRequest(r, &req); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Title == "" {
		sendError(w, http.StatusBadRequest, "title is required")
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		sendError(w, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
		return
	}

	event, err := s.calendar.CreateEvent(req.UserID, date, req.Title)
	if err != nil {
		sendError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, Response{Result: event})
}

func (s *Server) updateEventHandler(w http.ResponseWriter, r *http.Request) {
	var req UpdateEventRequest
	if err := decodeRequest(r, &req); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Title == "" {
		sendError(w, http.StatusBadRequest, "title is required")
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		sendError(w, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")
		return
	}

	event, err := s.calendar.UpdateEvent(req.EventID, req.UserID, date, req.Title)
	if err != nil {
		sendError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, Response{Result: event})
}

func (s *Server) deleteEventHandler(w http.ResponseWriter, r *http.Request) {
	var req DeleteEventRequest
	if err := decodeRequest(r, &req); err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.calendar.DeleteEvent(req.EventID, req.UserID); err != nil {
		sendError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, Response{Result: "event deleted successfully"})
}

func (s *Server) eventsForDayHandler(w http.ResponseWriter, r *http.Request) {
	userID, date, err := parseQueryParams(r)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	events := s.calendar.GetEventsForDay(userID, date)
	sendJSON(w, http.StatusOK, Response{Result: events})
}

func (s *Server) eventsForWeekHandler(w http.ResponseWriter, r *http.Request) {
	userID, date, err := parseQueryParams(r)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	events := s.calendar.GetEventsForWeek(userID, date)
	sendJSON(w, http.StatusOK, Response{Result: events})
}

func (s *Server) eventsForMonthHandler(w http.ResponseWriter, r *http.Request) {
	userID, date, err := parseQueryParams(r)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	events := s.calendar.GetEventsForMonth(userID, date)
	sendJSON(w, http.StatusOK, Response{Result: events})
}

func decodeRequest(r *http.Request, v interface{}) error {
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		return json.NewDecoder(r.Body).Decode(v)
	}
	// Handle form data
	if err := r.ParseForm(); err != nil {
		return err
	}
	// For simplicity, we'll handle JSON only in this example
	// For form data, you would need to manually parse each field
	return fmt.Errorf("only JSON content type is supported")
}

func parseQueryParams(r *http.Request) (int, time.Time, error) {
	userIDStr := r.URL.Query().Get("user_id")
	dateStr := r.URL.Query().Get("date")
	if userIDStr == "" || dateStr == "" {
		return 0, time.Time{}, fmt.Errorf("user_id and date are required")
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("invalid user_id")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("invalid date format, expected YYYY-MM-DD")
	}
	return userID, date, nil
}

func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func sendError(w http.ResponseWriter, status int, message string) {
	sendJSON(w, status, Response{Error: message})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		// Создаем wrapper для записи статуса
		rw := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, rw.status, time.Since(start))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

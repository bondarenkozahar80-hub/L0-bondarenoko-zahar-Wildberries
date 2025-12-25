package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"calendar/internal/calendar"
	"calendar/internal/server"
)

func main() {
	// Парсим аргументы командной строки
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	// Создаем календарь и сервер
	calendar := calendar.New()
	srv := server.New(calendar)

	// Настраиваем обработку запросов
	http.Handle("/", srv)

	// Запускаем сервер
	addr := fmt.Sprintf(":%s", *port)
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Order map[string]interface{} // гибкая модель — хранит весь json

type Service struct {
	db     *pgxpool.Pool
	cache  map[string]Order
	mu     sync.RWMutex
	kafkaR *kafka.Reader
}

// loadCacheFromDB загружает актуальные данные из БД в кеш при старте.
// Для простоты загружаем все записи; при большом объёме — ограничьте выборку.
func (s *Service) loadCacheFromDB(ctx context.Context) error {
	log.Println("Loading cache from DB...")
	rows, err := s.db.Query(ctx, `SELECT order_id, payload FROM orders`)
	if err != nil {
		return err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var id string
		var payloadBytes []byte
		if err := rows.Scan(&id, &payloadBytes); err != nil {
			log.Println("scan error:", err)
			continue
		}
		var ord Order
		if err := json.Unmarshal(payloadBytes, &ord); err != nil {
			log.Println("json unmarshal error for id", id, ":", err)
			continue
		}
		s.mu.Lock()
		s.cache[id] = ord
		s.mu.Unlock()
		count++
	}
	log.Printf("Loaded %d orders into cache\n", count)
	return rows.Err()
}

// handleGetOrder возвращает заказ по order_id (сначала из кеша, иначе из БД)
func (s *Service) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("order_id")
	if id == "" {
		http.Error(w, "order_id required", http.StatusBadRequest)
		return
	}
	s.mu.RLock()
	ord, ok := s.cache[id]
	s.mu.RUnlock()
	if ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ord)
		return
	}
	// подтянуть из БД
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	var payloadBytes []byte
	err := s.db.QueryRow(ctx, `SELECT payload FROM orders WHERE order_id=$1`, id).Scan(&payloadBytes)
	if err != nil {
		if err == pgxpool.ErrNoRows || err == sql.ErrNoRows {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Println("db select error:", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var o Order
	if err := json.Unmarshal(payloadBytes, &o); err != nil {
		http.Error(w, "stored payload invalid", http.StatusInternalServerError)
		return
	}
	// кладём в кеш
	s.mu.Lock()
	s.cache[id] = o
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(o)
}

// runKafkaConsumer запускает цикл чтения сообщений и обработки
func (s *Service) runKafkaConsumer(ctx context.Context, topic string) {
	log.Println("Starting kafka consumer for topic:", topic)
	for {
		m, err := s.kafkaR.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("kafka reader context done:", ctx.Err())
				return
			}
			log.Println("kafka read error:", err)
			time.Sleep(time.Second)
			continue
		}
		go s.processMessage(ctx, m) // параллельно, но можно ограничивать concurrency
	}
}

func (s *Service) processMessage(ctx context.Context, m kafka.Message) {
	// парсим JSON
	var ord Order
	if err := json.Unmarshal(m.Value, &ord); err != nil {
		log.Println("invalid JSON, skipping message:", err, "partition/offset:", m.Partition, m.Offset)
		return
	}

	// минимальная валидация: наличие order_id
	idRaw, ok := ord["order_id"]
	if !ok {
		log.Println("message missing order_id, skipping")
		return
	}
	id, ok := idRaw.(string)
	if !ok || id == "" {
		log.Println("invalid order_id type, skipping")
		return
	}

	// сохраняем в БД в транзакции
	payloadBytes, _ := json.Marshal(ord)
	tx, err := s.db.Begin(ctx)
	if err != nil {
		log.Println("db begin error:", err)
		return
	}
	defer func() {
		// если tx не nil and not committed/rolled back
		// pgxpool tx will auto-rollback on Close if not committed
	}()

	// UPSERT в orders
	_, err = tx.Exec(ctx, `
        INSERT INTO orders (order_id, payload, status, created_at, updated_at)
        VALUES ($1, $2, $3, now(), now())
        ON CONFLICT (order_id) DO UPDATE
          SET payload = EXCLUDED.payload,
              status = EXCLUDED.status,
              updated_at = now()
    `, id, payloadBytes, ord["status"])
	if err != nil {
		log.Println("db exec error:", err)
		tx.Rollback(ctx)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Println("db commit error:", err)
		return
	}

	// обновляем кеш
	s.mu.Lock()
	s.cache[id] = ord
	s.mu.Unlock()

	// подтверждаем сообщение: в kafka-go Reader при использовании GroupID авто-commit может быть включён.
	// Здесь для уверенности вызываем CommitMessages
	if err := s.kafkaR.CommitMessages(ctx, m); err != nil {
		log.Println("kafka commit error:", err)
		// не откатываем данные в БД — лучше логировать и разбираться
	}

	log.Printf("Processed order %s (partition=%d offset=%d)\n", id, m.Partition, m.Offset)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// конфигурация через env (можно заменить)
	pgURL := getenv("PG_URL", "postgres://orders_user:orders_pass@localhost:5432/orders_db")
	kafkaBroker := getenv("KAFKA_BROKER", "localhost:9092")
	kafkaTopic := getenv("KAFKA_TOPIC", "orders")
	kafkaGroup := getenv("KAFKA_GROUP", "orders-service-group")
	httpAddr := getenv("HTTP_ADDR", ":8080")

	// подключаемся к Postgres
	dbpool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		log.Fatal("failed connect pg:", err)
	}
	defer dbpool.Close()

	svc := &Service{
		db:    dbpool,
		cache: make(map[string]Order),
	}

	// preload cache
	if err := svc.loadCacheFromDB(ctx); err != nil {
		log.Println("warning: failed to load cache at startup:", err)
	}

	// kafka reader
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{kafkaBroker},
		GroupID:  kafkaGroup,
		Topic:    kafkaTopic,
		MinBytes: 1,
		MaxBytes: 10e6,
		MaxWait:  500 * time.Millisecond,
	})
	svc.kafkaR = r

	// запуск consumer
	go svc.runKafkaConsumer(ctx, kafkaTopic)

	// HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/api/order", svc.handleGetOrder)
	// файл фронтенда (если положен рядом)
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	srv := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	go func() {
		log.Println("HTTP server listening on", httpAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("http server error:", err)
		}
	}()

	// graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	log.Println("shutting down...")

	r.Close()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	log.Println("bye")
}

func getenv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

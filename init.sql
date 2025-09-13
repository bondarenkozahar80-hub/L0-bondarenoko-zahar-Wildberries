-- 001_schema.sql
-- будет выполнен при первом запуске контейнера postgres

-- Создан Database/Role уже через переменные окружения (см. docker-compose),
-- но для явности можно создать отдельного роле/бд если нужно:
-- CREATE ROLE orders_user WITH LOGIN PASSWORD 'orders_pass';
-- CREATE DATABASE orders_db OWNER orders_user;

-- Таблица для хранения полных JSON заказов
CREATE TABLE IF NOT EXISTS orders (
                                      order_id TEXT PRIMARY KEY,
                                      payload JSONB NOT NULL,
                                      status TEXT,
                                      created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
    );

-- Индекс для быстрого поиска по полям в JSON, например customer.id
CREATE INDEX IF NOT EXISTS idx_orders_payload_customer_id ON orders ((payload->>'customer_id'));
CREATE INDEX IF NOT EXISTS idx_orders_updated_at ON orders (updated_at);
// internal/database/postgres.go
package database

import (
	"database/sql"
	"fmt"
	"log" // Используйте структурированный логгер в реальном приложении

	"currency-service/internal/config" // Пример импорта конфига

	_ "github.com/lib/pq" // PostgreSQL driver
)

// NewPostgresConnection создает и возвращает новое соединение с PostgreSQL.
func NewPostgresConnection(cfg config.DBConfig) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия соединения с БД: %w", err)
	}

	// Установка параметров пула соединений (пример)
	db.SetMaxOpenConns(25) // Максимальное количество открытых соединений
	db.SetMaxIdleConns(25) // Максимальное количество простаивающих соединений
	// db.SetConnMaxLifetime(5*time.Minute) // Максимальное время жизни соединения

	if err = db.Ping(); err != nil {
		db.Close() // Закрыть, если пинг не прошел
		return nil, fmt.Errorf("ошибка проверки соединения с БД: %w", err)
	}

	log.Println("Соединение с PostgreSQL установлено")
	return db, nil
}

// MigrateSchema применяет миграции схемы БД.
func MigrateSchema(db *sql.DB) error {
	// Таблица курсов
	queryRates := `
    CREATE TABLE IF NOT EXISTS rates (
        id SERIAL PRIMARY KEY,
        value REAL NOT NULL CHECK (value > 0), -- Добавим проверку на положительное значение
        timestamp TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
    );`
	_, err := db.Exec(queryRates)
	if err != nil {
		return fmt.Errorf("ошибка инициализации схемы БД (rates): %w", err)
	}
	log.Println("Таблица 'rates' инициализирована (или уже существует)")

	// (!!!) Новая таблица кошельков
	queryWallets := `
    CREATE TABLE IF NOT EXISTS wallets (
        wallet_number VARCHAR(7) PRIMARY KEY, -- Номер кошелька как строка из 7 символов, первичный ключ
        balance REAL NOT NULL DEFAULT 0 CHECK (balance >= 0), -- Баланс, не может быть отрицательным
        created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
    );

    -- Индекс для ускорения поиска по номеру кошелька (хотя он и так PRIMARY KEY)
    -- CREATE INDEX IF NOT EXISTS idx_wallets_number ON wallets (wallet_number);

    -- Триггер для автоматического обновления updated_at
    CREATE OR REPLACE FUNCTION update_updated_at_column()
    RETURNS TRIGGER AS $$
    BEGIN
       NEW.updated_at = NOW();
       RETURN NEW;
    END;
    $$ language 'plpgsql';

    DROP TRIGGER IF EXISTS update_wallets_updated_at ON wallets; -- Удаляем старый, если вдруг был
    CREATE TRIGGER update_wallets_updated_at
    BEFORE UPDATE ON wallets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
    `
	_, err = db.Exec(queryWallets)
	if err != nil {
		return fmt.Errorf("ошибка инициализации схемы БД (wallets): %w", err)
	}
	log.Println("Таблица 'wallets' инициализирована (или уже существует)")

	return nil
}

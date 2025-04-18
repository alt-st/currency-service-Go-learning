// internal/repository/postgres_rate_repository.go
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log" // Используйте структурированный логгер

	"currency-service/internal/models"
)

type postgresRateRepository struct {
	// Убрали db *sql.DB отсюда, так как DBTX передается в каждый метод
}

// NewPostgresRateRepository создает новый экземпляр репозитория для PostgreSQL.
// Теперь не принимает *sql.DB, так как он будет передаваться в методы.
func NewPostgresRateRepository() RateRepository {
	return &postgresRateRepository{}
}

// SaveRate сохраняет курс, используя переданный DBTX (может быть *sql.DB или *sql.Tx)
func (r *postgresRateRepository) SaveRate(ctx context.Context, db DBTX, rate models.Rate) error {
	query := "INSERT INTO rates (value, timestamp) VALUES ($1, $2)"
	// Используем rate.Timestamp, если он установлен, иначе можно использовать CURRENT_TIMESTAMP в SQL
	if rate.Timestamp.IsZero() {
		query = "INSERT INTO rates (value) VALUES ($1)" // Полагаемся на DEFAULT в БД
		_, err := db.ExecContext(ctx, query, rate.Value)
		if err != nil {
			log.Printf("Ошибка сохранения курса в БД (без timestamp): %v\n", err)
			return fmt.Errorf("ошибка выполнения запроса INSERT: %w", err)
		}
	} else {
		_, err := db.ExecContext(ctx, query, rate.Value, rate.Timestamp)
		if err != nil {
			log.Printf("Ошибка сохранения курса в БД (с timestamp): %v\n", err)
			return fmt.Errorf("ошибка выполнения запроса INSERT: %w", err)
		}
	}

	return nil
}

// GetLatestRates извлекает последние 'limit' курсов.
func (r *postgresRateRepository) GetLatestRates(ctx context.Context, db DBTX, limit int) ([]models.Rate, error) {
	query := "SELECT id, value, timestamp FROM rates ORDER BY timestamp DESC LIMIT $1"
	rows, err := db.QueryContext(ctx, query, limit)
	if err != nil {
		log.Printf("Ошибка получения курсов из БД: %v\n", err)
		return nil, fmt.Errorf("ошибка выполнения запроса SELECT: %w", err)
	}
	defer rows.Close()

	var rates []models.Rate
	for rows.Next() {
		var rate models.Rate
		if err := rows.Scan(&rate.ID, &rate.Value, &rate.Timestamp); err != nil {
			log.Printf("Ошибка сканирования строки результата (rates): %v\n", err)
			// Можно вернуть ошибку или собранные данные
			return rates, fmt.Errorf("ошибка сканирования строки rates: %w", err)
		}
		rates = append(rates, rate)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Ошибка итерации по результатам запроса (rates): %v\n", err)
		return nil, fmt.Errorf("ошибка после итерации по результатам rates: %w", err)
	}

	return rates, nil
}

// GetLatestRate получает самый свежий курс.
func (r *postgresRateRepository) GetLatestRate(ctx context.Context, db DBTX) (models.Rate, error) {
	query := "SELECT id, value, timestamp FROM rates ORDER BY timestamp DESC LIMIT 1"
	row := db.QueryRowContext(ctx, query)

	var rate models.Rate
	err := row.Scan(&rate.ID, &rate.Value, &rate.Timestamp)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Println("Нет доступных курсов в БД")
			return models.Rate{}, fmt.Errorf("нет доступных курсов: %w", err) // Возвращаем ошибку, если нет строк
		}
		log.Printf("Ошибка получения последнего курса из БД: %v\n", err)
		return models.Rate{}, fmt.Errorf("ошибка выполнения запроса SELECT (latest rate): %w", err)
	}

	return rate, nil
}

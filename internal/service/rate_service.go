// internal/service/rate_service.go
package service

import (
	"context"
	"database/sql" // Понадобится для передачи *sql.DB в репозиторий
	"fmt"
	"log" // Используйте структурированный логгер

	"currency-service/internal/models"
	"currency-service/internal/repository"
)

type rateService struct {
	repo repository.RateRepository
	db   *sql.DB // Добавляем зависимость от *sql.DB для передачи в репозиторий
}

// NewRateService создает новый экземпляр сервиса курсов валют.
// Теперь принимает *sql.DB.
func NewRateService(repo repository.RateRepository, db *sql.DB) RateService {
	return &rateService{repo: repo, db: db}
}

func (s *rateService) CreateRate(ctx context.Context, value float64) error {
	if value <= 0 {
		return fmt.Errorf("курс валюты должен быть положительным числом")
	}

	rate := models.Rate{
		Value: value,
		// Timestamp будет установлен БД
	}

	// Вызываем репозиторий, передавая *sql.DB
	err := s.repo.SaveRate(ctx, s.db, rate)
	if err != nil {
		log.Printf("Ошибка при вызове SaveRate из сервиса: %v\n", err)
		return fmt.Errorf("не удалось сохранить курс: %w", err)
	}
	return nil
}

func (s *rateService) GetAverageRate(ctx context.Context, limit int) (models.AverageResponse, error) {
	if limit <= 0 {
		limit = 10
		log.Printf("Лимит не указан или некорректен, используется значение по умолчанию: %d\n", limit)
	}

	// Вызываем репозиторий, передавая *sql.DB
	rates, err := s.repo.GetLatestRates(ctx, s.db, limit)
	if err != nil {
		log.Printf("Ошибка при вызове GetLatestRates из сервиса: %v\n", err)
		return models.AverageResponse{}, fmt.Errorf("не удалось получить последние курсы: %w", err)
	}

	if len(rates) == 0 {
		return models.AverageResponse{Average: 0, Count: 0}, nil
	}

	var sum float64
	for _, rate := range rates {
		sum += rate.Value
	}

	average := sum / float64(len(rates))

	return models.AverageResponse{
		Average: average,
		Count:   len(rates),
	}, nil
}

// GetLatestRate получает самый свежий курс.
func (s *rateService) GetLatestRate(ctx context.Context) (models.Rate, error) {
	rate, err := s.repo.GetLatestRate(ctx, s.db)
	if err != nil {
		log.Printf("Ошибка при вызове GetLatestRate из сервиса: %v\n", err)
		// Обрабатываем ошибку "нет курсов" отдельно, если нужно
		if err.Error() == "нет доступных курсов: sql: no rows in result set" {
			// Можно вернуть кастомную ошибку сервисного уровня
			return models.Rate{}, fmt.Errorf("в системе нет зарегистрированных курсов валют")
		}
		return models.Rate{}, fmt.Errorf("не удалось получить последний курс: %w", err)
	}
	return rate, nil
}

// --- internal/service/interface.go ---
package service

import (
	"context"
	"currency-service/internal/models"
)

// RateService определяет методы бизнес-логики для работы с курсами валют.
type RateService interface {
	CreateRate(ctx context.Context, value float64) error
	GetAverageRate(ctx context.Context, limit int) (models.AverageResponse, error)
	GetLatestRate(ctx context.Context) (models.Rate, error) // <-- Новый метод
}

// (!!!) WalletService определяет методы бизнес-логики для работы с кошельками.
type WalletService interface {
	// UpdateBalance создает кошелек или обновляет его баланс.
	// amount может быть положительным или отрицательным.
	UpdateBalance(ctx context.Context, req models.UpdateBalanceRequest) (models.UpdateBalanceResponse, error)
	// ListWallets возвращает список всех кошельков.
	ListWallets(ctx context.Context) (models.ListWalletsResponse, error)
	// ConvertAndDeduct выполняет конвертацию и списание средств.
	ConvertAndDeduct(ctx context.Context, req models.ConvertRequest) (models.ConvertResponse, error)
}

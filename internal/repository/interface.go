// internal/repository/interface.go
package repository

import (
	"context"
	"database/sql" // Понадобится для транзакций

	"currency-service/internal/models"
)

// DBTX определяет интерфейс, который может быть *sql.DB или *sql.Tx
// Это позволяет использовать одни и те же методы репозитория как с транзакциями, так и без них.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// RateRepository определяет методы для взаимодействия с хранилищем курсов валют.
type RateRepository interface {
	// SaveRate сохраняет один курс валюты
	SaveRate(ctx context.Context, db DBTX, rate models.Rate) error // <-- Принимает DBTX
	// GetLatestRates получает последние 'limit' курсов валют
	GetLatestRates(ctx context.Context, db DBTX, limit int) ([]models.Rate, error) // <-- Принимает DBTX
	// GetLatestRate получает самый свежий курс
	GetLatestRate(ctx context.Context, db DBTX) (models.Rate, error) // <-- Новый метод
}

// (!!!) WalletRepository определяет методы для работы с кошельками.
type WalletRepository interface {
	// GetWalletByNumber находит кошелек по номеру. Возвращает sql.ErrNoRows, если не найден.
	GetWalletByNumber(ctx context.Context, db DBTX, number string) (models.Wallet, error)
	// GetAllWallets получает все кошельки.
	GetAllWallets(ctx context.Context, db DBTX) ([]models.Wallet, error)
	// CreateWallet создает новый кошелек.
	CreateWallet(ctx context.Context, db DBTX, wallet models.Wallet) error
	// UpdateWalletBalance обновляет баланс кошелька.
	// Важно: этот метод должен использоваться внутри транзакции для безопасности.
	UpdateWalletBalance(ctx context.Context, db DBTX, number string, newBalance float64) error
	// GetWalletByNumberForUpdate находит кошелек по номеру с блокировкой строки (SELECT ... FOR UPDATE).
	// Используется внутри транзакций для предотвращения гонок обновлений.
	GetWalletByNumberForUpdate(ctx context.Context, tx *sql.Tx, number string) (models.Wallet, error)
}

// --- internal/repository/postgres_wallet_repository.go ---
// (!!!) Новый файл
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"currency-service/internal/models"

	"github.com/lib/pq" // Для обработки ошибок PostgreSQL (например, unique_violation)
)

type postgresWalletRepository struct {
	// Пустая структура, так как *sql.DB передается в методы
}

// NewPostgresWalletRepository создает новый экземпляр репозитория кошельков.
func NewPostgresWalletRepository() WalletRepository {
	return &postgresWalletRepository{}
}

// GetWalletByNumber находит кошелек по номеру.
func (r *postgresWalletRepository) GetWalletByNumber(ctx context.Context, db DBTX, number string) (models.Wallet, error) {
	query := "SELECT wallet_number, balance, created_at, updated_at FROM wallets WHERE wallet_number = $1"
	row := db.QueryRowContext(ctx, query, number)

	var wallet models.Wallet
	err := row.Scan(&wallet.Number, &wallet.Balance, &wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		// Ошибку sql.ErrNoRows обрабатываем в сервисе
		if err != sql.ErrNoRows {
			log.Printf("Ошибка получения кошелька %s из БД: %v\n", number, err)
		}
		return models.Wallet{}, err // Возвращаем ошибку как есть
	}
	return wallet, nil
}

// GetWalletByNumberForUpdate находит кошелек по номеру с блокировкой строки (ДЛЯ ТРАНЗАКЦИЙ).
func (r *postgresWalletRepository) GetWalletByNumberForUpdate(ctx context.Context, tx *sql.Tx, number string) (models.Wallet, error) {
	query := "SELECT wallet_number, balance, created_at, updated_at FROM wallets WHERE wallet_number = $1 FOR UPDATE"
	row := tx.QueryRowContext(ctx, query, number) // Используем транзакцию tx

	var wallet models.Wallet
	err := row.Scan(&wallet.Number, &wallet.Balance, &wallet.CreatedAt, &wallet.UpdatedAt)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Ошибка получения кошелька %s из БД (FOR UPDATE): %v\n", number, err)
		}
		return models.Wallet{}, err
	}
	return wallet, nil
}

// GetAllWallets получает все кошельки.
func (r *postgresWalletRepository) GetAllWallets(ctx context.Context, db DBTX) ([]models.Wallet, error) {
	query := "SELECT wallet_number, balance, created_at, updated_at FROM wallets ORDER BY created_at ASC"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Ошибка получения списка кошельков из БД: %v\n", err)
		return nil, fmt.Errorf("ошибка выполнения запроса SELECT (all wallets): %w", err)
	}
	defer rows.Close()

	var wallets []models.Wallet
	for rows.Next() {
		var wallet models.Wallet
		if err := rows.Scan(&wallet.Number, &wallet.Balance, &wallet.CreatedAt, &wallet.UpdatedAt); err != nil {
			log.Printf("Ошибка сканирования строки результата (wallets): %v\n", err)
			return wallets, fmt.Errorf("ошибка сканирования строки wallets: %w", err)
		}
		wallets = append(wallets, wallet)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Ошибка итерации по результатам запроса (wallets): %v\n", err)
		return nil, fmt.Errorf("ошибка после итерации по результатам wallets: %w", err)
	}

	return wallets, nil
}

// CreateWallet создает новый кошелек.
func (r *postgresWalletRepository) CreateWallet(ctx context.Context, db DBTX, wallet models.Wallet) error {
	query := "INSERT INTO wallets (wallet_number, balance) VALUES ($1, $2)"
	_, err := db.ExecContext(ctx, query, wallet.Number, wallet.Balance)
	if err != nil {
		// Проверка на ошибку уникальности (если кошелек уже существует)
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // 23505 - unique_violation
			log.Printf("Попытка создать дублирующийся кошелек: %s\n", wallet.Number)
			return fmt.Errorf("кошелек с номером %s уже существует", wallet.Number) // Возвращаем специфичную ошибку
		}
		log.Printf("Ошибка создания кошелька %s в БД: %v\n", wallet.Number, err)
		return fmt.Errorf("ошибка выполнения запроса INSERT (wallet): %w", err)
	}
	log.Printf("Кошелек %s успешно создан с балансом %.2f\n", wallet.Number, wallet.Balance)
	return nil
}

// UpdateWalletBalance обновляет баланс кошелька. Должен вызываться внутри транзакции.
func (r *postgresWalletRepository) UpdateWalletBalance(ctx context.Context, db DBTX, number string, newBalance float64) error {
	// Используем db (который должен быть *sql.Tx в этом контексте)
	query := "UPDATE wallets SET balance = $1 WHERE wallet_number = $2"
	result, err := db.ExecContext(ctx, query, newBalance, number)
	if err != nil {
		log.Printf("Ошибка обновления баланса кошелька %s в БД: %v\n", number, err)
		return fmt.Errorf("ошибка выполнения запроса UPDATE (wallet balance): %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("Ошибка получения количества затронутых строк при обновлении кошелька %s: %v\n", number, err)
		return fmt.Errorf("ошибка проверки результата UPDATE: %w", err)
	}

	if rowsAffected == 0 {
		log.Printf("Попытка обновить баланс несуществующего кошелька: %s\n", number)
		return sql.ErrNoRows // Возвращаем стандартную ошибку, если кошелек не найден
	}

	log.Printf("Баланс кошелька %s успешно обновлен на %.2f\n", number, newBalance)
	return nil
}

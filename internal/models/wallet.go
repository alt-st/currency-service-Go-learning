// internal/models/wallet.go
package models

import "time"

// Wallet представляет кошелек пользователя.
type Wallet struct {
	Number    string    `json:"number" db:"wallet_number"` // Номер кошелька (7 знаков)
	Balance   float64   `json:"balance" db:"balance"`      // Баланс кошелька
	CreatedAt time.Time `json:"-" db:"created_at"`         // Время создания (не отдаем в JSON)
	UpdatedAt time.Time `json:"-" db:"updated_at"`         // Время последнего обновления (не отдаем в JSON)
}

// UpdateBalanceRequest представляет тело запроса на обновление баланса.
type UpdateBalanceRequest struct {
	WalletNumber string  `json:"wallet_number"`
	Amount       float64 `json:"amount"` // Может быть положительным (пополнение) или отрицательным (списание)
}

// UpdateBalanceResponse представляет ответ после обновления баланса.
type UpdateBalanceResponse struct {
	WalletNumber string  `json:"wallet_number"`
	NewBalance   float64 `json:"new_balance"`
	Message      string  `json:"message,omitempty"` // Сообщение об успехе или ошибке (например, недостаточно средств)
}

// ListWalletsResponse представляет ответ со списком кошельков.
type ListWalletsResponse struct {
	Wallets []Wallet `json:"wallets"`
}

// ConvertRequest представляет тело запроса на конвертацию.
type ConvertRequest struct {
	FirstName          string  `json:"first_name"` // Пока не используется в логике, но есть в запросе
	LastName           string  `json:"last_name"`  // Пока не используется в логике, но есть в запросе
	UserID             string  `json:"user_id"`    // Пока не используется в логике, но есть в запросе
	AmountToConvert    float64 `json:"amount_to_convert"`
	SourceWalletNumber string  `json:"source_wallet_number"`
}

// ConvertResponse представляет ответ после попытки конвертации.
type ConvertResponse struct {
	SourceWalletNumber string  `json:"source_wallet_number"`
	RemainingBalance   float64 `json:"remaining_balance,omitempty"` // Поле будет заполнено при успехе
	ConvertedAmount    float64 `json:"converted_amount,omitempty"`  // Поле будет заполнено при успехе
	RateUsed           float64 `json:"rate_used,omitempty"`         // Поле будет заполнено при успехе
	Message            string  `json:"message"`                     // Сообщение об успехе или ошибке
}

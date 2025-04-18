// internal/handlers/tests/wallet_handler_test.go
package handlers_test // Пакет тот же, что и у main_test.go

import (
	"currency-service/internal/models" // Импортируем нужные модели
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Тесты для Wallet Handler ---
// Используют testRouter и testDB из main_test.go

func TestWalletHandler_UpdateBalance_CreateWallet(t *testing.T) {
	cleanupTestDB(t) // Очищаем БД перед тестом

	walletNumber := "1234567"
	initialAmount := 100.50

	payload := models.UpdateBalanceRequest{
		WalletNumber: walletNumber,
		Amount:       initialAmount,
	}
	req := createRequest(t, http.MethodPost, "/api/v1/wallets/balance", payload)
	rr := executeRequest(t, req)

	// Проверка статус-кода
	assert.Equal(t, http.StatusOK, rr.Code, "Ожидался статус OK (200)")

	// Проверка тела ответа
	var resp models.UpdateBalanceResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err, "Ошибка демаршалинга ответа")

	assert.Equal(t, walletNumber, resp.WalletNumber)
	assert.InDelta(t, initialAmount, resp.NewBalance, 0.001) // Сравнение float с допуском
	assert.Contains(t, resp.Message, "создан", "Ожидалось сообщение о создании")

	// (Опционально) Проверка состояния БД
	var balance float64
	err = testDB.QueryRow("SELECT balance FROM wallets WHERE wallet_number = $1", walletNumber).Scan(&balance)
	require.NoError(t, err, "Кошелек должен существовать в БД")
	assert.InDelta(t, initialAmount, balance, 0.001, "Баланс в БД не совпадает")
}

func TestWalletHandler_UpdateBalance_DepositExisting(t *testing.T) {
	cleanupTestDB(t)
	walletNumber := "7654321"
	initialBalance := 50.0
	depositAmount := 25.50

	// Создаем кошелек заранее
	_, err := testDB.Exec("INSERT INTO wallets (wallet_number, balance) VALUES ($1, $2)", walletNumber, initialBalance)
	require.NoError(t, err, "Не удалось создать начальный кошелек")

	payload := models.UpdateBalanceRequest{
		WalletNumber: walletNumber,
		Amount:       depositAmount,
	}
	req := createRequest(t, http.MethodPost, "/api/v1/wallets/balance", payload)
	rr := executeRequest(t, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp models.UpdateBalanceResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	expectedBalance := initialBalance + depositAmount
	assert.Equal(t, walletNumber, resp.WalletNumber)
	assert.InDelta(t, expectedBalance, resp.NewBalance, 0.001)
	assert.Contains(t, resp.Message, "пополнен", "Ожидалось сообщение о пополнении")

	// Проверка БД
	var dbBalance float64
	err = testDB.QueryRow("SELECT balance FROM wallets WHERE wallet_number = $1", walletNumber).Scan(&dbBalance)
	require.NoError(t, err)
	assert.InDelta(t, expectedBalance, dbBalance, 0.001)
}

func TestWalletHandler_UpdateBalance_WithdrawExisting_Success(t *testing.T) {
	cleanupTestDB(t)
	walletNumber := "1112233"
	initialBalance := 100.0
	withdrawAmount := -30.0 // Отрицательное значение для списания

	_, err := testDB.Exec("INSERT INTO wallets (wallet_number, balance) VALUES ($1, $2)", walletNumber, initialBalance)
	require.NoError(t, err)

	payload := models.UpdateBalanceRequest{
		WalletNumber: walletNumber,
		Amount:       withdrawAmount,
	}
	req := createRequest(t, http.MethodPost, "/api/v1/wallets/balance", payload)
	rr := executeRequest(t, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp models.UpdateBalanceResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	expectedBalance := initialBalance + withdrawAmount // 100 - 30 = 70
	assert.Equal(t, walletNumber, resp.WalletNumber)
	assert.InDelta(t, expectedBalance, resp.NewBalance, 0.001)
	assert.Contains(t, resp.Message, "Списание успешно", "Ожидалось сообщение о списании")

	// Проверка БД
	var dbBalance float64
	err = testDB.QueryRow("SELECT balance FROM wallets WHERE wallet_number = $1", walletNumber).Scan(&dbBalance)
	require.NoError(t, err)
	assert.InDelta(t, expectedBalance, dbBalance, 0.001)
}

func TestWalletHandler_UpdateBalance_WithdrawExisting_InsufficientFunds(t *testing.T) {
	cleanupTestDB(t)
	walletNumber := "4445566"
	initialBalance := 20.0
	withdrawAmount := -50.0 // Пытаемся списать больше, чем есть

	_, err := testDB.Exec("INSERT INTO wallets (wallet_number, balance) VALUES ($1, $2)", walletNumber, initialBalance)
	require.NoError(t, err)

	payload := models.UpdateBalanceRequest{
		WalletNumber: walletNumber,
		Amount:       withdrawAmount,
	}
	req := createRequest(t, http.MethodPost, "/api/v1/wallets/balance", payload)
	rr := executeRequest(t, req)

	// Ожидаем статус 409 Conflict
	assert.Equal(t, http.StatusConflict, rr.Code, "Ожидался статус Conflict (409)")

	var resp models.UpdateBalanceResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, walletNumber, resp.WalletNumber)
	assert.InDelta(t, initialBalance, resp.NewBalance, 0.001, "Баланс не должен был измениться")
	assert.Contains(t, resp.Message, "недостаточно средств на счете", "Ожидалось сообщение о недостатке средств")

	// Проверка БД (баланс не должен измениться)
	var dbBalance float64
	err = testDB.QueryRow("SELECT balance FROM wallets WHERE wallet_number = $1", walletNumber).Scan(&dbBalance)
	require.NoError(t, err)
	assert.InDelta(t, initialBalance, dbBalance, 0.001)
}

func TestWalletHandler_UpdateBalance_InvalidWalletNumber(t *testing.T) {
	cleanupTestDB(t)
	payload := models.UpdateBalanceRequest{
		WalletNumber: "invalid", // Некорректный номер
		Amount:       100,
	}
	req := createRequest(t, http.MethodPost, "/api/v1/wallets/balance", payload)
	rr := executeRequest(t, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Ожидался статус Bad Request (400)")

	// Проверяем сообщение об ошибке (может отличаться в зависимости от реализации)
	assert.Contains(t, rr.Body.String(), "Некорректный формат номера кошелька", "Ожидалось сообщение об ошибке формата")
}

func TestWalletHandler_ListWallets(t *testing.T) {
	cleanupTestDB(t)

	// Добавляем несколько кошельков
	walletsData := []models.Wallet{
		{Number: "1000001", Balance: 10},
		{Number: "1000002", Balance: 20.5},
		{Number: "1000003", Balance: 0},
	}
	for _, w := range walletsData {
		_, err := testDB.Exec("INSERT INTO wallets (wallet_number, balance) VALUES ($1, $2)", w.Number, w.Balance)
		require.NoError(t, err, "Не удалось добавить кошелек %s", w.Number)
	}

	req := createRequest(t, http.MethodGet, "/api/v1/wallets", nil)
	rr := executeRequest(t, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp models.ListWalletsResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Len(t, resp.Wallets, len(walletsData), "Количество кошельков в ответе не совпадает")

	// Проверяем наличие кошельков (порядок может быть не гарантирован)
	foundWallets := make(map[string]float64)
	for _, w := range resp.Wallets {
		foundWallets[w.Number] = w.Balance
	}

	for _, wData := range walletsData {
		balance, found := foundWallets[wData.Number]
		assert.True(t, found, "Кошелек %s не найден в ответе", wData.Number)
		assert.InDelta(t, wData.Balance, balance, 0.001, "Баланс кошелька %s не совпадает", wData.Number)
	}
}

func TestWalletHandler_ConvertAndDeduct_Success(t *testing.T) {
	cleanupTestDB(t)
	walletNumber := "2223344"
	initialBalance := 200.0
	rateValue := 90.5 // Пример курса
	amountToConvert := 1.5

	// Создаем кошелек
	_, err := testDB.Exec("INSERT INTO wallets (wallet_number, balance) VALUES ($1, $2)", walletNumber, initialBalance)
	require.NoError(t, err)
	// Добавляем курс
	_, err = testDB.Exec("INSERT INTO rates (value) VALUES ($1)", rateValue)
	require.NoError(t, err)

	payload := models.ConvertRequest{
		SourceWalletNumber: walletNumber,
		AmountToConvert:    amountToConvert,
		// FirstName, LastName, UserID - не используются в логике, но можно добавить
	}
	req := createRequest(t, http.MethodPost, "/api/v1/wallets/convert", payload)
	rr := executeRequest(t, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp models.ConvertResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	expectedRemainingBalance := initialBalance - amountToConvert
	expectedConvertedAmount := amountToConvert * rateValue

	assert.Equal(t, walletNumber, resp.SourceWalletNumber)
	assert.InDelta(t, expectedRemainingBalance, resp.RemainingBalance, 0.001)
	assert.InDelta(t, expectedConvertedAmount, resp.ConvertedAmount, 0.001)
	assert.InDelta(t, rateValue, resp.RateUsed, 0.001)
	assert.Contains(t, resp.Message, "успешно")

	// Проверка БД
	var dbBalance float64
	err = testDB.QueryRow("SELECT balance FROM wallets WHERE wallet_number = $1", walletNumber).Scan(&dbBalance)
	require.NoError(t, err)
	assert.InDelta(t, expectedRemainingBalance, dbBalance, 0.001)
}

func TestWalletHandler_ConvertAndDeduct_InsufficientFunds(t *testing.T) {
	cleanupTestDB(t)
	walletNumber := "5556677"
	initialBalance := 10.0
	rateValue := 90.0
	amountToConvert := 15.0 // Больше, чем на балансе

	_, err := testDB.Exec("INSERT INTO wallets (wallet_number, balance) VALUES ($1, $2)", walletNumber, initialBalance)
	require.NoError(t, err)
	_, err = testDB.Exec("INSERT INTO rates (value) VALUES ($1)", rateValue)
	require.NoError(t, err)

	payload := models.ConvertRequest{
		SourceWalletNumber: walletNumber,
		AmountToConvert:    amountToConvert,
	}
	req := createRequest(t, http.MethodPost, "/api/v1/wallets/convert", payload)
	rr := executeRequest(t, req)

	assert.Equal(t, http.StatusConflict, rr.Code) // Ожидаем 409

	var resp models.ConvertResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, walletNumber, resp.SourceWalletNumber)
	assert.Contains(t, resp.Message, "недостаточно средств на счете")
	// Баланс в ответе должен быть равен начальному
	assert.InDelta(t, initialBalance, resp.RemainingBalance, 0.001)
	// Остальные поля (ConvertedAmount, RateUsed) могут быть нулевыми или отсутствовать

	// Проверка БД (баланс не должен измениться)
	var dbBalance float64
	err = testDB.QueryRow("SELECT balance FROM wallets WHERE wallet_number = $1", walletNumber).Scan(&dbBalance)
	require.NoError(t, err)
	assert.InDelta(t, initialBalance, dbBalance, 0.001)
}

// TODO: Добавить тесты для других случаев ConvertAndDeduct:
// - Кошелек не найден (StatusNotFound)
// - Курс не найден (StatusServiceUnavailable)
// - Некорректный номер кошелька (StatusBadRequest)
// - Сумма конвертации <= 0 (StatusBadRequest)

// TODO: Добавить тесты для Rate Handler в отдельный файл rate_handler_test.go

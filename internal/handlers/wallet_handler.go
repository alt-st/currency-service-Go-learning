// internal/handlers/wallet_handler.go
package handlers

import (
	"currency-service/internal/models"
	"currency-service/internal/service"
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

// WalletHandler обрабатывает HTTP-запросы, связанные с кошельками.
type WalletHandler struct {
	walletService service.WalletService
}

// NewWalletHandler создает новый экземпляр обработчика кошельков.
func NewWalletHandler(svc service.WalletService) *WalletHandler {
	return &WalletHandler{walletService: svc}
}

// writeJSONResponse (без изменений)
func writeJSONResponse(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if payload != nil {
		err := json.NewEncoder(w).Encode(payload)
		if err != nil {
			log.Printf("Ошибка кодирования JSON ответа: %v\n", err)
		}
	}
}

// UpdateBalance godoc
// @Summary      Создать кошелек или обновить баланс
// @Description  Создает новый кошелек с указанным балансом (если сумма положительная) или обновляет баланс существующего кошелька. Положительная сумма - пополнение, отрицательная - списание. Списание с несуществующего кошелька или до отрицательного баланса невозможно.
// @Tags         Wallets
// @Accept       json
// @Produce      json
// @Param        balance_update body models.UpdateBalanceRequest true "Данные для обновления баланса"
// @Success      200  {object}  models.UpdateBalanceResponse "Баланс успешно обновлен"
// @Failure      400  {object}  models.ErrorResponse "Некорректный формат запроса, номера кошелька или суммы"
// @Failure      409  {object}  models.UpdateBalanceResponse "Конфликт бизнес-логики (например, недостаточно средств)"
// @Failure      500  {object}  models.ErrorResponse "Внутренняя ошибка сервера"
// @Router       /wallets/balance [post]
func (h *WalletHandler) UpdateBalance(w http.ResponseWriter, r *http.Request) {
	var req models.UpdateBalanceRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)
	if err != nil {
		log.Printf("Ошибка декодирования JSON (UpdateBalance): %v\n", err)
		writeJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "Некорректный формат запроса: " + err.Error()})
		return
	}

	resp, err := h.walletService.UpdateBalance(r.Context(), req)

	statusCode := http.StatusOK
	errorPayload := models.ErrorResponse{} // Используем для ошибок 400 и 500

	if err != nil {
		log.Printf("Ошибка из сервиса UpdateBalance: %v\n", err)
		switch {
		case errors.Is(err, service.ErrInvalidWalletNumber):
			statusCode = http.StatusBadRequest
			errorPayload.Error = err.Error()
		case errors.Is(err, service.ErrWithdrawNonExistent):
			statusCode = http.StatusBadRequest // Считаем это ошибкой запроса
			errorPayload.Error = err.Error()
		case errors.Is(err, service.ErrInsufficientFunds):
			statusCode = http.StatusConflict // 409 - возвращаем структуру ответа с текущим балансом
		case errors.Is(err, service.ErrNegativeDeposit):
			statusCode = http.StatusBadRequest
			errorPayload.Error = err.Error()
		default:
			statusCode = http.StatusInternalServerError
			errorPayload.Error = "Внутренняя ошибка сервера"
		}
	}

	// Отправляем ответ
	if statusCode == http.StatusOK || statusCode == http.StatusConflict {
		// Для 200 OK и 409 Conflict возвращаем структуру UpdateBalanceResponse
		writeJSONResponse(w, statusCode, resp)
	} else {
		// Для 400 и 500 возвращаем стандартную ErrorResponse
		writeJSONResponse(w, statusCode, errorPayload)
	}
}

// ListWallets godoc
// @Summary      Получить список всех кошельков
// @Description  Возвращает массив всех зарегистрированных кошельков с их балансами.
// @Tags         Wallets
// @Produce      json
// @Success      200  {object}  models.ListWalletsResponse "Список кошельков"
// @Failure      500  {object}  models.ErrorResponse "Внутренняя ошибка сервера"
// @Router       /wallets [get]
func (h *WalletHandler) ListWallets(w http.ResponseWriter, r *http.Request) {
	resp, err := h.walletService.ListWallets(r.Context())
	if err != nil {
		log.Printf("Ошибка из сервиса ListWallets: %v\n", err)
		writeJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Не удалось получить список кошельков"})
		return
	}
	writeJSONResponse(w, http.StatusOK, resp)
}

// ConvertAndDeduct godoc
// @Summary      Конвертировать и списать сумму с кошелька
// @Description  Получает самый свежий курс, конвертирует указанную сумму и списывает ее с баланса указанного кошелька. Возвращает остаток на счете и результат конвертации.
// @Tags         Wallets
// @Accept       json
// @Produce      json
// @Param        conversion_request body models.ConvertRequest true "Данные для конвертации и списания"
// @Success      200  {object}  models.ConvertResponse "Конвертация и списание прошли успешно"
// @Failure      400  {object}  models.ErrorResponse "Некорректный формат запроса, номера кошелька или суммы"
// @Failure      404  {object}  models.ErrorResponse "Указанный кошелек не найден"
// @Failure      409  {object}  models.ConvertResponse "Конфликт: недостаточно средств на кошельке"
// @Failure      500  {object}  models.ErrorResponse "Внутренняя ошибка сервера"
// @Failure      503  {object}  models.ErrorResponse "Не удалось получить актуальный курс валют"
// @Router       /wallets/convert [post]
func (h *WalletHandler) ConvertAndDeduct(w http.ResponseWriter, r *http.Request) {
	var req models.ConvertRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)
	if err != nil {
		log.Printf("Ошибка декодирования JSON (ConvertAndDeduct): %v\n", err)
		writeJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "Некорректный формат запроса: " + err.Error()})
		return
	}

	resp, err := h.walletService.ConvertAndDeduct(r.Context(), req)

	statusCode := http.StatusOK
	errorPayload := models.ErrorResponse{}

	if err != nil {
		log.Printf("Ошибка из сервиса ConvertAndDeduct: %v\n", err)
		switch {
		case errors.Is(err, service.ErrInvalidWalletNumber):
			statusCode = http.StatusBadRequest
			errorPayload.Error = err.Error()
		case err.Error() == "сумма для конвертации должна быть положительной":
			statusCode = http.StatusBadRequest
			errorPayload.Error = err.Error()
		case errors.Is(err, service.ErrWalletNotFound):
			statusCode = http.StatusNotFound
			errorPayload.Error = err.Error()
		case errors.Is(err, service.ErrInsufficientFunds):
			statusCode = http.StatusConflict // Возвращаем ConvertResponse с сообщением
		case errors.Is(err, service.ErrRateNotAvailable):
			statusCode = http.StatusServiceUnavailable
			errorPayload.Error = err.Error()
		default:
			statusCode = http.StatusInternalServerError
			errorPayload.Error = "Внутренняя ошибка сервера при конвертации"
		}
	}

	// Отправляем ответ
	if statusCode == http.StatusOK || statusCode == http.StatusConflict {
		writeJSONResponse(w, statusCode, resp)
	} else {
		writeJSONResponse(w, statusCode, errorPayload)
	}
}

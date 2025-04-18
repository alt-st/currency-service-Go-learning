// internal/handlers/rate_handler.go
package handlers

import (
	"currency-service/internal/models" // Импортируем модели для ссылок в аннотациях
	"currency-service/internal/service"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

// RateHandler обрабатывает HTTP-запросы, связанные с курсами валют.
type RateHandler struct {
	rateService service.RateService
}

// NewRateHandler создает новый экземпляр обработчика курсов валют.
func NewRateHandler(svc service.RateService) *RateHandler {
	return &RateHandler{rateService: svc}
}

// CreateRate godoc
// @Summary      Добавить новый курс валюты
// @Description  Принимает значение курса в теле запроса и сохраняет его.
// @Tags         Rates
// @Accept       json
// @Produce      json
// @Param        rate body models.Rate true "Данные для создания курса (нужно только поле 'value')" SchemaExample({\n \"value\": 95.5\n})
// @Success      201  {object}  models.SuccessResponse "Курс успешно добавлен"
// @Failure      400  {object}  models.ErrorResponse "Некорректный формат запроса или значение курса"
// @Failure      500  {object}  models.ErrorResponse "Внутренняя ошибка сервера"
// @Router       /rates [post]
func (h *RateHandler) CreateRate(w http.ResponseWriter, r *http.Request) {
	// Временная структура только для получения value из JSON
	var reqPayload struct {
		Value float64 `json:"value"`
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&reqPayload)
	if err != nil {
		log.Printf("Ошибка декодирования JSON (CreateRate): %v\n", err)
		writeJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "Некорректный формат запроса: " + err.Error()})
		return
	}

	// Вызов сервисного слоя
	err = h.rateService.CreateRate(r.Context(), reqPayload.Value)
	if err != nil {
		log.Printf("Ошибка при вызове сервиса CreateRate: %v\n", err)
		// TODO: Улучшить маппинг ошибок сервиса на HTTP статусы
		// Пока возвращаем 500 для всех ошибок сервиса, но можно проверять тип ошибки
		if err.Error() == "курс валюты должен быть положительным числом" {
			writeJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		} else {
			writeJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Внутренняя ошибка сервера"})
		}
		return
	}

	writeJSONResponse(w, http.StatusCreated, models.SuccessResponse{Message: "Курс успешно добавлен"})
}

// GetAverageRate godoc
// @Summary      Получить средний курс
// @Description  Возвращает среднее значение для последних N курсов валют.
// @Tags         Rates
// @Produce      json
// @Param        limit query int false "Количество последних курсов для расчета (по умолчанию 10)" minimum(1)
// @Success      200  {object}  models.AverageResponse "Средний курс и количество записей"
// @Failure      400  {object}  models.ErrorResponse "Некорректное значение параметра 'limit'"
// @Failure      500  {object}  models.ErrorResponse "Внутренняя ошибка сервера"
// @Router       /rates/average [get]
func (h *RateHandler) GetAverageRate(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 10 // Значение по умолчанию
	var err error
	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			log.Printf("Некорректное значение параметра limit: %s\n", limitStr)
			writeJSONResponse(w, http.StatusBadRequest, models.ErrorResponse{Error: "Некорректное значение параметра 'limit'"})
			return
		}
	}

	// Вызов сервисного слоя
	avgResponse, err := h.rateService.GetAverageRate(r.Context(), limit)
	if err != nil {
		log.Printf("Ошибка при вызове сервиса GetAverageRate: %v\n", err)
		writeJSONResponse(w, http.StatusInternalServerError, models.ErrorResponse{Error: "Внутренняя ошибка сервера"})
		return
	}

	writeJSONResponse(w, http.StatusOK, avgResponse)
}

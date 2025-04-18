// internal/handlers/tests/rate_handler_test.go
package handlers_test

import (
	"currency-service/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Тесты для Rate Handler ---
// Используют testRouter и testDB из main_test.go

func TestRateHandler_CreateRate_Success(t *testing.T) {
	cleanupTestDB(t) // Очищаем БД

	rateValue := 95.5
	payload := map[string]float64{"value": rateValue} // Простой map для запроса

	req := createRequest(t, http.MethodPost, "/api/v1/rates", payload)
	rr := executeRequest(t, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "Ожидался статус Created (201)")

	var resp map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Курс успешно добавлен", resp["message"])

	// Проверка БД
	var count int
	var value float64
	err = testDB.QueryRow("SELECT COUNT(*), value FROM rates WHERE value = $1 GROUP BY value", rateValue).Scan(&count, &value)
	require.NoError(t, err, "Курс должен существовать в БД")
	assert.Equal(t, 1, count, "Должен быть один курс с таким значением")
	assert.InDelta(t, rateValue, value, 0.001)
}

func TestRateHandler_CreateRate_InvalidValue(t *testing.T) {
	cleanupTestDB(t)

	// Попытка добавить некорректное значение (0 или отрицательное)
	// Сервис должен вернуть ошибку, но хендлер может вернуть 500, если сервис не обработал специфично
	// В нашей реализации сервис вернет ошибку, которую хендлер обернет в 500.
	// TODO: Улучшить обработку ошибок в хендлере CreateRate для возврата 400 Bad Request
	rateValue := -10.0
	payload := map[string]float64{"value": rateValue}

	req := createRequest(t, http.MethodPost, "/api/v1/rates", payload)
	rr := executeRequest(t, req)

	// Пока ожидаем 500, но в идеале должно быть 400
	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Ожидался статус Internal Server Error (500)")
	// Можно добавить проверку тела ошибки, если оно стандартизировано
}

func TestRateHandler_GetAverageRate(t *testing.T) {
	cleanupTestDB(t)

	// Добавляем несколько курсов
	rates := []float64{90.0, 91.0, 92.0, 93.0, 94.0}
	expectedSum := 0.0
	for _, r := range rates {
		_, err := testDB.Exec("INSERT INTO rates (value, timestamp) VALUES ($1, $2)", r, time.Now())
		require.NoError(t, err)
		expectedSum += r
		time.Sleep(1 * time.Millisecond) // Небольшая задержка для уникальности timestamp
	}
	expectedAverage := expectedSum / float64(len(rates))

	// Запрашиваем среднее для последних 3
	limit := 3
	req := createRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/rates/average?limit=%d", limit), nil)
	rr := executeRequest(t, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp models.AverageResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Среднее для последних 3: (92 + 93 + 94) / 3 = 279 / 3 = 93
	expectedLimitedAverage := (92.0 + 93.0 + 94.0) / 3.0
	assert.Equal(t, limit, resp.Count, "Количество курсов в расчете неверно")
	assert.InDelta(t, expectedLimitedAverage, resp.Average, 0.001, "Среднее значение рассчитано неверно")

	// Запрашиваем без лимита (должен использоваться дефолтный лимит 10, но у нас всего 5 записей)
	reqDefault := createRequest(t, http.MethodGet, "/api/v1/rates/average", nil)
	rrDefault := executeRequest(t, reqDefault)
	assert.Equal(t, http.StatusOK, rrDefault.Code)
	var respDefault models.AverageResponse
	errDefault := json.Unmarshal(rrDefault.Body.Bytes(), &respDefault)
	require.NoError(t, errDefault)
	assert.Equal(t, len(rates), respDefault.Count)
	assert.InDelta(t, expectedAverage, respDefault.Average, 0.001)

}

func TestRateHandler_GetAverageRate_InvalidLimit(t *testing.T) {
	cleanupTestDB(t)
	// Добавим один курс, чтобы было что считать
	_, err := testDB.Exec("INSERT INTO rates (value) VALUES ($1)", 100.0)
	require.NoError(t, err)

	// Некорректный лимит (строка)
	reqStr := createRequest(t, http.MethodGet, "/api/v1/rates/average?limit=abc", nil)
	rrStr := executeRequest(t, reqStr)
	assert.Equal(t, http.StatusBadRequest, rrStr.Code)
	assert.Contains(t, rrStr.Body.String(), "Некорректное значение параметра 'limit'")

	// Некорректный лимит (ноль)
	reqZero := createRequest(t, http.MethodGet, "/api/v1/rates/average?limit=0", nil)
	rrZero := executeRequest(t, reqZero)
	assert.Equal(t, http.StatusBadRequest, rrZero.Code)
	assert.Contains(t, rrZero.Body.String(), "Некорректное значение параметра 'limit'")
}

func TestRateHandler_GetAverageRate_NoRates(t *testing.T) {
	cleanupTestDB(t) // Убедимся, что таблица пуста

	req := createRequest(t, http.MethodGet, "/api/v1/rates/average?limit=5", nil)
	rr := executeRequest(t, req)

	assert.Equal(t, http.StatusOK, rr.Code) // Успешный ответ, даже если данных нет

	var resp models.AverageResponse
	err := json.Unmarshal(rr.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Count, "Количество должно быть 0")
	assert.Equal(t, 0.0, resp.Average, "Среднее должно быть 0")
}

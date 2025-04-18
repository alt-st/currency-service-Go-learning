// internal/handlers/tests/main_test.go
package handlers_test

import (
	"bytes"
	"currency-service/internal/config"
	"currency-service/internal/database"
	"currency-service/internal/handlers"
	"currency-service/internal/repository"
	"currency-service/internal/service"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq" // DB driver
	"github.com/stretchr/testify/require"
)

var (
	testRouter chi.Router
	testDB     *sql.DB
)

// TestMain выполняется один раз перед всеми тестами в пакете.
func TestMain(m *testing.M) {
	// 1. Загрузка тестовой конфигурации
	// !!! ВАЖНО: Убедитесь, что эти переменные указывают на вашу ТЕСТОВУЮ БД из docker-compose-test.yml !!!
	os.Setenv("DB_HOST", "localhost")         // Хост, где доступен контейнер db_test
	os.Setenv("DB_PORT", "5434")              // !!! ПОРТ, который вы пробросили для db_test (например, 5434) !!!
	os.Setenv("DB_USER", "test_user")         // Пользователь тестовой БД
	os.Setenv("DB_PASSWORD", "test_password") // Пароль тестовой БД
	os.Setenv("DB_NAME", "currency_db_test")  // Имя тестовой БД
	os.Setenv("SERVER_PORT", "8081")          // Тестовый порт сервера (не используется напрямую здесь)

	// Используем функцию загрузки конфига, которая читает переменные окружения
	cfg := config.LoadConfig()

	// 2. Подключение к тестовой БД
	var err error
	// Используем данные из cfg, которые были установлены через os.Setenv
	testDB, err = database.NewPostgresConnection(cfg.DB)
	if err != nil {
		log.Fatalf("Не удалось подключиться к тестовой базе данных (%s:%d): %v", cfg.DB.Host, cfg.DB.Port, err)
	}
	defer testDB.Close()

	log.Printf("Успешное подключение к тестовой БД: %s:%d, DB: %s", cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)

	// 3. Применение миграций к тестовой БД
	// Убедимся, что таблицы существуют перед очисткой и тестами
	if err := database.MigrateSchema(testDB); err != nil {
		log.Fatalf("Не удалось применить миграции к тестовой БД: %v", err)
	}
	log.Println("Миграции к тестовой БД успешно применены.")

	// 4. Инициализация зависимостей для тестов
	rateRepo := repository.NewPostgresRateRepository()
	walletRepo := repository.NewPostgresWalletRepository()
	rateSvc := service.NewRateService(rateRepo, testDB)
	walletSvc := service.NewWalletService(walletRepo, rateRepo, testDB)
	rateHandler := handlers.NewRateHandler(rateSvc)
	walletHandler := handlers.NewWalletHandler(walletSvc)

	// 5. Настройка роутера
	testRouter = chi.NewRouter()
	testRouter.Route("/api/v1", func(r chi.Router) {
		r.Route("/rates", func(r chi.Router) {
			r.Post("/", rateHandler.CreateRate)
			r.Get("/average", rateHandler.GetAverageRate)
		})
		r.Route("/wallets", func(r chi.Router) {
			r.Post("/balance", walletHandler.UpdateBalance)
			r.Get("/", walletHandler.ListWallets)
			r.Post("/convert", walletHandler.ConvertAndDeduct)
		})
	})

	// 6. Запуск тестов
	log.Println("Запуск тестов...")
	exitCode := m.Run()

	log.Println("Тесты завершены.")
	os.Exit(exitCode)
}

// --- Вспомогательные функции ---

// cleanupTestDB очищает таблицы перед/после каждого теста
func cleanupTestDB(t *testing.T) {
	t.Helper()
	// Очищаем таблицы в определенном порядке из-за возможных внешних ключей (если появятся)
	// Сначала таблицы, на которые могут ссылаться, потом основные.
	// RESTART IDENTITY сбрасывает счетчики SERIAL/IDENTITY.
	_, err := testDB.Exec("TRUNCATE TABLE wallets, rates RESTART IDENTITY;")
	require.NoError(t, err, "Ошибка очистки тестовой БД")
}

// createRequest создает тестовый HTTP запрос
func createRequest(t *testing.T, method, url string, body interface{}) *http.Request {
	t.Helper()
	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		require.NoError(t, err, "Ошибка маршалинга тела запроса")
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	require.NoError(t, err, "Ошибка создания тестового запроса")
	req.Header.Set("Content-Type", "application/json")
	return req
}

// executeRequest выполняет запрос к тестовому роутеру
func executeRequest(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	return rr
}

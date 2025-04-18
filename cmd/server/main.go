// cmd/server/main.go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"currency-service/internal/config"
	"currency-service/internal/database"
	"currency-service/internal/handlers"
	"currency-service/internal/repository"
	"currency-service/internal/service"

	// (!!!) Импорты для Swagger
	_ "currency-service/docs" // Путь к сгенерированному пакету docs

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq" // DB driver

	// (!!!) Используем стандартный http-swagger
	httpSwagger "github.com/swaggo/http-swagger"
)

// (!!!) Аннотации для основной информации API (без изменений)
// @title           Currency Service API
// @version         1.0
// @description     Сервис для управления курсами валют и кошельками.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @externalDocs.description  OpenAPI Spec
// @externalDocs.url          http://localhost:8080/swagger/doc.json

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

//go:generate swag init -g cmd/server/main.go -o ./docs --parseDependency --parseInternal
// Обновленная директива go:generate:
// -g ./cmd/server/main.go : Путь к main файлу относительно корня проекта
// -o ./docs               : Путь к папке docs относительно корня проекта
// --parseDependency       : Анализировать зависимости
// --parseInternal         : Анализировать внутренние папки (internal)

func main() {
	// --- Загрузка конфига, логгер, БД, миграции (без изменений) ---
	cfg := config.LoadConfig()
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Запуск currency-service...")
	db, err := database.NewPostgresConnection(cfg.DB)
	if err != nil {
		log.Fatalf("Не удалось подключиться к базе данных: %v", err)
	}
	defer db.Close()
	if err := database.MigrateSchema(db); err != nil {
		log.Fatalf("Не удалось применить миграции схемы: %v", err)
	}
	log.Println("Миграции схемы успешно применены.")

	// --- Инициализация слоев (без изменений) ---
	rateRepo := repository.NewPostgresRateRepository()
	walletRepo := repository.NewPostgresWalletRepository()
	rateSvc := service.NewRateService(rateRepo, db)
	walletSvc := service.NewWalletService(walletRepo, rateRepo, db)
	rateHandler := handlers.NewRateHandler(rateSvc)
	walletHandler := handlers.NewWalletHandler(walletSvc)

	// --- Настройка роутера (chi) ---
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// --- (!!!) Маршрут для Swagger UI ---
	// Используем стандартный httpSwagger.WrapHandler
	// Он возвращает http.Handler, который chi может использовать
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	log.Println("Swagger UI доступен по адресу: http://localhost:8080/swagger/index.html")

	// --- Маршруты API v1 (без изменений в логике) ---
	r.Route("/api/v1", func(r chi.Router) {
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

	// --- Health check (без изменений) ---
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		// ... (код health check)
		err := db.PingContext(r.Context())
		if err != nil {
			log.Printf("Health check failed: DB ping error: %v", err)
			http.Error(w, "Service Unavailable (DB Error)", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// --- Настройка и запуск HTTP-сервера (без изменений) ---
	serverAddr := fmt.Sprintf(":%d", cfg.Server.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// --- Graceful Shutdown (без изменений) ---
	go func() {
		// ... (код запуска сервера)
		log.Printf("Сервер запускается на %s\n", serverAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Ошибка запуска сервера: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	// ... (код graceful shutdown)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Получен сигнал %s, начинаем graceful shutdown...", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Ошибка при graceful shutdown: %v", err)
	}

	log.Println("Сервер успешно остановлен.")
}

// internal/config/config.go
package config

import (
	"log"
	"os"
	"strconv"
	// "github.com/spf13/viper" // Пример с Viper
)

type ServerConfig struct {
	Port int
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
}

type Config struct {
	Server ServerConfig
	DB     DBConfig
}

// LoadConfig загружает конфигурацию из переменных окружения (простой пример).
func LoadConfig() Config {
	// Используйте Viper или аналоги для более надежной загрузки
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	serverPort, _ := strconv.Atoi(getEnv("SERVER_PORT", "8080"))

	return Config{
		Server: ServerConfig{
			Port: serverPort,
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "user"),
			Password: getEnv("DB_PASSWORD", "password"),
			Name:     getEnv("DB_NAME", "currency_db"),
		},
	}
}

// getEnv вспомогательная функция для получения переменной окружения с значением по умолчанию.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	log.Printf("Переменная окружения '%s' не установлена, используется значение по умолчанию: '%s'\n", key, fallback)
	return fallback
}

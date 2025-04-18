// internal/models/rate.go
package models

import "time"

// Rate представляет запись о курсе валюты.
type Rate struct {
	ID        int64     `json:"-" db:"id"` // ID из БД
	Value     float64   `json:"value" db:"value"`
	Timestamp time.Time `json:"timestamp" db:"timestamp"`
}

// AverageResponse представляет ответ для запроса среднего курса.
type AverageResponse struct {
	Average float64 `json:"average"`
	Count   int     `json:"count"`
}

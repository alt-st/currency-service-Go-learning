// internal/models/common.go
package models

// ErrorResponse стандартная структура для ответа об ошибке API.
type ErrorResponse struct {
	Error string `json:"error" example:"Сообщение об ошибке"` // Содержит текст ошибки для клиента
}

// SuccessResponse стандартная структура для простого успешного ответа.
type SuccessResponse struct {
	Message string `json:"message" example:"Операция выполнена успешно"` // Сообщение об успехе
}

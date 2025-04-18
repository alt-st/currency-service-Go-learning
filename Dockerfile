# Stage 1: Build the Go application
FROM golang:1.24.2-alpine AS builder

WORKDIR /app

# СНАЧАЛА копируем ВСЕ файлы проекта (включая *.go, go.mod, go.sum)
COPY . .

# ТЕПЕРЬ загружаем зависимости (go.mod уже должен быть в /app)
RUN go mod download

# Компиляция Go-приложения
# Выходной файл будет /currency-service
# Собираем пакет из ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -a -installsuffix cgo -o /currency-service ./cmd/server

# Stage 2: Create the final, lightweight image
FROM alpine:latest

# Рабочая директория в финальном образе
WORKDIR /app 
# Можно использовать /root/ или /app, главное чтобы совпадало с CMD ниже

# Копируем ТОЛЬКО собранный бинарник из стадии builder
COPY --from=builder /currency-service .

EXPOSE 8080

# Запускаем бинарник (убедитесь, что путь совпадает с WORKDIR)
CMD ["./currency-service"]
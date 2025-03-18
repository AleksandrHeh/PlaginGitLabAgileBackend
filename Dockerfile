# Используем базовый образ для Go (версия 1.22 или выше)
FROM golang:1.23 AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем исходный код проекта в контейнер
COPY . .

# Переходим в директорию с основным кодом
WORKDIR /app/cmd/web

# Собираем приложение
RUN go build -o agile-board .

# Финальный образ
FROM alpine:latest

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем собранный бинарник
COPY --from=builder /app/cmd/web/ .

# Открываем порт
EXPOSE 4000

# Запускаем приложение
CMD ["./agile-board"]
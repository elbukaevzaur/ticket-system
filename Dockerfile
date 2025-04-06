# Этап сборки
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Устанавливаем необходимые зависимости для сборки с CGO
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Копируем файлы зависимостей
COPY go.mod go.sum* ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=1 GOOS=linux go build -a -o ticket-app .

# Финальный этап
FROM alpine:3.18

# Устанавливаем необходимые зависимости для SQLite
RUN apk add --no-cache libc6-compat sqlite-libs

WORKDIR /app

# Копируем собранное приложение
COPY --from=builder /app/ticket-app .

# Копируем шаблоны и статические файлы
COPY templates ./templates
COPY static ./static

# Создаем директорию для загрузок и устанавливаем права
RUN mkdir -p /app/static/uploads && chmod -R 755 /app/static

# Создаем директорию для базы данных
RUN mkdir -p /app/data && chmod 755 /app/data

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./ticket-app"]


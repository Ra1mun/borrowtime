# BorrowTime

Сервис безопасной одноразовой передачи конфиденциальных файлов с автоматическим уничтожением после скачивания или истечения срока действия.

## Возможности

- **Безопасная передача файлов** — загрузка файлов с генерацией уникальной ссылки для скачивания
- **Политики доступа** — настройка срока действия, ограничение количества скачиваний, привязка к email получателя
- **Двухфакторная аутентификация (2FA)** — TOTP для дополнительной защиты аккаунта
- **Журнал аудита** — полный лог действий: создание передач, скачивания, отзыв доступа, попытки несанкционированного доступа
- **Админ-панель** — управление пользователями, ролями, глобальными настройками и статистикой
- **Автоматический lifecycle** — планировщик удаляет просроченные передачи и файлы из хранилища
- **S3-совместимое хранилище (MinIO)** — файлы хранятся отдельно от БД

## Архитектура

```
┌──────────────┐       ┌──────────────┐       ┌──────────────┐
│   Frontend   │──────▶│   Backend    │──────▶│  PostgreSQL   │
│  React SPA   │  API  │   Go (chi)   │       │              │
│  (nginx)     │       │              │──────▶│    MinIO      │
└──────────────┘       │              │       │  (S3 storage) │
     :3000             │              │──────▶│    Redis      │
                       └──────────────┘       └──────────────┘
                            :8080
```

### Backend (Go)

- **Фреймворк**: [chi](https://github.com/go-chi/chi) — HTTP роутер
- **БД**: PostgreSQL 16 + миграции через [golang-migrate](https://github.com/golang-migrate/migrate)
- **Хранилище**: MinIO (S3-совместимое)
- **Аутентификация**: JWT (access + refresh) + TOTP 2FA
- **Хеширование паролей**: Argon2id
- **Документация API**: Swagger UI (`/swagger/index.html`)

### Frontend (React + TypeScript)

- **Сборка**: Vite
- **Роутинг**: React Router
- **Анимации**: Motion
- **Стили**: Чистый CSS

## Быстрый старт (Docker)

### Предварительные требования

- [Docker](https://docs.docker.com/get-docker/) и Docker Compose

### Запуск

```bash
# Клонировать репозиторий
git clone <url> && cd BorrowTime

# Запустить все сервисы
docker compose up --build -d
```

После запуска:

| Сервис            | URL                              |
|-------------------|----------------------------------|
| Веб-интерфейс    | http://localhost:3000             |
| API               | http://localhost:8080/api/v1      |
| Swagger UI        | http://localhost:8080/swagger/index.html |

### Остановка

```bash
docker compose down
```

Для полного удаления данных:

```bash
docker compose down -v
```

## Разработка

### Backend

```bash
cd backend

# Запустить инфраструктуру (БД, Redis, MinIO)
docker compose -f docker-compose.dev.yml up -d

# Запустить сервер
go run ./cmd/server
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

В режиме разработки Vite проксирует запросы `/api/*` на `http://localhost:8080`.

## API Endpoints

### Аутентификация

| Метод  | Путь                    | Описание                    |
|--------|-------------------------|-----------------------------|
| POST   | `/api/v1/auth/register` | Регистрация                 |
| POST   | `/api/v1/auth/login`    | Вход                        |
| GET    | `/api/v1/auth/me`       | Текущий пользователь        |
| POST   | `/api/v1/auth/refresh`  | Обновление токенов          |
| POST   | `/api/v1/auth/logout`   | Выход                       |
| POST   | `/api/v1/auth/2fa/*`    | Управление 2FA              |

### Передачи

| Метод  | Путь                      | Описание                    |
|--------|---------------------------|-----------------------------|
| POST   | `/api/v1/transfers`       | Создать передачу            |
| GET    | `/api/v1/transfers`       | Список своих передач        |
| GET    | `/api/v1/transfers/{id}`  | Детали передачи             |
| DELETE | `/api/v1/transfers/{id}`  | Отозвать доступ             |
| GET    | `/api/v1/s/{token}`       | Скачать файл по токену      |

### Аудит

| Метод  | Путь                      | Описание                    |
|--------|---------------------------|-----------------------------|
| GET    | `/api/v1/audit`           | Журнал аудита               |
| GET    | `/api/v1/audit/export`    | Экспорт в CSV               |

### Администрирование

| Метод  | Путь                          | Описание                    |
|--------|-------------------------------|-----------------------------|
| GET    | `/api/v1/users`               | Список пользователей        |
| GET    | `/api/v1/users/search?q=`     | Поиск пользователей         |
| PUT    | `/api/v1/users/{id}/role`     | Изменить роль               |
| DELETE | `/api/v1/users/{id}`          | Удалить пользователя        |
| GET    | `/api/v1/admin/settings`      | Настройки системы           |
| PUT    | `/api/v1/admin/settings`      | Обновить настройки          |
| GET    | `/api/v1/admin/stats`         | Статистика                  |

## Переменные окружения

| Переменная         | По умолчанию                           | Описание                   |
|--------------------|----------------------------------------|----------------------------|
| `POSTGRES_USER`    | `borrowtime`                           | Пользователь БД            |
| `POSTGRES_PASSWORD`| `borrowtime`                           | Пароль БД                  |
| `POSTGRES_DB`      | `borrowtime`                           | Имя БД                     |
| `JWT_SECRET`       | `change-me-in-production-secret-32ch`  | Секрет для подписи JWT     |
| `MINIO_ACCESS_KEY` | `minioadmin`                           | Логин MinIO                |
| `MINIO_SECRET_KEY` | `minioadmin`                           | Пароль MinIO               |
| `BASE_URL`         | `http://localhost:3000`                | Базовый URL для ссылок     |

## Структура проекта

```
BorrowTime/
├── docker-compose.yml          # Запуск всего стека
├── .env.example                # Шаблон переменных окружения
├── backend/
│   ├── Dockerfile
│   ├── cmd/server/             # Точка входа
│   ├── internal/
│   │   ├── config/             # Конфигурация из env
│   │   ├── domain/             # Доменные модели и ошибки
│   │   ├── handler/            # HTTP-хендлеры
│   │   ├── postgres/           # Реализация репозиториев
│   │   ├── repository/         # Интерфейсы репозиториев
│   │   ├── storage/            # Интерфейс хранилища файлов
│   │   └── usecase/            # Бизнес-логика (use cases)
│   └── docs/                   # Swagger-документация
└── frontend/
    ├── Dockerfile
    ├── nginx.conf              # Конфиг nginx (проксирование API)
    └── src/
        ├── api/                # HTTP-клиент к backend API
        ├── context/            # React-контексты (AuthContext)
        ├── components/         # UI-компоненты
        ├── pages/              # Страницы приложения
        └── styles/             # CSS-стили
```

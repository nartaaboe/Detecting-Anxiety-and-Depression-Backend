## MoodInsight Backend (Go)

Production-like REST API для SaaS “MoodInsight”: сохраняем тексты и результаты анализов в Postgres, отдаём историю и дашборды. Есть JWT auth (access+refresh), RBAC (user/admin), аудит админ-действий, миграции (embedded через `go:embed`), worker pool.

AI-инференс можно делать двумя способами:

1. **Классический**: Go backend создаёт `analysis` (queued) → worker вызывает Python inference service (HTTP) → сохраняем `analysis_results`.
2. **Через внешний анализатор**: frontend вызывает FastAPI микросервис (см. `../Detecting-Anxiety-and-Depression-AI`) → получает `{label,score,confidence,explanation}` → отправляет это в Go backend, и backend сохраняет результат в БД (без worker).

### Стек

- **Go**: 1.22+
- **Router**: `github.com/gorilla/mux`
- **DB**: PostgreSQL, `sqlx` + `pgx` driver
- **Auth**: JWT access + refresh, bcrypt, refresh sessions (token hash)
- **Validation**: `github.com/go-playground/validator/v10`
- **Migrations**: `golang-migrate/migrate` (встроены в бинарь, но CLI тоже возможен)
- **Logging**: `slog` (std)
- **Docker**: `Dockerfile` + `docker-compose.yml` (api + postgres + optional ai mock)

### Быстрый старт (локально, `go run main.go`)

1. Подготовь env:

```bash
cp .env.example .env
```

2. Подними Postgres (и опционально AI mock):

```bash
docker compose up -d db
docker compose --profile ai up -d ai
```

3. Запусти API:

```bash
go run main.go
```

При старте применятся миграции из `internal/migrations/*.sql`.

### Запуск через Docker Compose

```bash
docker compose up --build
```

Для “сквозного” AI (mock inference) добавь профиль:

```bash
docker compose --profile ai up --build
```

### Миграции через CLI (опционально)

Если установлен `migrate` CLI:

```bash
migrate -path internal/migrations -database "$DB_DSN" up
migrate -path internal/migrations -database "$DB_DSN" down 1
```

### Формат ответов

- **Успех**: `{ "data": ... }`
- **Ошибка**: `{ "error": { "code": "string", "message": "string" } }`

### Curl примеры

Регистрация:

```bash
curl -sS -X POST "http://localhost:8080/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}'
```

Логин:

```bash
curl -sS -X POST "http://localhost:8080/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}'
```

Положи access token в переменную (пример через `jq`):

```bash
ACCESS_TOKEN="$(curl -sS -X POST "http://localhost:8080/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"password123"}' | jq -r '.data.accessToken')"
```

Создать текст:

```bash
curl -sS -X POST "http://localhost:8080/texts" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"content":"I feel anxious lately and can’t sleep well."}'
```

Создать анализ по `text_id`:

```bash
curl -sS -X POST "http://localhost:8080/analyses" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"text_id":"<TEXT_UUID>","model_version":"baseline","threshold":0.5}'
```

Создать анализ “в один шаг” (создаёт `texts` + `analyses` в транзакции):

```bash
curl -sS -X POST "http://localhost:8080/analyses" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"content":"Sometimes I feel down and unmotivated.","model_version":"baseline","threshold":0.5}'
```

Создать анализ с **готовым результатом** (backend сохранит результат в БД и пометит анализ как `done`):

```bash
curl -sS -X POST "http://localhost:8080/analyses" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "content":"Sometimes I feel down and unmotivated.",
    "result":{
      "label":"medium",
      "score":0.72,
      "confidence":0.91,
      "explanation":{
        "key_phrases":["Sometimes","I","feel","down"],
        "top_sentences":["Sometimes I feel down and unmotivated"]
      }
    }
  }'
```

Список анализов (пагинация `limit/offset`, фильтры `status/label/from/to`):

```bash
curl -sS "http://localhost:8080/analyses?limit=20&offset=0&status=done" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Результат анализа:

```bash
curl -sS "http://localhost:8080/analyses/<ANALYSIS_UUID>/result" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

Дашборд:

```bash
curl -sS "http://localhost:8080/dashboard/summary" \
  -H "Authorization: Bearer ${ACCESS_TOKEN}"
```

### Admin RBAC (как получить первого admin)

По умолчанию все новые пользователи получают роль `user`. Чтобы сделать первого администратора, можно вручную обновить роль в БД:

```sql
-- 1) Найди user_id нужного пользователя:
SELECT id, email FROM users WHERE email = 'user@example.com';

-- 2) Поставь роль admin:
DELETE FROM user_roles WHERE user_id = '<USER_UUID>';
INSERT INTO user_roles (user_id, role_id)
SELECT '<USER_UUID>', id FROM roles WHERE name = 'admin';
```

После этого можно использовать `/admin/*` endpoints.

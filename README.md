# PR Reviewer Assignment Service

* Язык: **Go 1.22**
* БД: **PostgreSQL 16**
* Запуск: **docker-compose up** (приложение + Postgres)
* HTTP-порт сервиса: `8080`
* Контракт API: `openapi.yaml` в корне репозитория

---

## Функциональность

### Основные требования

Реализовано:

* Управление командами и участниками:

  * `POST /team/add` — создать команду с участниками (создаёт/обновляет пользователей, если команда ещё не существует; если уже существует — возвращает `TEAM_EXISTS`).
  * `GET /team/get` - получить команду с участниками.
  * `POST /users/setIsActive` — установить флаг активности пользователя (`is_active`).

* Работа с PR:

  * `POST /pullRequest/create`

    * создаёт PR со статусом `OPEN`;
    * автоматически назначает до **2** активных ревьюверов:

      * только из **команды автора**;
      * исключая **автора**;
      * исключая неактивных (`isActive=false`);
      * если кандидатов меньше 2 - назначает 0 или 1.
  * `POST /pullRequest/merge`

    * помечает PR как `MERGED`;
    * **идемпотентен** - повторные вызовы не падают и возвращают актуальное состояние PR;
    * после `MERGED` любые изменения списка ревьюверов запрещены (доменные правила).
  * `POST /pullRequest/reassign`

    * переназначает **конкретного** ревьювера на случайного активного участника из **его команды**;
    * новый ревьювер:

      * активен;
      * из той же команды;
      * не автор PR;
      * не дублирует уже назначенного другого ревьювера;
    * если:

      * PR уже `MERGED` → `409 PR_MERGED`;
      * пользователь не был ревьювером этого PR → `409 NOT_ASSIGNED`;
      * нет подходящего кандидата → `409 NO_CANDIDATE`.

* Просмотр PR по ревьюверу:

  * `GET /users/getReview`

    * возвращает список PR’ов, где пользователь назначен ревьювером (короткая форма `PullRequestShort`).

* Строгие ограничения:

  * Пользователь с `isActive = false` **никогда** не назначается на ревью.
  * После `MERGED` менять список ревьюверов нельзя (ни через `/reassign`, ни через массовую деактивацию).

---

## Дополнительные задания

Сделано:

1. **Эндпоинт статистики**

   * `GET /stats`
   * Возвращает:

     * `by_user`: по каждому пользователю

       * `authored` — сколько PR он создал;
       * `assigned_as_reviewer` — сколько PR’ов он ревьюит.
     * `by_pr`: по каждому PR

       * `reviewer_count` — сколько ревьюверов назначено.

2. **Метод массовой деактивации + безопасная переназначаемость открытых PR**

   * `POST /team/deactivateMembers`

     * тело: `{ "team_name": "...", "user_ids": ["u2","u3", ...] }`

       * если `user_ids` пустой/отсутствует — деактивируются **все активные** участники команды.
     * делает:

       1. Деактивирует указанных пользователей (`is_active=false`).
       2. Находит все **OPEN** PR, где они были ревьюверами.
       3. Для каждого такого PR:

          * оставляет ревьюверов, которые **не деактивированы**;
          * для каждого деактивированного ревьювера пытается найти замену:

            * активный участник команды;
            * не автор PR;
            * не уже назначенный ревьювер.
          * если кандидата нет — ревьювер просто убирается (кол-во ревьюверов уменьшается, но остаётся в диапазоне 0..2).
       4. Для замёрженных PR (`MERGED`) список ревьюверов не трогается.
     * Возвращает:

       * `deactivated_user_ids` — кого реально деактивировали;
       * `updated_pull_request_ids` — какие PR были обновлены.

3. **Нагрузочное тестирование**

   * Используется `k6`, сценарий `loadtest.js` (лежит в корне проекта).
   * Сценарий гоняет:

     * `GET /users/getReview`
     * `POST /pullRequest/merge` (идемпотентный merge)
     * `GET /stats`
   * Пример параметров:

     * `vus = 20`, `duration = 30s` (нагрузка сильно выше требуемых 5 RPS).
   * Результаты на моей машине:
    ```bash
     execution: local
        script: loadtest.js
        output: -

     scenarios: (100.00%) 1 scenario, 20 max VUs, 1m0s max duration (incl. graceful stop):
              * default: 20 looping VUs for 30s (gracefulStop: 30s)



  █ TOTAL RESULTS 

    checks_total.......: 16509   548.385185/s
    checks_succeeded...: 100.00% 16509 out of 16509
    checks_failed......: 0.00%   0 out of 16509

    ✓ getReview status 200
    ✓ merge status 200
    ✓ stats status 200

    HTTP
    http_req_duration..............: avg=2.82ms   min=472µs    med=1.75ms   max=51.4ms   p(90)=5.79ms  p(95)=8.66ms  
      { expected_response:true }...: avg=2.82ms   min=472µs    med=1.75ms   max=51.4ms   p(90)=5.79ms  p(95)=8.66ms  
    http_req_failed................: 0.00%  0 out of 16509
    http_reqs......................: 16509  548.385185/s

    EXECUTION
    iteration_duration.............: avg=109.21ms min=102.41ms med=106.64ms max=173.37ms p(90)=116.5ms p(95)=121.54ms
    iterations.....................: 5503   182.795062/s
    vus............................: 20     min=20         max=20
    vus_max........................: 20     min=20         max=20

    NETWORK
    data_received..................: 4.7 MB 156 kB/s
    data_sent......................: 1.9 MB 62 kB/s
    ```
   * Вывод: при нагрузке выше указанной в задании сервис укладывается в SLI.

4. **E2E-тестирование**

   * Файл: `test/e2e/e2e_test.go`.
   * Сценарий `TestEndToEnd_CreateTeam_PR_Assign_Merge_ReassignOnMerged` проверяет:

     1. Создание команды `/team/add`.
     2. Создание PR `/pullRequest/create` и назначение 1–2 ревьюверов (не автора).
     3. То, что ревьювер видит PR в `/users/getReview`.
     4. Merge PR `/pullRequest/merge` и смену статуса на `MERGED`.
     5. Попытка `/pullRequest/reassign` на `MERGED` PR даёт `409 PR_MERGED`.

---

## Архитектура и структура проекта

* **domain** — чистые доменные модели и бизнес-правила;
* **repository** — работа с Postgres;
* **service** — usecase-слой (бизнес-логика);
* **transport/http** — HTTP API, маппинг DTO ↔ домен, маппинг ошибок ↔ коды/enum’ы;
* **config** — загрузка конфигурации;
* **stats** — агрегирующая статистика;
* **test/e2e** — end-to-end тесты.

Дерево (упрощённо):

```text
.
├── cmd/
│   └── app/
│       └── main.go               # входная точка, wiring зависимостей
├── internal/
│   ├── config/
│   │   └── config.go             # HTTP_PORT, DB_DSN из env
│   ├── domain/
│   │   ├── user.go               # User, UserID, базовые инварианты
│   │   ├── team.go               # Team, TeamName, TeamMember
│   │   └── pull_request.go       # PullRequest, PullRequestID, статус OPEN/MERGED, доменные методы:
│   │                             #   NewPullRequest, SetReviewers, ReplaceReviewer, Merge, CanModifyReviewers
│   ├── repository/
│   │   └── postgres/
│   │       ├── user_repo.go      # работа с таблицей users
│   │       ├── team_repo.go      # работа с teams (+ upsert членов)
│   │       ├── pr_repo.go        # pull_requests + pull_request_reviewers
│   │       └── stats_repo.go     # агрегирующие запросы для /stats
│   ├── service/
│   │   ├── user_service.go       # SetIsActive, GetUser
│   │   ├── team_service.go       # CreateTeamWithMembers, GetTeam, ListActiveMembers,
│   │   │                         # DeactivateMembersAndReassignOpenPRs
│   │   └── pr_service.go         # CreatePR, MergePR, ReassignReviewer, ListPRsByReviewer
│   ├── stats/
│   │   └── stats_service.go      # StatsService, Snapshot (ByUser, ByPR)
│   └── transport/
│       └── http/
│           ├── router.go         # регистрация всех хендлеров и /health
│           ├── middleware.go     # логирование, recovery, маппинг ошибок -> ErrorResponse
│           ├── team_handler.go   # /team/add, /team/get, /team/deactivateMembers
│           ├── user_handler.go   # /users/setIsActive, /users/getReview
│           ├── pr_handler.go     # /pullRequest/create|merge|reassign
│           └── stats_handler.go  # /stats
├── migrations/
│   └── 0001_init.sql             # up-миграции: создание схемы БД
├── migrations_down/
│   └── 0001_init_down.sql        # down-миграции (DROP TABLE ...) — не монтируются в Docker
├── test/
│   └── e2e/
│       └── e2e_test.go           # end-to-end тест HTTP API на живом сервисе
├── loadtest.js                   # сценарий нагрузочного теста (k6)
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── openapi.yaml                  # спецификация API (задана в условии)
```

---

## Доменные решения и допущения

### PR и ревьюверы

* **Максимум ревьюверов на PR** — 2 (по требованию).
* При создании PR:

  * берутся все активные участники команды автора, кроме автора;
  * случайным образом выбираются до 2 ревьюверов;
  * если доступных кандидатов меньше 2 — назначаем 0 или 1.
* При ручном переназначении (`/pullRequest/reassign`):

  * кандидат выбирается из **команды старого ревьювера**;
  * новый ревьювер:

    * активен;
    * не автор;
    * не старый ревьювер;
    * не уже назначенный другой ревьювер в этом PR;
  * если подходящего кандидата нет → `409 NO_CANDIDATE`.
* После перехода PR в `MERGED`:

  * любой вызов, пытающийся изменить список ревьюверов (`/reassign`, массовая деактивация), возвращает `PR_MERGED` или просто пропускает PR.

### Массовая деактивация

* `POST /team/deactivateMembers`:

  * если `user_ids` пустой/не передан → деактивируются **все активные** участники команды;
  * если передан список — деактивируются только те, кто реально есть в этой команде и активен.
* Открытые PR:

  * для каждого OPEN PR, где есть деактивированный ревьювер:

    * он убирается из списка ревьюверов;
    * если возможно — подбирается новый по тем же правилам, что в `/reassign`;
    * в итоге PR может остаться с 0/1/2 ревьюверами.
* Закрытые (`MERGED`) PR:

  * не трогаются, чтобы не нарушать историю ревью.

### Ошибки и коды

Маппинг доменных ошибок → `ErrorResponse.error.code`:

* `TEAM_EXISTS` — при попытке `POST /team/add` для уже существующей команды.
* `PR_EXISTS` — при `create` PR с существующим `pull_request_id`.
* `PR_MERGED` — попытка изменить ревьюверов для `MERGED` PR.
* `NOT_ASSIGNED` — пользователь не был ревьювером указанного PR.
* `NO_CANDIDATE` — нет подходящих активных кандидатов для замены ревьювера.
* `NOT_FOUND` — ресурс не найден (пользователь, команда, PR) — общая 404.

---

## Запуск

### Требования

* Docker / Docker Compose
* Go 1.22+ (если запускать локально без Docker)
* k6 (для нагрузочного тестирования, опционально)

### Запуск в Docker (рекомендуемый)

```bash
docker-compose up --build
```

Сервис будет доступен по адресу:

* `http://localhost:8080`

Проверка health:

```bash
curl http://localhost:8080/health
```

### Локальный запуск без Docker

1. Запустить PostgreSQL локально (или через Docker) и настроить переменную окружения `DB_DSN`, например:

```bash
export DB_DSN="postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
export HTTP_PORT=8080
```

2. Применить миграции (можно скопировать sql и выполнить через psql):

```bash
psql -U postgres -d postgres -f migrations/0001_init.sql
```

3. Запустить сервис:

```bash
go run ./cmd/app
```

---

## Тестирование

### Unit / простые тесты

Команда запуска всех тестов:

```bash
go test ./...
```

### E2E-тесты

1. Поднять сервис:

```bash
docker-compose up --build
```

2. Запустить E2E:

```bash
go test ./test/e2e -v
```

Тест проверит:

* создание команды;
* создание PR и автозаполнение ревьюверов;
* работу `/users/getReview`;
* идемпотентный `/pullRequest/merge`;
* запрет `/pullRequest/reassign` на `MERGED` PR.

### Нагрузочное тестирование (k6)

1. Установить k6 на macOS:

```bash
brew install k6
```
на Windows:
```powershell
winget install k6 --source winget
```
на Linux:
```bash
sudo apt install k6
```
2. Убедиться, что сервис поднят (`docker-compose up`).

3. Запустить тест:

```bash
k6 run loadtest.js
```

k6 будет в цикле выполнять:

* `GET /users/getReview?user_id=u2`
* `POST /pullRequest/merge` (идемпотентный merge PR)
* `GET /stats`

В выводе смотреть:

* `http_req_failed` — доля неуспешных запросов (должна быть ≈ 0%);
* `http_req_duration` — среднее и перцентили (`p(95)`, `p(99)`), ожидаемо < 300 ms.

---

## Миграции

### Up

* `migrations/0001_init.sql` — создаёт таблицы:

  * `teams(team_name PK)`
  * `users(user_id PK, username, team_name FK -> teams, is_active)`
  * `pull_requests(pull_request_id PK, pull_request_name, author_id FK -> users, status, created_at, merged_at)`
  * `pull_request_reviewers(pull_request_id FK -> pull_requests, reviewer_id FK -> users, PK (pull_request_id, reviewer_id))`

В Docker:

* каталог `migrations/` монтируется в `/docker-entrypoint-initdb.d`, и Postgres применяет скрипты автоматически при первом старте.

### Down

* `migrations_down/0001_init_down.sql` — содержит обратные операции:

  ```sql
  DROP TABLE IF EXISTS pull_request_reviewers;
  DROP TABLE IF EXISTS pull_requests;
  DROP TABLE IF EXISTS users;
  DROP TABLE IF EXISTS teams;
  ```

Down-миграции **не** монтируются в Docker и могут применяться вручную при необходимости.

---

## Makefile

В корне лежит простой `Makefile`, который упрощает типичные действия:

```make
make build   
make run      
make test       
make docker-build
make docker-run
```
---

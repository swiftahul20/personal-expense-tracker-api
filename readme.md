# Expense Tracker API

A REST API for tracking personal expenses, built in Go while learning the language. Started as a CLI tool, evolved into a full REST API backed by PostgreSQL with JWT authentication, refresh token rotation, and per-user data isolation.

## Features

- **Full CRUD** for expenses (amount, category ID, sub-category ID, description, date)
- **User-managed categories** with nested sub-categories and full CRUD operations
- **Pagination and filtering** on the expense list (page/limit, category, text search, date range; limit capped at 100)
- **CSV export** for filtered expenses
- **Category, monthly, and daily summaries** — each includes both totals and the underlying list of expenses for that
- **Swagger/OpenAPI documentation** — interactive API explorer at `/swagger/index.html`, generated via `swaggo/swag` from code annotations
- **Combined dashboard endpoint** — expenses + all three summaries in a single response
- **JWT authentication** — short-lived access tokens + long-lived refresh tokens, rotated on every use
- **Logout** — revokes a refresh token server-side
- **Current user endpoint** — fetch the authenticated user's own profile
- **Per-user data isolation** — enforced at the database query level, not just in application logic
- **Rate limiting** on login attempts (per-IP, in-memory)
- **Input validation** on all writes (amount > 0, required category, no future dates)
- **Dockerized** — the API and PostgreSQL both run via a single `docker compose up`

## Tech Stack

- **Language:** Go 1.27+
- **Router:** [chi](https://github.com/go-chi/chi)
- **Database:** PostgreSQL, via [pgx](https://github.com/jackc/pgx)
- **Auth:** [golang-jwt](https://github.com/golang-jwt/jwt) + bcrypt password hashing
- **Config:** environment variables via `.env` ([godotenv](https://github.com/joho/godotenv)); `PORT` defaults to `8080`
- **Containerization:** Docker, Docker Compose (multi-stage build)
- **API Docs:** [swaggo/swag](https://github.com/swaggo/swag) (OpenAPI/Swagger generation)
- **Deployment:** [Aiven](https://aiven.io) (managed PostgreSQL + API)

## Architecture

The project follows a layered structure with domain logic decoupled from storage and transport:

```
cmd/
└── rest-server/       # entry point, wires config, DB pool, handlers, routes
internal/
├── expense/           # domain model, filters, Store interface, Postgres implementation, validation
├── category/          # category/sub-category model, Store interface, Postgres implementation
├── user/              # user domain, Store interface, Postgres implementation
├── auth/              # password hashing, JWT issuing/verification, refresh tokens, middleware
├── ratelimit/         # in-memory per-IP rate limiter
├── report/            # types (types.go) + pure aggregation logic (summary.go) — by category/month/day
├── rest/              # HTTP handlers and route wiring
└── config/            # environment-based configuration loading
```

**Key design decision:** both `expense.Store` and `user.Store` are interfaces, with PostgreSQL as the only current implementation. Handlers depend only on these interfaces, never on Postgres directly — this keeps storage swappable and testable without touching business logic.

## Getting Started

### Prerequisites

- Docker and Docker Compose
- Go 1.27+ (only required when running the server outside Docker)

### Setup

1. Clone the repo and create a `.env` file in the project root:

   ```
   DATABASE_URL=postgres://expense_user:expense_pass@postgres:5432/expense_tracker
   # For a managed Postgres provider (e.g. Aiven), use the provided connection string with sslmode=require:
   DATABASE_URL=postgres://<user>:<password>@<host>:<port>/<database>?sslmode=require
   JWT_SECRET=<a long random string>
   JWT_TTL_HOURS=1
   REFRESH_TTL_DAYS=7
   ```

2. Start everything:

   ```bash
   docker compose up --build
   ```

   This builds the Go server image and starts both the API and PostgreSQL, waiting for the database to be healthy before the API starts.

3. Create the schema (first run only):

   ```bash
   docker exec -it expense-postgres psql -U expense_user -d expense_tracker
   ```

   ```sql
   CREATE TABLE users (
       id SERIAL PRIMARY KEY,
       email VARCHAR(255) UNIQUE NOT NULL,
       password_hash VARCHAR(255) NOT NULL,
       created_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );

   CREATE TABLE categories (
       id SERIAL PRIMARY KEY,
       user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
       name VARCHAR(100) NOT NULL,
       UNIQUE (user_id, name)
     );

   CREATE TABLE sub_categories (
       id SERIAL PRIMARY KEY,
       category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
       name VARCHAR(100) NOT NULL,
       UNIQUE (category_id, name)
     );

   CREATE TABLE expenses (
       id SERIAL PRIMARY KEY,
       user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
       amount NUMERIC(12, 2) NOT NULL,
       category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
       sub_category_id INTEGER REFERENCES sub_categories(id) ON DELETE SET NULL,
       description TEXT,
       date TIMESTAMPTZ NOT NULL
   );

   CREATE TABLE refresh_tokens (
       id SERIAL PRIMARY KEY,
       user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
       token_hash VARCHAR(255) NOT NULL UNIQUE,
       expires_at TIMESTAMPTZ NOT NULL,
       created_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );
   ```

The API does not run migrations automatically. Run the schema statements once against the PostgreSQL database before using the protected endpoints. If the database already contains the older expense schema, migrate it to the category and sub-category columns before starting the API.

The API is now available at `http://localhost:8080`.

### API Documentation

Once the server is running, interactive Swagger docs are available at:
http://localhost:8080/swagger/index.html

For protected endpoints, click **Authorize** and enter your access token in the format `Bearer <token>`.

To regenerate the docs after adding or changing annotations:

```bash
swag init -g cmd/rest-server/main.go
```

### Stopping

```bash
docker compose down       # stop containers, keep data
docker compose down -v    # stop containers and wipe the database volume
```

### Live Deployment

The API is deployed and publicly accessible:

[API Health Check](https://01a0d77e-2ac4-781d-a870-26f4e9a39a72-8080.eur-1.aiven.app/health)

- **Hosting:** [Aiven](https://aiven.io) (Docker-based web service, free tier)
- **Database:** [Aiven](https://aiven.io) (managed PostgreSQL, free tier, requires `sslmode=require`)

Note: Aiven's free tier spins down after 15 minutes of inactivity — the first request after idle time may take 10–30 seconds to respond while the service wakes up.

Swagger docs for the live API: `https://01a0d77e-2ac4-781d-a870-26f4e9a39a72-8080.eur-1.aiven.app/swagger/index.html`

## API Reference

### Auth

| Method | Endpoint         | Auth required | Description                                                               |
| ------ | ---------------- | :-----------: | ------------------------------------------------------------------------- |
| POST   | `/auth/register` |      No       | Create an account                                                         |
| POST   | `/auth/login`    |      No       | Log in, receive access + refresh tokens (rate-limited)                    |
| POST   | `/auth/refresh`  |      No       | Exchange a refresh token for a new token pair (rotates the refresh token) |
| POST   | `/auth/logout`   |      No       | Revoke a refresh token                                                    |
| GET    | `/auth/me`       |      Yes      | Get the authenticated user's profile                                      |

### Health

| Method | Endpoint  | Auth required | Description                                      |
| ------ | --------- | :-----------: | ------------------------------------------------ |
| GET    | `/health` |      No       | Check API availability and database connectivity |

### Expenses

All routes below require `Authorization: Bearer <access_token>`.

| Method | Endpoint           | Description                                                      |
| ------ | ------------------ | ---------------------------------------------------------------- |
| GET    | `/expenses`        | List the authenticated user's expenses, paginated and filterable |
| POST   | `/expenses`        | Create an expense                                                |
| GET    | `/expenses/export` | Download filtered expenses as `expenses.csv`                     |
| GET    | `/expenses/{id}`   | Get a single expense                                             |
| PUT    | `/expenses/{id}`   | Partially update an expense                                      |
| DELETE | `/expenses/{id}`   | Delete an expense                                                |

`GET /expenses` supports these optional query parameters:

- `page` and `limit` (default `1` and `20`; maximum limit `100`)
- `category_id` to filter by category
- `search` to search descriptions case-insensitively
- `from` and `to` using `YYYY-MM-DD` date boundaries

`GET /expenses/export` accepts the same filters and returns CSV with the columns `ID`, `Amount`, `Category`, `Sub-Category`, `Description`, and `Date`.

`GET /expenses` response shape:

```json
{
  "expenses": [ ... ],
  "page": 1,
  "limit": 20,
  "total": 47,
  "total_pages": 3
}
```

### Summaries

| Method | Endpoint            | Description                                                 |
| ------ | ------------------- | ----------------------------------------------------------- |
| GET    | `/summary/category` | Totals grouped by category, including each group's expenses |
| GET    | `/summary/month`    | Totals grouped by month, including each group's expenses    |
| GET    | `/summary/day`      | Totals grouped by day, including each group's expenses      |
| GET    | `/dashboard`        | Expenses + all three summaries combined in one response     |

### Categories

All category routes require `Authorization: Bearer <access_token>`.

| Method | Endpoint                          | Description                                |
| ------ | --------------------------------- | ------------------------------------------ |
| GET    | `/categories`                     | List categories with nested sub-categories |
| POST   | `/categories`                     | Create a category                          |
| PUT    | `/categories/{id}`                | Rename a category                          |
| DELETE | `/categories/{id}`                | Delete a category                          |
| POST   | `/categories/{id}/sub-categories` | Create a sub-category                      |
| PUT    | `/sub-categories/{id}`            | Rename a sub-category                      |
| DELETE | `/sub-categories/{id}`            | Delete a sub-category                      |

Category and sub-category write requests use `{ "name": "..." }`. Category names are scoped to the authenticated user. Deleting a category or sub-category sets matching expense references to `null`.

## Example: Register → Login → Create Category → Create Expense

```bash
# Register
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "you@example.com", "password": "password123"}'

# Response includes access_token and refresh_token

# Create a category and note its returned id
curl -X POST http://localhost:8080/categories \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <access_token>" \
  -d '{"name": "Food"}'

# Create an expense
curl -X POST http://localhost:8080/expenses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <access_token>" \
  -d '{"amount": 50000, "category_id": 1, "description": "Lunch", "date": "2026-09-21T00:00:00Z"}'
```

## Notes

This project started as a CLI tool for learning Go fundamentals (structs, interfaces, file I/O) before evolving into this REST API. A GraphQL API was originally planned alongside REST for comparison purposes but was dropped in favor of focusing on backend fundamentals (auth, security, containerization).

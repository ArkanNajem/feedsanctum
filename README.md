# FeedSanctum
[![Tests](https://github.com/ArkanNajem/feedsanctum/actions/workflows/test.yaml/badge.svg)](https://github.com/ArkanNajem/feedsanctum/actions/workflows/test.yaml)

FeedSanctum is an RSS aggregator and REST API backend written in Go. Beyond CRUD endpoints for users, feeds, and subscriptions, it runs a background scraper that fans out across feeds on a timer, handles duplicate posts through Postgres constraints, and tolerates malformed feed dates. Built on Chi, sqlc, goose, and PostgreSQL, with JWT auth, refresh-token rotation, and graceful shutdown.

A live instance runs on Render. See Live Demo below to try it in your browser.

Full OpenAPI 3.0 spec is included. Interactive reference is served by the API at `/docs` — [live](https://feedsanctum.onrender.com/docs), or locally at `http://localhost:8081/docs` when running with `go run` or Docker Compose.

---

## 🌐 Live Demo

**Try the API:** [https://feedsanctum.onrender.com/docs](https://feedsanctum.onrender.com/docs)

**Demo credentials:**
- Email: `demo@example.com`
- Password: `feedsanctum-demo-2026`

The demo account is pre-seeded with 4 RSS feeds (Hacker News, Wired, Ars Technica, and TechCrunch) and their scraped posts. Log in via `POST /api/v1/login` (or the **Authorize** button in Swagger UI) to explore authenticated endpoints.

### Quick Start

1. Open the [live API reference](https://feedsanctum.onrender.com/docs)
2. Call `POST /api/v1/login` with the demo credentials. Copy the `access_token` and `refresh_token` from the response.
3. Click **Authorize** in Swagger UI. Paste the access token into `bearerAuth` (no `Bearer ` prefix needed) and the refresh token into `refreshTokenAuth`.
4. Try `GET /api/v1/feeds` or `GET /api/v1/posts` to see scraped data.

No installation required.

## 🚀 Getting Started

### Prerequisites

- [Go](https://go.dev/doc/install) (v1.25 or later)
- [Docker](https://www.docker.com/) & [Docker Compose](https://docs.docker.com/compose/)
- [goose](https://github.com/pressly/goose) for database migrations


### Clone & Configure Environment

Clone the repository and create a `.env` file in the root directory:

```bash
git clone https://github.com/ArkanNajem/feedsanctum.git
cd feedsanctum
```

`.env`:
```env
PORT=8081
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=feedsanctum
TEST_DB_URL=postgres://postgres:yourpassword@localhost:5432/feedsanctum_test?sslmode=disable
JWT_SECRET=yoursecret
```


Start PostgreSQL:
```bash
docker compose up -d
```

If you want to run the test suite, create the test database first:
```bash
docker compose exec db psql -U postgres -c "CREATE DATABASE feedsanctum_test;"
```

Apply schema migrations using goose and run the API server:
```bash
goose -dir sql/migrations postgres "postgres://postgres:yourpassword@localhost:5432/feedsanctum?sslmode=disable" up
go run cmd/server/main.go
```
Interactive docs at http://localhost:8081/docs.

To run tests:
```bash
go test -p 1 ./...
```
**Note:** `-p 1` runs packages sequentially. The shared test database doesn't support parallel packages.

---

## API Documentation

Interactive reference is served by the API itself at [`/docs`](http://localhost:8081/docs) or on the [live deployment](https://feedsanctum.onrender.com/docs).

The full OpenAPI 3.0 spec is embedded in the binary from [`internal/docs/openapi.yaml`](./internal/docs/openapi.yaml).

---

## 📐 Key Design Decisions & Trade-Offs

### 1. Atomic Operations via Database Transactions
Multi-step database state changes are wrapped in explicit database transactions:
- **Feed Creation (`HandlerCreateFeed`):** Creating a feed and automatically following it must be an atomic operation. If inserting the `feed_follows` fails, the entire transaction is rolled back, preventing orphaned feed records without owners.
- **Refresh Token Rotation (`HandlerRefresh`):** During token rotation, revoking the existing refresh token and persisting the newly issued refresh token must succeed together. A transaction prevents race conditions where a user loses their session mid-rotation or an old token remains valid after failure.

### 2. Mitigation of Timing Attacks on Authentication
A server that immediately returns `401 Unauthorized` when an email does not exist responds significantly faster than when performing an expensive cryptographic hash comparison for an existing user. 
- To prevent **User Enumeration via Timing Attacks**, `HandlerLogin` executes an Argon2id comparison against a dummy hash whenever a user lookup returns no rows. 
- This ensures constant-time response latency regardless of whether an email is registered.

### 3. Error Handling Trade-Offs in Background Scraping
The `scrapeFeed` background worker prioritizes system resiliency over fail-fast behavior:
- **Batch Processing:** When parsing a feed with multiple items, a single malformed post or duplicate URL does not abort the batch. Instead, individual errors are logged, the failed post is skipped, and remaining valid posts in the feed are processed and inserted.
- **Feed Polling Safety:** The feed's `last_fetched_at` timestamp is updated immediately before post insertion. This prevents a persistently broken feed or corrupted post payload from trapping the scraper in an infinite retry loop during subsequent fetch cycles.

### 4. Decoupled API Contracts from Database Models
Handlers explicitly define custom `requestParam` and `response` struct types rather than directly embedding or exposing `sqlc`-generated database structs:
- **Encapsulation & Security:** Internal schema details are never inadvertently leaked to clients via JSON serialization.
- **Decoupled Contracts:** Changes to the underlying database schema or migrations do not directly break external client contracts, allowing independent evolution of the API and database layers.
- **Null-Value Handling:** Custom response types map `sql.NullString` and `sql.NullTime` into clean nullable JSON primitives (e.g., `*string`, `*time.Time`) rather than exposing raw database struct types to the caller.

### 5. Layered Rate Limiting for Authentication
- **Global per-IP+endpoint (60/min):** blunts general abuse and scripted scraping.
- **Route-level per-IP on `/login` (5/min):** caps single-source brute force attempts.
- **In-handler per-email on `/login` (8/min):** stops distributed attempts against one account, where each IP stays under its own limit but the target account receives hundreds of attempts.

### 6. Scrape-on-Create with Periodic Reconciliation

New feeds are scraped immediately in a background goroutine after the HTTP response is sent.
- **Responsive UX:** A user adding a feed sees posts within seconds, not after the next tick.
- **Failure isolation:** If the initial scrape fails (network, malformed feed), the feed is still created successfully; the ticker retries on the next cycle.
- **Server-scoped context:** The scraper goroutine inherits the process context, so in-flight scrapes abort cleanly on shutdown rather than being cancelled the moment the HTTP handler returns.

---

## 🏗️ System Architecture

```mermaid
graph TD
    Client[HTTP Client] -->|REST API Requests| Router[Chi Router]
    Router -->|MiddlewareAuth| Handlers[HTTP Handlers]
    Handlers -->|sqlc Queries| DB[(PostgreSQL)]
    
    subgraph Background Service
        Scraper[Scraper Worker] -->|Fetch RSS XML| ExternalFeeds[External RSS Feeds]
        Scraper -->|Insert New Posts| DB
    end
```

---

## 🌟 Key Features

- **Concurrent Feed Scraping:** Runs a background scraper that polls feeds on a configurable `time.Ticker` interval, fetching multiple RSS feeds concurrently.
- **Feed Parsing:** Parses RSS 2.0 XML structures, unescapes HTML entities, and handles multiple `pubDate` formats (RFC1123, RFC3339, etc.).
- **Authentication:** User registration and authentication by **Argon2id** password hashing and **JWT** for HTTP authorization.
- **Transactional DB Operations:** Uses PostgreSQL transactions to guarantee atomicity when creating a feed and its follow row, and when rotating refresh tokens.
- **Type-Safe Database Access:** Type-safe SQL query generation using **sqlc** and schema migrations managed via **goose**.
- **Integration Test Suite:** Runs migrations against an isolated Postgres instance via **goose**, with coverage of the scraper logic, JWT auth middleware, and the create feed handler.
- **Multi-Layer Rate Limiting:** Per-IP+endpoint global limits, stricter per-IP limits on authentication, and per-email.

---

## 🛠️ Tech Stack

- **Language:** [Go 1.25](https://go.dev/)
- **HTTP Router:** [Chi Router](https://github.com/go-chi/chi)
- **Rate Limiting:** [httprate](https://github.com/go-chi/httprate)
- **Database:** [PostgreSQL](https://www.postgresql.org/)
- **SQL Code Generation:** [sqlc](https://sqlc.dev/)
- **Database Migrations:** [goose](https://github.com/pressly/goose)
- **Password Hashing:** `golang.org/x/crypto/argon2`
- **OpenAPI/Swagger UI:** [OpenAPI 3.0](https://www.openapis.org/) / [Swagger UI](https://swagger.io/tools/swagger-ui/)
- **Docker:** [Docker](https://www.docker.com/)
- **Hosting:** [Render](https://render.com/) (compute) + [Neon](https://neon.tech/) (Postgres)

---

## Project Structure

```text
feedsanctum/
├── cmd/
│   └── server/         # entrypoint
├── internal/
│   ├── auth/           # password hashing and token creation
│   ├── database/       # sqlc-generated queries
│   ├── docs/           # OpenAPI spec embed
│   ├── handlers/       # HTTP handlers and middleware
│   ├── integration/    # integration tests
│   ├── scraper/        # background feed scraper
│   └── testingutils/   # shared test suite
├── sql/
│   ├── migrations/     # goose migrations
│   └── queries/        # sqlc source SQL
├── docker-compose.yml
└── README.md
```

## API Endpoints Summary

The full spec is defined in [`internal/docs/openapi.yaml`](./internal/docs/openapi.yaml) and served interactively via Swagger UI.

### Public Routes
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/v1/register` | Register a new user account |
| `POST` | `/api/v1/login` | Authenticate user and receive access/refresh tokens |
| `POST` | `/api/v1/refresh` | Rotate refresh token and issue new access token |
| `POST` | `/api/v1/logout` | Revoke refresh token |
| `GET` | `/api/v1/feeds` | List all registered RSS feeds |
| `GET` | `/api/v1/feeds/{feedID}` | Retrieve a specific RSS feed by ID |

### Protected Routes
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/v1/me` | Retrieve current user profile |
| `PATCH` | `/api/v1/me` | Update user profile details |
| `DELETE` | `/api/v1/me` | Delete current user account |
| `POST` | `/api/v1/feeds` | Create a new RSS feed and automatically follow it |
| `GET` | `/api/v1/follows` | Get all feed follows for current user |
| `POST` | `/api/v1/follows` | Follow a feed |
| `DELETE` | `/api/v1/follows/{feedID}` | Unfollow a feed by ID |
| `GET` | `/api/v1/posts` | Fetch RSS posts for followed feeds |
| `POST` | `/api/v1/posts/{postID}/read` | Mark a specific post as read |
| `GET` | `/api/v1/posts/read` | Retrieve all posts marked as read by the user |

---

## 📄 License

This project is open-source and available under the [MIT License](LICENSE).

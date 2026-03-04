# HAB — Online Judge Backend

A LeetCode-style backend service built with **Go**, featuring an asynchronous code execution engine that evaluates user submissions inside isolated Docker containers.

## Tech Stack

- **Go** (Gin) — HTTP server & business logic
- **MySQL** — Persistent storage (users, problems, submissions, test cases)
- **Redis** — Message queue (Streams) + data cache
- **Docker** — Sandboxed code execution

## System Architecture

```
  ┌──────────┐         ┌──────────────────────────────────────────────────┐
  │  Client  │  HTTP   │              Go Server (Gin)                     │
  │ (React)  ├────────►│                                                  │
  └──────────┘         │   Handles REST API, auth, submission intake,     │
                       │   and hosts the worker pool in-process           │
                       └──────┬──────────────┬───────────────┬────────────┘
                              │              │               │
                         read/write     publish/consume   spin up per
                              │         submission IDs    submission
                              │              │               │
                       ┌──────▼──────┐ ┌─────▼──────┐ ┌─────▼───────────┐
                       │    MySQL    │ │   Redis    │ │     Docker      │
                       │             │ │            │ │                 │
                       │ • users     │ │ • Stream   │ │ ┌─────────────┐ │
                       │ • problems  │ │   (queue)  │ │ │python-runner│ │
                       │ • test_cases│ │ • Cache    │ │ │  (3.12-slim)│ │
                       │ • submiss-  │ │   (1h TTL) │ │ └─────────────┘ │
                       │   ions      │ │            │ │ ┌─────────────┐ │
                       │ • system_   │ │            │ │ │  go-runner  │ │
                       │   code      │ │            │ │ │(1.21-alpine)│ │
                       └─────────────┘ └────────────┘ │ └─────────────┘ │
                                                      └─────────────────┘
```

Redis serves a dual role: as a **message queue** (Streams with consumer groups) to distribute submission jobs to workers, and as a **cache layer** (1h TTL) for test cases and system code to reduce database load.

## How the Execution Engine Works

The execution engine uses a **producer–consumer** pattern built on **Redis Streams** with a consumer group, enabling horizontal scaling of workers.

### Submission Lifecycle

**1. Request Intake (synchronous)**

When a new submission request comes in, the server extracts the request data (source code, language, problem ID). It immediately inserts a new submission record into MySQL with `status = PROCESSING` and returns `202 Accepted` to the client right away — the connection is not held open.

**2. Queue**

The handler publishes the `submission_id` to the `code_submissions` Redis Stream. This is a lightweight reference — no code or test data is included in the message.

**3. Worker Picks Up the Job (asynchronous)**

An idle worker from the pool reads the message via `XREADGROUP` (consumer group: `judgers`). The worker then fetches everything it needs:

| Data | Source | Cached? |
|------|--------|---------|
| Submission record (code, language, problem ID) | MySQL | No |
| Test cases (input + expected output) | Redis → MySQL fallback | Yes (1h TTL) |
| System code (driver/harness code) | Redis → MySQL fallback | Yes (1h TTL) |
| Language imports | Redis → MySQL fallback | Yes (1h TTL) |

**4. Code Execution (Docker sandbox)**

The worker sends the job to the `CodeRunnerService`, which:

1. **Combines** imports + user code + system code into a single source file
2. **Starts** an isolated Docker container (`python:3.12-slim` or `golang:1.21-alpine`) with the code file mounted
3. **Compiles** (Go only) — if compilation fails, returns `COMPILATION_ERROR` immediately
4. **Runs** each test case sequentially. For each test case, the input is piped via `stdin` and `stdout` is compared against the expected output
5. **Fails fast** — execution stops at the first failure (wrong answer or runtime error)

**5. Result Persistence**

After execution, the worker updates the submission record in MySQL:

| Status | Meaning | Extra Data |
|--------|---------|------------|
| `ACCEPTED` | All test cases passed | — |
| `WRONG_ANSWER` | Output mismatch on a test case | Failed test case input + expected vs actual output |
| `COMPILATION_ERROR` | Build failure or runtime error | Error message / stderr |

**6. Polling**

The client polls `GET /submissions/:id` until the status is no longer `PROCESSING`.

### Why This Design?

- **Non-blocking** — The API returns instantly. Users don't wait for code to compile and run.
- **Scalable** — Redis Streams consumer groups distribute work across N workers. Adding capacity = increasing `NUM_OF_WORKERS`.
- **Safe** — Each submission runs in a disposable Docker container. Malicious code can't affect the host or other submissions.
- **Efficient** — Test cases and system code are cached in Redis, so repeated submissions for the same problem don't re-query the database.

## Project Structure

```
├── cmd/HAB/              # Application entrypoint
├── configs/              # Environment-based configuration
├── internal/
│   ├── handlers/         # HTTP handlers (auth, problems, submissions)
│   ├── services/         # Business logic (code runner, JWT, cache)
│   ├── repositories/     # Data access layer (MySQL queries + caching)
│   ├── models/           # Domain types (Submission, Problem, User)
│   ├── middlewares/      # Auth middleware (JWT cookie validation)
│   ├── workerpool/       # Redis Stream consumer pool
│   ├── docker/           # Dockerfiles for runner images
│   │   ├── go/           #   golang:1.21-alpine
│   │   └── python/       #   python:3.12-slim
│   ├── dbs/              # Database & Redis initialization
│   ├── logger/           # Structured logging (zap)
│   └── utils/            # Shared utilities
```

## API Overview

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/auth/register` | No | Create account |
| POST | `/auth/login` | No | Login (sets JWT cookies) |
| POST | `/auth/logout` | No | Clear cookies |
| GET | `/auth/verify` | No | Check auth status |
| GET | `/problems` | Optional | List all problems |
| GET | `/problems/:id` | Optional | Problem details + starter code |
| POST | `/submissions` | Required | Submit code (returns 202) |
| GET | `/submissions/:id` | Required | Get submission result |
| GET | `/submissions?problem_id=X` | Required | User's submission history |
| GET | `/health` | No | Health check |

### Submission Statuses

| Status | Description |
|--------|-------------|
| `PROCESSING` | Queued or currently being evaluated |
| `ACCEPTED` | All test cases passed |
| `WRONG_ANSWER` | Output didn't match expected result |
| `COMPILATION_ERROR` | Build failure or runtime error |

### Supported Languages

| ID | Language | Runner Image |
|----|----------|-------------|
| 1 | Python | `python:3.12-slim` |
| 2 | Go | `golang:1.21-alpine` |

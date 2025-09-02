---
trigger: glob
description: 
globs: .go
---

Go Backend Development Guidelines (v2)

General Responsibilities
- Develop idiomatic, maintainable, and high-performance Go code.
- Enforce modular design and separation of concerns via Clean Architecture.
- Promote TDD, robust observability, and scalable microservice patterns.
- Ensure security, performance, and operational excellence across services.

Architecture Patterns
- Apply Clean Architecture:
  - Handlers/Controllers → request/response handling
  - Services/Use Cases → business logic
  - Repositories/Data Access → persistence
  - Domain Models → core entities
- Use Domain-Driven Design (DDD) principles where applicable.
- Design interface-driven APIs with explicit dependency injection.
- Prefer composition over inheritance; keep interfaces small and focused.
- All public functions should work with interfaces, not concrete types.

Project Structure
cmd/         → application entry points
internal/    → core service logic (private to the module)
pkg/         → reusable packages across projects
api/         → gRPC/REST definitions and handlers (with versioning: api/v1)
configs/     → configuration files and loaders
build/       → Dockerfiles, Helm charts, deployment scripts
test/        → test helpers, mocks, integration tests
- Group by feature when it improves cohesion.
- Keep business logic decoupled from frameworks.
- Clearly distinguish internal/ (non-exported) vs pkg/ (exported).

Development Best Practices
- Keep functions short, single-responsibility.
- Always check and wrap errors with context: fmt.Errorf("serviceName: %w", err)
- Use errors.Is and errors.As for error classification.
- Avoid global state; use constructors for dependency injection.
- Use context.Context for deadlines, cancellation, and scoped values.
- Manage goroutines safely; protect shared state with channels or sync primitives.
- defer resource closures and handle them carefully.

Security & Resilience
- Validate and sanitize all external inputs.
- Use secure defaults for JWT, cookies, and configs.
- Protect secrets with Vault, AWS Secrets Manager, or SOPS.
- Enable TLS for gRPC and HTTP.
- Implement retries with exponential backoff, timeouts, and circuit breakers.
- Apply distributed rate-limiting (e.g., Redis) for abuse prevention.

Testing
- Write table-driven unit tests; run in parallel when possible.
- Mock external dependencies with mockgen or testify/mock.
- Separate fast unit tests from integration/E2E tests.
- Use Docker Compose for integration test environments (DB, queues, etc.).
- Ensure coverage for all exported functions (go test -cover).

Documentation & Standards
- Document public APIs with GoDoc comments.
- Maintain concise README.md for each service.
- Provide CONTRIBUTING.md and ARCHITECTURE.md.
- Enforce formatting with go fmt, goimports, and golangci-lint (minimum linters: govet, errcheck, staticcheck, gosimple, ineffassign).

Observability
- Use OpenTelemetry for tracing, metrics, logging.
- Start and propagate spans across all service boundaries.
- Always attach context.Context to spans, logs, and metrics.
- Record attributes like request params, user ID, error details.
- Correlate logs with trace IDs.
- Define metrics SLIs (e.g., latency < 300ms) and export to Prometheus/Grafana.
- Set sampling rates and retention policies for telemetry data.

Performance
- Use benchmarks to track regressions.
- Profile using pprof and visualize with flame graphs.
- Avoid premature optimization; measure before tuning.
- Minimize allocations in hot paths.

Concurrency
- Use goroutines safely; guard shared state with channels or sync.
- Cancel goroutines with context to avoid leaks.

Tooling & Dependencies
- Prefer the standard library; keep third-party dependencies minimal.
- Use Go modules; lock versions for reproducibility.
- Run linting, testing, and security checks in CI.

Key Conventions
1. Readability, simplicity, maintainability first.
2. Isolate business logic to allow framework changes.
3. Apply dependency inversion and clear boundaries.
4. Make all behavior observable, testable, documented.
5. Automate builds, tests, deployments.

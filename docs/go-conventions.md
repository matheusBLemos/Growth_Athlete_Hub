# Go Conventions — GAH Server

## Naming

- **Files**: `snake_case.go` — one type per file when the type is non-trivial (e.g., `activity.go`, `hrv_rule.go`).
- **Packages**: short, lowercase, single word when possible (`entity`, `port`, `usecase`).
- **Interfaces**: noun or adjective describing capability — `Repository`, `Evaluator`, `Publisher`. No `I` prefix.
- **Structs implementing interfaces**: concrete name — `PostgresActivityRepository`, `DeterministicEvaluator`.
- **Constructors**: `New<Type>` — `NewActivity(...)`, `NewHRVRule(...)`.
- **Errors**: `Err<Description>` as package-level sentinels — `ErrInvalidEmail`, `ErrMetricOutOfRange`.

## Formatting

All code must pass `gofmt` with no diff. CI enforces this automatically.

- No manual alignment of struct fields or comments.
- Use `goimports` for import grouping: stdlib, then external, then internal.

## Architecture — Clean Architecture (inside-out)

```
server/internal/
├── domain/           # No external dependencies
│   ├── entity/       # Core business objects + validation
│   └── valueobject/  # Immutable typed values (Severity, MetricType, etc.)
├── application/
│   ├── port/         # Interfaces consumed by use cases
│   └── usecase/      # Application logic, orchestrates domain + ports
└── infra/            # Concrete implementations
    ├── persistence/  # Repository implementations (PostgreSQL, TimescaleDB)
    ├── http/         # Controllers, routes, middleware
    └── insights/
        └── deterministic/  # Insight rules as Strategy pattern
```

**Dependency rule**: dependencies point inward. Domain imports nothing from application or infra. Application imports domain but not infra. Infra imports both.

## Dependency Injection

Use compile-time interface verification:

```go
var _ port.ActivityRepository = (*PostgresActivityRepository)(nil)
```

Place this declaration at the top of the implementing file, right after the type definition.

## Testing

- TDD: write failing tests before production code.
- Test files live alongside source: `activity.go` → `activity_test.go`.
- Use table-driven tests for validations and rule evaluation.
- Use `testify` only if needed for complex assertions; prefer stdlib `testing`.
- Name test functions: `Test<Type>_<Method>_<Scenario>` — `TestActivity_Validate_NegativeDuration`.

## Error Handling

- Domain entities return typed sentinel errors for validation failures.
- Use `errors.Is` / `errors.As` for error checking; avoid string comparison.
- Wrap errors with `fmt.Errorf("context: %w", err)` when adding context.

## Comments

- Default: no comments. Code should be self-documenting through naming.
- Only comment when the WHY is non-obvious: hidden constraints, workarounds, surprising behavior.
- No comments explaining WHAT the code does.

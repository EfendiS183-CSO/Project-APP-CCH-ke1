# CCH Enterprise Platform

This workspace contains three independently runnable applications:

- `CCH-BE`: Go API and owner of CCH business rules and MySQL access.
- `CCH-FE`: React web client. It calls the Go API, never the AI engine directly.
- `CCH-AI-ENGINE`: Python FastAPI inference boundary. It does not connect to or write CCH business tables.

## Architecture constraints

- MySQL is retained. PostgreSQL migration is excluded from the current scope.
- The applications are separate processes with separate entrypoints and validation.
- The legacy PHP application remains the current business system until an explicit migration plan is approved. New Go code must not write legacy tables directly without a reviewed ownership and consistency contract.
- AI output is an untrusted suggestion. The Go backend validates it and owns authorization, persistence, and business decisions.
- OOP is applied idiomatically: Go structs, methods, interfaces, composition, and package boundaries; React components by responsibility; Python classes where they represent useful contracts or behavior.
- Provider credentials, customer data, and production database credentials do not belong in source control or frontend configuration.

## Local development

Run each application from its own directory. See the corresponding README for commands.

The initial scope is service bootstrapping and health contracts only. Ticket workflows, authentication, integrations, queues, caching, orchestration, and provider selection require their own requirements and acceptance tests before implementation.
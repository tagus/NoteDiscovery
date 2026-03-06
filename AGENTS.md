# Repository Guidelines

## Project Structure & Module Organization
`NoteDiscovery` is a Go application with a static frontend.
- `cmd/notediscovery/`: main entrypoint and runtime wiring.
- `internal/`: core application packages (`notes`, `server`, `plugins`, `graph`, `auth`, `config`, `themes`, `locales`, `share`, `utils`).
- `frontend/`: browser assets (`index.html`, `app.js`, service worker, client resources).
- `themes/`, `locales/`, `plugins/`: configurable runtime assets and extension points.
- `data/`: local markdown note storage (runtime content, not core source).
- `documentation/` and `docs/`: user documentation and website/public assets.

## Build, Test, and Development Commands
- `make run` : run locally on `:8000` with `config.yaml`.
- `make run CONFIG=config.yaml PORT=9000` : override config path or port.
- `make build` : compile all Go packages (`go build ./...`).
- `make test` : run unit tests across the repository (`go test ./...`).
- `make audit-regex` : check Go regex usage for RE2-incompatible patterns.
- `docker-compose up -d` : run locally in Docker.

## Coding Style & Naming Conventions
- Format Go code with `gofmt` before committing.
- Follow idiomatic Go naming: exported `PascalCase`, unexported `camelCase`, short lowercase package names.
- Keep functions focused and readable; avoid unnecessary defensive validation of every field.
- Prefer existing Mango utility methods when they already solve the problem.
- Frontend changes should stay consistent with current plain JS/HTML/CSS structure in `frontend/`.

## Testing Guidelines
- Place tests next to implementation files using `*_test.go`.
- Use table-driven tests for behavior variants.
- Use `github.com/stretchr/testify/require` for assertions.
- Prefer `expected` and `actual` variable names in tests for clarity.
- Run `make test` before opening a PR; add regression tests for bug fixes.

## Commit & Pull Request Guidelines
- Use concise, imperative commit subjects; current history favors prefixes like `fix:`, `refactor:`, `test:`, and `chore:`.
- Keep commits scoped to a single logical change.
- For major features or architecture changes, open an issue for discussion before implementation.
- PRs should include: clear summary, linked issue(s), testing notes, and screenshots for UI updates.
- Update docs when behavior, configuration, or plugin/theme interfaces change.

## Security & Configuration Tips
- Treat `config.yaml` as the primary local configuration surface.
- Do not commit secrets or real credentials.
- If exposing beyond localhost/private LAN, enable authentication and place the app behind HTTPS/reverse proxy.

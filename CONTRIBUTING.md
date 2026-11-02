# Contributing to Observatory

Observatory is a local-first Docker monitoring and investigation tool for individual developers and small teams. Contributions use the MIT license in LICENSE. Do not submit material you do not have permission to contribute; third-party dependencies retain their licenses.

Discuss changes to architecture, monitoring semantics, retention or Docker permissions before implementation. Prefer focused changes with clear user-facing behavior. Report bugs with operating system, Docker/Compose versions, image version, reproduction steps and redacted errors. Do not include private logs, credentials or environment dumps.

## Development

Use Docker Desktop Linux mode or a Linux Engine. `docker compose up --build -d` builds the app and starts its development database. The developer Compose project is `go-observability`; the downloadable user bundle uses `observatory` and a different volume. Never use production/user data for integration tests.

Go version is in go.mod; frontend builds use Node 24. For UI development, run `npm ci` then `npm run dev` inside frontend while the Go API is running. Before checks, run `npm run build` there to refresh embedded assets. Then `npm test`, `go test ./...` and `go vet ./...`. See README.md for local database configuration. Set TEST_DATABASE_URL to a dedicated database to run the nine isolated PostgreSQL integration tests; they create/drop only temporary schemas, but the role needs permission to do that.

Packaging uses Python 3.10+ standard library. Run `python -m unittest discover -s scripts -p 'test_*.py'`. After changing dependencies, run `python scripts/collect_licenses.py` with dependencies installed and review THIRD_PARTY_NOTICES.md/LICENSES. Do not commit .env files, dumps, work/, outputs/, dist/ or node_modules/.

The CI workflow checks Go/frontend/database behavior on native Linux amd64 and arm64 runners and smoke-tests a disposable release installation. Both native Linux jobs passed on the initial main-branch upload; each numbered release reruns them for its tagged source. Maintainer publication steps are in docs/releases.md.

# Install Observatory

This bundle runs one local Observatory instance for one Docker Engine. You do not need Go, Node, Git, psql, or a separate PostgreSQL installation.

## Before starting

- Start Docker Engine with Compose v2, or Docker Desktop in **Linux containers** mode. Allow Docker enough resources for PostgreSQL and your own workloads.
- Images target Linux amd64 (Intel/AMD) and arm64. Windows Docker Desktop amd64 is locally verified. Native Linux, macOS/Apple Silicon and ARM require their own release verification; an available image architecture alone does not prove full platform compatibility.
- The dashboard binds to your computer's localhost. This release has no authentication and is intended for local development.
- Observatory needs access to the Docker socket to discover containers and read measurements/logs across that Engine. A `:ro` socket mount does **not** restrict Docker API permissions; treat Observatory as trusted software with access to your Engine. Docker Desktop Enhanced Container Isolation or rootless configurations may restrict this mount.

## First start

1. Extract this ZIP into a folder you intend to keep. Open a terminal in the extracted folder containing compose.yaml.
2. Copy `.env.example` to `.env` (show hidden files if needed). On PowerShell: `Copy-Item .env.example .env`; on macOS/Linux: `cp .env.example .env`.
3. Edit `.env`: set `POSTGRES_PASSWORD` to a unique string of at least 32 letters/digits. Leave the image reference as supplied. Keep the file private. Use letters/digits because the password is embedded in a database connection URL.
4. Run:

```
docker compose pull
docker compose up -d
```

5. Open http://localhost:8080. Allow two successful collection cycles (normally around 20 seconds) for CPU/I/O baselines. PostgreSQL and schema creation happen automatically. `/api/health` reports readiness; Docker's “started” state alone is not proof that collection is ready.

If 8080 is already used, change `OBSERVATORY_PORT` in `.env`, restart with `docker compose up -d`, and use that port in your browser. Do not run the development Compose stack on the same port. The release Compose project is named `observatory`; the developer stack is `go-observability`. They intentionally use separate database volumes. Changing folders does not change the fixed project name; multiple release installations on the same Engine need separate `-p` names and ports on every command.

Your other applications can use their own Compose projects. Observatory discovers existing containers on the same Engine; it does not start or generate load for them. Resource/event history lasts seven days; Docker logs are fetched on demand and are not archived. Alerts start disabled per project.

## Stop, restart and inspect

```
docker compose stop
docker compose start
docker compose logs --tail 100 app
docker compose ps
```

`docker compose down` removes the Observatory containers/network while preserving its database volume. `docker compose up -d` starts it again. Do not change the password in `.env` after initialization without also changing the database role password; PostgreSQL initialization variables do not rotate an existing password.

## Back up

Stop the app for a quiet, consistent maintenance window; your application containers keep running. These commands avoid binary output redirection, which can damage dumps in older PowerShell versions:

```
docker compose stop app
docker compose exec -T db pg_dump -U observatory -d observatory -Fc -f /tmp/observatory.dump
docker compose cp db:/tmp/observatory.dump ./observatory.dump
docker compose start app
```

Keep the dump and a private copy of `.env` somewhere safe. Dumps include stored monitoring history, sessions, alerts and project settings; they do not contain an independent log archive. Copying a live PostgreSQL volume directory is not a substitute for a database backup.

## Upgrade

Back up first. Read the target release notes. Keep your current `.env` and the same Compose project name/volume. Update only `OBSERVATORY_IMAGE` to the exact reference supplied with the new release; use its Compose file if the notes require a change. Then:

```
docker compose pull
docker compose up -d
docker compose logs --tail 100 app
```

Migrations run on startup. Schema downgrades are not supported: switching the image tag back may not undo a migration. Recovery from a bad upgrade means restoring a pre-upgrade dump with the matching old image into a compatible PostgreSQL version. PostgreSQL major-version upgrades need a separate migration; do not change `postgres:18` to a different major just to update Observatory.

## Restore a backup

Restoring **replaces Observatory's stored history/settings in this Compose project's database**. Verify the project/backup first. Stop the app; start the db if needed, then:

```
docker compose stop app
docker compose up -d db
docker compose cp ./observatory.dump db:/tmp/observatory.dump
docker compose exec -T db pg_restore --exit-on-error --clean --if-exists --no-owner -U observatory -d observatory /tmp/observatory.dump
docker compose up -d app
```

Use a matching old app image when restoring a pre-upgrade backup. Wait for PostgreSQL readiness before the restore. Keep a separate backup of the current database before replacing it.

## Uninstall

`docker compose down` keeps data for later. To intentionally erase **all Observatory data for this Compose project**, use `docker compose down --volumes`. That deletion cannot be reversed without a backup. Remove the extracted folder yourself when finished. These commands do not remove your monitored application's containers.

## Troubleshooting

- Image unauthorized/not found: confirm the reference is from the intended release; its GHCR package must be public for anonymous download. A new/private package may need the maintainer to change visibility.
- Registry DNS/connection errors: confirm Docker can reach its registry (VPN, proxy and DNS settings). Rebuilding source still needs dependency/base-image downloads.
- Docker socket missing/permission denied: use the standard local Linux Engine or Docker Desktop Linux mode. For rootless Docker, set `DOCKER_SOCKET_PATH` to its actual Unix socket path and verify permissions. Remote Engines, Windows containers and Docker-in-Docker are outside the verified setup.
- Database password errors: retain the original `.env`; changing the variable does not change an existing database role.
- Collection unavailable: check `/api/health` and `docker compose logs --tail 100 app`; do not infer zero load from missing readings.

License: see LICENSE. Dependency notices: THIRD_PARTY_NOTICES.md and LICENSES/. Release identity: release.json. This bundle is a local installation, with no hosted account or telemetry service configured by Observatory.

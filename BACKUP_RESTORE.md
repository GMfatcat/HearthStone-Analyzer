# Backup and Restore Guide

## What Must Be Preserved

For the current deployment model, the critical persisted state is:

- `/data/hearthstone.db`

That SQLite file contains:

- settings
- encrypted LLM credentials and model settings
- cards
- richer persisted card metadata and functional tags
- synced meta snapshots
- persisted decks and deck cards
- AI reports
- scheduled jobs
- job execution history

If you preserve `/data/hearthstone.db`, you preserve the app's main working state.

## Recommended Backup Rule

Use a stop-the-app-first backup for the most conservative and simplest workflow.

Why:

- the scheduler and API write to SQLite in the same process
- stopping the container avoids copying the database mid-write
- this is the least surprising restore path for a single-machine deployment

## Backup Procedure

### Option A: Host Bind Mount

If your container uses a host bind mount for `/data`, back up the host directory after stopping the container.

Example:

```bash
docker stop hearthstone-analyzer

cp /absolute/host/path/hearthstone-data/hearthstone.db \
   /absolute/backup/path/hearthstone-$(date +%Y%m%d-%H%M%S).db
```

### Option B: Named Docker Volume

If your container uses a named volume, copy the database out through a temporary helper container after stopping the app container.

Example:

```bash
docker stop hearthstone-analyzer

mkdir -p ./backups

docker run --rm \
  -v hearthstone-data:/from \
  -v ${PWD}/backups:/to \
  debian:bookworm-slim \
  sh -c "cp /from/hearthstone.db /to/hearthstone-$(date +%Y%m%d-%H%M%S).db"
```

## Restore Procedure

### Restore Into a Host Bind Mount

1. stop and remove the running container
2. replace the current database file with the backup copy
3. start the container again with the same environment variables

Example:

```bash
docker stop hearthstone-analyzer
docker rm hearthstone-analyzer

cp /absolute/backup/path/hearthstone-20260327-030000.db \
   /absolute/host/path/hearthstone-data/hearthstone.db

docker run -d \
  --name hearthstone-analyzer \
  -p 8080:8080 \
  -e APP_SETTINGS_KEY=replace-with-32-byte-secret \
  -v /absolute/host/path/hearthstone-data:/data \
  hearthstone-analyzer:dev
```

### Restore Into a Named Volume

1. stop and remove the running container
2. copy the backup database into the volume
3. start the container again using the same volume

Example:

```bash
docker stop hearthstone-analyzer
docker rm hearthstone-analyzer

docker run --rm \
  -v hearthstone-data:/to \
  -v ${PWD}/backups:/from \
  debian:bookworm-slim \
  sh -c "cp /from/hearthstone-20260327-030000.db /to/hearthstone.db"

docker run -d \
  --name hearthstone-analyzer \
  -p 8080:8080 \
  -e APP_SETTINGS_KEY=replace-with-32-byte-secret \
  -v hearthstone-data:/data \
  hearthstone-analyzer:dev
```

## Important Restore Constraint

Use the same `APP_SETTINGS_KEY` that was used when the backup was created.

Why:

- sensitive settings such as `llm.api_key` are encrypted before being stored
- restoring the database with a different settings key may leave encrypted values unreadable

If the key changes:

- the app may still start
- encrypted settings may no longer decrypt correctly
- you may need to re-enter secret settings in the UI

## Post-Restore Verification

After restoring:

1. confirm `GET /healthz` returns `ok`
2. open the UI and verify settings load
3. confirm `llm.base_url` and `llm.model` still appear as expected
4. test one deck parse/analyze request
5. open report history and verify saved reports still load
6. inspect Jobs and confirm schedules/history are present
7. if meta had been synced before backup, confirm snapshot history is still visible

## Practical Recommendations

- keep at least one backup from before major upgrades or schema changes
- always take a backup before starting a newer image against an existing `/data` volume
- keep the backup filename timestamped
- store the matching `APP_SETTINGS_KEY` securely outside the container
- test one restore on a non-critical copy before relying on the process operationally

## Current Scope Limitation

This guide assumes:

- single-machine deployment
- SQLite as the only persistent datastore
- manual or operator-triggered backups

It does not yet define:

- automated scheduled backups
- offsite replication
- point-in-time recovery

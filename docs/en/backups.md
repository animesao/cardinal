<!-- cardinal-version:start -->
**Documentation version:** `2.0.10`
**Project release:** `v2.0.10`
<!-- cardinal-version:end -->

# cardinal Backups Guide

Complete guide to automatic and manual backups, restoration, downloading backups to your local machine, and safety guarantees.

> **Key fact:** cardinal backups archive the container's **writable overlay** and **named volumes**. They do **not** archive host bind mounts (e.g. `-v /data/app:/app`). If your important data is in a bind mount, archive it separately (see [Section 6](#6-bind-mount-workaround)).

---

## Table of contents

1. [What gets backed up](#1-what-gets-backed-up)
2. [Automatic backups](#2-automatic-backups)
3. [Manual backups](#3-manual-backups)
4. [Restoring from backups](#4-restoring-from-backups)
5. [Downloading backups to your local machine](#5-downloading-backups-to-your-local-machine)
6. [Bind-mount workaround](#6-bind-mount-workaround)
7. [Edge cases and safety guarantees](#7-edge-cases-and-safety-guarantees)
8. [Backup settings reference](#8-backup-settings-reference)
9. [Backup directory structure](#9-backup-directory-structure)
10. [Best practices](#10-best-practices)
11. [Troubleshooting](#11-troubleshooting)
12. [Quick reference](#12-quick-reference)

---

## 1. What gets backed up

| Included in archive | Not included |
|---------------------|-------------|
| Named volumes (`-v mydata:/path`) | Host bind mounts (`-v /data/app:/app`) |
| Container writable overlay (files created inside the container) | Image layers (read-only base) |
| Container metadata (ports, env, restart policy, etc.) | Container runtime state (PID, cgroup) |
| SHA-256 checksum sidecar (`.sha256`) | — |

**Why bind mounts are excluded:** bind mounts point to existing host directories that are managed outside cardinal's container lifecycle. Archiving them could silently capture stale or partially-written data. For bind-mounted data, use the host-level backup approach in [Section 6](#6-bind-mount-workaround).

**What this means in practice:**

| Use case | Data location | Backed up by cardinal? |
|----------|---------------|-------------------|
| PostgreSQL database | Named volume `postgres-data:/var/lib/postgresql/data` | ✅ Yes |
| Redis with AOF | Named volume `redis-data:/data` | ✅ Yes |
| Bot config + code | Bind mount `/bot:/bot` | ❌ No — use cron backup |
| Minecraft world | Bind mount `/data/minecraft:/data` | ❌ No — use cron backup |
| Files created inside container | Overlay (e.g. `/tmp`, `/var/log`) | ✅ Yes |

---

## 2. Automatic backups

### 2.1 Prerequisites

The backup scheduler runs inside the `cardinal-bootstrap.service` systemd unit. Make sure it is installed and running:

```bash
cardinal bootstrap --install
systemctl status cardinal-bootstrap
```

### 2.2 Enable automatic backups

```bash
cardinal backup enable <container> [OPTIONS]
```

**Options:**

| Option | Default | Description |
|--------|---------|-------------|
| `--interval DURATION` | `24h` | How often to back up. Accepts Go durations: `6h`, `30m`, `1h30m`, `7d` |
| `--retention N` | `7` | Number of archives to keep (range: 1–1000). Older archives are pruned automatically |
| `--dir PATH` | `~/.cardinal/backups/<container>` | Custom backup directory. Protected host paths and symlinks are rejected |

**Examples:**

```bash
# Database: backup every 6 hours, keep 14 copies
cardinal backup enable postgres --interval 6h --retention 14

# Bot: daily backup, keep 7 copies
cardinal backup enable bot --interval 24h --retention 7

# Minecraft writable layer: backup every 12 hours, keep 30 copies, custom path
cardinal backup enable minecraft --interval 12h --retention 30 --dir /data/backups/minecraft

# Minimal: default settings (every 24h, keep 7)
cardinal backup enable webapp
```

### 2.3 How automatic backups work

The supervisor follows this sequence:

1. **Scan:** On each tick (every few minutes), the supervisor checks each container with `auto_backup = true`.
2. **Time check:** If the interval has not elapsed, the container is skipped.
3. **State check:** The latest container state is re-read. If the container is **not running**, backup is silently skipped (see [Edge cases](#7-edge-cases-and-safety-guarantees)).
4. **Lock:** An exclusive file lock is acquired on the backup directory to prevent concurrent backups.
5. **Stop:** The container is gracefully stopped.
6. **Archive:** The writable overlay and named volumes are packed into a compressed `.tar.gz` archive. A SHA-256 checksum sidecar (`.sha256`) is written alongside.
7. **Prune:** Archives beyond the retention count are deleted automatically.
8. **Restart:** The container is started again with its original restart policy intact.
9. **Record:** `last_backup_at` is updated in the container state.

**Downtime during backup:** The container is stopped for the duration of the archive creation. For small containers (a few hundred MB), this typically takes 1–5 seconds. For large databases (multi-GB), it may take 30–60 seconds. Schedule backups during off-peak hours to minimize user impact.

### 2.4 Check backup status

```bash
cardinal backup status <container>
```

Example output:

```
Container: postgres
  Enabled: true
  Interval: 6h0m0s
  Retention: 14
  Directory: /root/.cardinal/backups/postgres
  Last successful backup: 2026-08-12T04:00:00+03:00
  Next retry after: 0001-01-01T00:00:00Z
```

If `Last successful backup` shows `never`, the first backup has not completed yet. The first archive is created after the first interval elapses.

### 2.5 List all backups

```bash
cardinal backup list                           # list default backup directory
cardinal backup list /data/custom-backups      # list a custom directory
```

Example output:

```
postgres-20260812-040000.tar.gz      2048576 bytes  2026-08-12T04:00:00+03:00
postgres-20260811-220000.tar.gz      1998344 bytes  2026-08-11T22:00:00+03:00
postgres-20260811-160000.tar.gz      1945600 bytes  2026-08-11T16:00:00+03:00
```

### 2.6 Verify a backup archive

```bash
cardinal backup verify /path/to/backup.tar.gz
```

Expected output:

```
Backup verified: /path/to/backup.tar.gz
```

If no `.sha256` sidecar exists (older backups created before checksum support was added), cardinal reports:

```
Backup is valid but unverified (no checksum sidecar): /path/to/backup.tar.gz
```

### 2.7 Disable automatic backups

```bash
cardinal backup disable <container>
```

This stops future scheduled backups. Existing archives are not deleted. The `auto_backup` flag in the container state is set to `false`.

---

## 3. Manual backups

### 3.1 Create a one-shot backup

The container **must be stopped** before creating a manual backup. This ensures data consistency.

```bash
cardinal stop <container>
cardinal backup create <container>                              # auto-generated filename
cardinal backup create <container> -o /path/to/file.tar.gz     # custom output path
cardinal start <container>
```

**Example — backup before a risky operation:**

```bash
# Stop, backup, then perform the operation
cardinal stop postgres
cardinal backup create postgres -o /data/backups/postgres-pre-upgrade.tar.gz
cardinal start postgres

# ... perform upgrade ...
```

**Example — backup with default naming:**

```bash
cardinal stop webapp
cardinal backup create webapp
# Creates: ~/.cardinal/backups/webapp/webapp-20260812-150000.tar.gz
cardinal start webapp
```

### 3.2 Verify a manual backup

Always verify after creating:

```bash
cardinal backup verify /data/backups/postgres-pre-upgrade.tar.gz
```

### 3.3 Backup output path validation

cardinal rejects backup output paths that are:

- Protected host directories (`/root`, `/etc`, `/var`, `/usr`, `/opt`, `/run`, `/bin`, `/sbin`, `/lib`, `/lib64`, `/boot`, `/dev`, `/proc`, `/sys`, `/home`, `/media`, `/mnt`, `/srv`)
- Inside the cardinal data directory (prevents recursive archiving)
- Containing symlink components (prevents path traversal attacks)

If you need backups in a non-standard location, create a dedicated directory under `/data`:

```bash
mkdir -p /data/backups
cardinal backup create webapp -o /data/backups/webapp-manual.tar.gz
```

---

## 4. Restoring from a backup

### 4.1 Basic restore procedure

The target container **must exist and be stopped** before restoring.

```bash
# Step 1: Verify the backup integrity
cardinal backup verify /path/to/backup.tar.gz

# Step 2: Stop the container
cardinal stop <container>

# Step 3: Restore
cardinal backup restore <container> /path/to/backup.tar.gz

# Step 4: Start the container
cardinal start <container>
```

**Full example:**

```bash
cardinal backup verify /root/.cardinal/backups/postgres/postgres-20260812-040000.tar.gz
cardinal stop postgres
cardinal backup restore postgres /root/.cardinal/backups/postgres/postgres-20260812-040000.tar.gz
cardinal start postgres
```

### 4.2 What gets restored

- Container's writable overlay (all files created or modified during runtime)
- Named volume data
- Container metadata: image, ports, environment variables, restart policy, hostname, labels, volume mounts

**Not restored:** bind mounts. If your service uses bind mounts, you must restore that data separately (e.g., from a host-level tar backup).

### 4.3 Restore into a new container

If the original container was deleted, recreate it first with the same name and matching configuration:

```bash
# Recreate the container with the same settings
cardinal run -d -n postgres \
  --restart unless-stopped \
  -p 5432:5432 \
  -v postgres-data:/var/lib/postgresql/data \
  -e POSTGRES_DB=app \
  -e POSTGRES_USER=app \
  -e POSTGRES_PASSWORD='change-me' \
  postgres:16

# Stop, restore, start
cardinal stop postgres
cardinal backup restore postgres /root/.cardinal/backups/postgres/postgres-20260812-040000.tar.gz
cardinal start postgres
```

### 4.4 Restore safety checks

During restore, cardinal performs the following safety checks:

1. **Checksum verification:** The archive is verified against its `.sha256` sidecar before extraction begins. Mismatched checksums abort the restore.
2. **ID matching:** The archive's container ID must match the target container. You cannot restore a backup from container A into container B.
3. **Symlink traversal protection:** Archive entries are checked for path traversal (e.g., `../../etc/passwd`). Suspicious entries are rejected.
4. **Atomic file writes:** Restored files are written to temporary files first, then renamed into place. A crash during restore cannot leave a half-written file.

---

## 5. Downloading backups to your local machine

### 5.1 Using scp (simple)

From your **local machine** (not the server):

```bash
# Download a specific backup
scp root@<SERVER_IP>:/root/.cardinal/backups/postgres/postgres-20260812-040000.tar.gz ./

# Download the checksum too
scp root@<SERVER_IP>:/root/.cardinal/backups/postgres/postgres-20260812-040000.tar.gz.sha256 ./
```

### 5.2 Using rsync (faster, resumable)

From your **local machine**:

```bash
# Sync an entire backup directory
rsync -avz --progress root@<SERVER_IP>:/root/.cardinal/backups/postgres/ ./local-backups/postgres/

# Resume an interrupted download
rsync -avz --progress --partial root@<SERVER_IP>:/root/.cardinal/backups/postgres/ ./local-backups/postgres/
```

### 5.3 Download the latest backup only

From your **local machine**:

```bash
# Find the latest backup on the server and download it
LATEST=$(ssh root@<SERVER_IP> "ls -1t /root/.cardinal/backups/postgres/*.tar.gz | head -1")
scp root@<SERVER_IP>:"$LATEST" ./

# Also grab its checksum
scp root@<SERVER_IP>:"${LATEST}.sha256" ./
```

### 5.4 Encrypt before download (recommended for sensitive data)

**On the server:**

```bash
# Symmetric encryption with AES-256
gpg -c --symmetric --cipher-algo AES256 /root/.cardinal/backups/postgres/postgres-20260812-040000.tar.gz
# You will be prompted for a passphrase. Output: postgres-20260812-040000.tar.gz.gpg
```

**From your local machine:**

```bash
scp root@<SERVER_IP>:/root/.cardinal/backups/postgres/postgres-20260812-040000.tar.gz.gpg ./
```

**To decrypt locally:**

```bash
gpg -d postgres-20260812-040000.tar.gz.gpg > postgres-20260812-040000.tar.gz
```

### 5.5 Verify a downloaded backup

**Option A — with cardinal installed locally:**

```bash
cardinal backup verify ./postgres-20260812-040000.tar.gz
```

**Option B — manual checksum verification:**

```bash
# Compute the SHA-256 of the downloaded file
sha256sum postgres-20260812-040000.tar.gz
# Compare with the sidecar
cat postgres-20260812-040000.tar.gz.sha256
# The hash values must match
```

### 5.6 Download all backups at once

From your **local machine**:

```bash
# Sync the entire backup tree for all containers
rsync -avz --progress root@<SERVER_IP>:/root/.cardinal/backups/ ./cardinal-backups/
```

### 5.7 Restore a downloaded backup to the server

If you downloaded a backup and need to put it back:

```bash
# Upload from local machine
scp ./postgres-20260812-040000.tar.gz root@<SERVER_IP>:/tmp/

# On the server
cardinal stop postgres
cardinal backup restore postgres /tmp/postgres-20260812-040000.tar.gz
cardinal start postgres
rm /tmp/postgres-20260812-040000.tar.gz
```

---

## 6. Bind-mount workaround

`cardinal backup` does **not** archive bind mounts. For data that lives in a bind mount (Minecraft worlds, bot code, configuration files), use a cron job on the host.

### 6.1 Backup script for bind-mounted data

```bash
#!/bin/bash
# /usr/local/bin/bind-backup.sh
# Backup bind-mounted data with consistency and rotation
set -euo pipefail

SRC="$1"           # Source directory (bind mount path)
DEST="$2"          # Backup destination
CONTAINER="$3"     # Container name (for optional stop/start)
RETENTION="${4:-5}" # Number of copies to keep

STAMP=$(date +%Y%m%d-%H%M%S)
BASENAME=$(basename "$SRC")

# Create destination
mkdir -p "$DEST"

# Optional: stop container for perfect consistency
# Uncomment the next two lines if you need crash-consistent backups:
# cardinal stop "$CONTAINER" 2>/dev/null || true
# STOPPED=true

# Create archive
tar czf "$DEST/${BASENAME}-${STAMP}.tar.gz" -C "$(dirname "$SRC")" "$BASENAME"

# Create checksum
sha256sum "$DEST/${BASENAME}-${STAMP}.tar.gz" > "$DEST/${BASENAME}-${STAMP}.tar.gz.sha256"

# Optional: restart container
# if [ "${STOPPED:-}" = "true" ]; then
#   cardinal start "$CONTAINER"
# fi

# Rotate old backups
ls -1t "$DEST"/${BASENAME}-*.tar.gz 2>/dev/null | tail -n +$((RETENTION + 1)) | xargs -r rm -f
ls -1t "$DEST"/${BASENAME}-*.tar.gz.sha256 2>/dev/null | tail -n +$((RETENTION + 1)) | xargs -r rm -f

echo "[$(date)] Backup done: $DEST/${BASENAME}-${STAMP}.tar.gz"
```

### 6.2 Schedule with cron

```bash
sudo chmod +x /usr/local/bin/bind-backup.sh

# Edit crontab
sudo crontab -e

# Add: backup /data/minecraft every day at 04:00, keep 5 copies
0 4 * * * /usr/local/bin/bind-backup.sh /data/minecraft /data/mc-backups minecraft 5 >> /var/log/mc-backup.log 2>&1

# Add: backup /bot every 12 hours, keep 7 copies
0 */12 * * * /usr/local/bin/bind-backup.sh /bot /data/bot-backups bot 7 >> /var/log/bot-backup.log 2>&1
```

### 6.3 Minecraft-specific backup (with server stop)

For Minecraft worlds, stopping the server during backup ensures region files are not mid-write:

```bash
#!/bin/bash
# /usr/local/bin/mc-backup.sh
set -euo pipefail

SRC="/data/minecraft"
DEST="/data/mc-backups"
RETENTION=5

mkdir -p "$DEST"
STAMP=$(date +%Y%m%d-%H%M%S)

# Save world and stop server
echo "save-off" | cardinal attach minecraft 2>/dev/null || true
echo "save-all" | cardinal attach minecraft 2>/dev/null || true
sleep 2
cardinal stop minecraft

# Create archive
tar czf "$DEST/mc-${STAMP}.tar.gz" -C "$SRC" .
sha256sum "$DEST/mc-${STAMP}.tar.gz" > "$DEST/mc-${STAMP}.tar.gz.sha256"

# Restart server
cardinal start minecraft

# Rotate
ls -1t "$DEST"/mc-*.tar.gz 2>/dev/null | tail -n +$((RETENTION + 1)) | xargs -r rm -f
ls -1t "$DEST"/mc-*.tar.gz.sha256 2>/dev/null | tail -n +$((RETENTION + 1)) | xargs -r rm -f

echo "[$(date)] Minecraft backup done: $DEST/mc-${STAMP}.tar.gz"
```

---

## 7. Edge cases and safety guarantees

### 7.1 Container crashes during a backup cycle

The backup process re-reads container state **immediately** before stopping it. If the container is not running at that moment, the backup is silently skipped.

```
Supervisor scans → sees "running" → queues backup
Meanwhile container crashes → status becomes "stopped"
performAutomaticBackup re-reads state → sees "stopped" → returns nil
Result: backup skipped, next interval will retry
```

**No data is lost, no containers are affected.**

### 7.2 VPS reboots mid-backup

The backup archive may be partially written. After reboot:

1. `cardinal-bootstrap.service` starts the supervisor.
2. Containers with `--restart unless-stopped` are restarted by boot recovery.
3. Run `cardinal backup verify <archive>` on recent backups — checksum mismatches indicate incomplete archives.
4. The next scheduled backup cycle will create a fresh, complete archive.

### 7.3 Two backup processes run simultaneously

Impossible. Each backup acquires an **exclusive file lock** (`flock`) on the backup directory. A second concurrent process detects the locked file and exits immediately.

### 7.4 Backup and crash-loop protection

If a container is in a crash-loop (`restart_blocked: true`), it is not in `running` state, so backups are skipped. Once you manually run `cardinal start <container>` and it recovers, normal backup scheduling resumes on the next interval.

### 7.5 Backup and manual stop (`unless-stopped`)

When the backup stops the container for archiving, it sets `stopped_by_user = false` before restarting. This means:

- The container restarts normally after backup.
- The `unless-stopped` policy is not broken — the container remembers it was running before the backup.
- A manual `cardinal stop` by a user is correctly distinguished from a backup-induced stop.

### 7.6 Archive corruption

Each backup archive includes a `.sha256` checksum sidecar. Corruption is detected by:

- `cardinal backup verify` (explicit check)
- `cardinal backup restore` (automatic verification before extraction)

A corrupted archive is never silently applied. The restore operation aborts with an error.

### 7.7 Disk full during backup

If the backup directory runs out of space, the archive creation fails with an error. The container is restarted (the `restartAfterBackup` defer runs regardless of errors). Check disk space:

```bash
du -sh ~/.cardinal/backups/*
df -h /
```

Reduce `--retention` or move backups to a larger disk.

---

## 8. Backup settings reference

These settings are stored in the container's state file (`~/.cardinal/containers/<id>.json`):

| Field | JSON key | Type | Description |
|-------|----------|------|-------------|
| Auto backup | `auto_backup` | bool | Whether scheduled backups are enabled |
| Backup interval | `backup_interval` | string | Go duration (e.g., `"6h0m0s"`, `"24h0m0s"`) |
| Backup retention | `backup_retention` | int | Number of archives to keep (1–1000) |
| Backup directory | `backup_dir` | string | Absolute path to backup storage |
| Last backup time | `last_backup_at` | timestamp | When the last successful backup completed |
| Next retry time | `backup_next_attempt_at` | timestamp | When the next backup retry will be attempted (set on failure) |

**Inspect these settings:**

```bash
cardinal inspect <container> | grep -i backup
```

Example:

```json
{
  "auto_backup": true,
  "backup_interval": "6h0m0s",
  "backup_retention": 14,
  "backup_dir": "/root/.cardinal/backups/postgres",
  "last_backup_at": "2026-08-12T04:00:00+03:00"
}
```

---

## 9. Backup directory structure

```
~/.cardinal/backups/
├── postgres/
│   ├── postgres-20260812-040000.tar.gz
│   ├── postgres-20260812-040000.tar.gz.sha256
│   ├── postgres-20260811-220000.tar.gz
│   ├── postgres-20260811-220000.tar.gz.sha256
│   └── .lock                          # flock file for concurrent backup protection
├── redis/
│   ├── redis-20260812-040000.tar.gz
│   └── redis-20260812-040000.tar.gz.sha256
├── bot/
│   ├── bot-20260812-040000.tar.gz
│   └── bot-20260812-040000.tar.gz.sha256
└── custom-path/
    └── ...                            # custom --dir backups
```

**Archive naming:** `<container-name>-YYYYMMDD-HHMMSS.tar.gz`

**Checksum sidecar:** `<archive>.sha256` — contains `<sha256hash>  <filename>\n`

---

## 10. Best practices

| Practice | Why |
|----------|-----|
| **Use named volumes for databases** | cardinal backup automatically archives named volumes — zero extra setup |
| **Schedule backups during off-peak hours** | Container is stopped briefly during backup; minimize user impact |
| **Keep 7–14 daily copies for databases** | Enough recovery window without excessive disk usage |
| **Keep 3–5 copies for bind-mount cron backups** | Bind-mount backups tend to be larger (full directory tar) |
| **Verify backups periodically** | Run `cardinal backup verify` weekly to detect corruption early |
| **Download important backups offsite** | Protect against disk failure, accidental deletion, ransomware |
| **Encrypt before downloading sensitive data** | Defense in depth: `gpg -c --symmetric --cipher-algo AES256` |
| **Monitor backup disk usage** | Check with `du -sh ~/.cardinal/backups/*` and `df -h /` |
| **Test restore procedures** | A backup you cannot restore is not a backup — practice on a test container |
| **Set appropriate intervals** | Databases with frequent writes → 6h; bots with infrequent changes → 24h |
| **Combine cardinal backup with bind-mount backup** | Use `cardinal backup enable` for named volumes + cron for bind mounts |

### Disk space estimation

| Workload | Typical archive size | Retention | Total disk |
|----------|---------------------|-----------|-----------|
| Small bot (Python) | 5–20 MB | 7 copies | ~140 MB |
| PostgreSQL (< 1 GB data) | 200–500 MB | 14 copies | ~7 GB |
| PostgreSQL (5 GB data) | 2–4 GB | 14 copies | ~56 GB |
| Minecraft world (19 GB) | 12–15 GB | 5 copies | ~75 GB |
| Redis (< 500 MB data) | 100–300 MB | 7 copies | ~2 GB |

---

## 11. Troubleshooting

### Backup never runs

```bash
# Check if supervisor is running
systemctl status cardinal-bootstrap

# Check if auto_backup is enabled
cardinal backup status <container>

# Check supervisor logs
journalctl -u cardinal-bootstrap --since "1 hour ago" | grep -i backup
```

### Backup fails with "Error: stop the container before creating a consistent backup"

The container is still running. Stop it first:

```bash
cardinal stop <container>
cardinal backup create <container>
```

Automatic backups handle this automatically (they stop the container themselves).

### Backup fails with disk errors

```bash
# Check available space
df -h ~/.cardinal/backups

# Check backup sizes
du -sh ~/.cardinal/backups/*

# Reduce retention
cardinal backup disable <container>
cardinal backup enable <container> --retention 3
```

### Restore fails with "checksum mismatch"

The archive is corrupted. Use a different backup:

```bash
cardinal backup list
# Pick a different archive and verify it
cardinal backup verify /path/to/different-archive.tar.gz
cardinal backup restore <container> /path/to/different-archive.tar.gz
```

### Restore fails with "backup belongs to container X, not Y"

The archive was created from a different container. You cannot restore a backup from container A into container B. Recreate the container with the original name first.

---

## 12. Quick reference

```bash
# ─── Setup ───
cardinal bootstrap --install                                    # install supervisor

# ─── Automatic backups ───
cardinal backup enable <container> --interval 6h --retention 14  # enable
cardinal backup disable <container>                               # disable
cardinal backup status <container>                                # check status

# ─── Manual backups ───
cardinal stop <container>                                         # stop first
cardinal backup create <container> -o /path/to/file.tar.gz       # create
cardinal start <container>                                        # restart

# ─── Verify ───
cardinal backup verify /path/to/file.tar.gz                      # check integrity

# ─── Restore ───
cardinal stop <container>
cardinal backup verify /path/to/file.tar.gz
cardinal backup restore <container> /path/to/file.tar.gz
cardinal start <container>

# ─── List ───
cardinal backup list                                              # list all archives

# ─── Download to local PC ───
scp root@SERVER:~/.cardinal/backups/<container>/<file>.tar.gz ./
rsync -avz root@SERVER:~/.cardinal/backups/ ./local-backups/

# ─── Encrypt for offsite storage ───
gpg -c --symmetric --cipher-algo AES256 backup.tar.gz
gpg -d backup.tar.gz.gpg > backup.tar.gz
```

For complete CLI syntax, see [Command Reference](commands.md). For practical recipes, see [Examples](examples.md).

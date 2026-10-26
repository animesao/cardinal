<!-- cardinal-version:start -->
**Documentation version:** `2.4.11`
**Project release:** `v2.4.11`
<!-- cardinal-version:end -->

# State and storage

`cardinal` has no database and no required daemon. The data directory is the
source of truth: images, containers, volumes, logs, overlays, backups, and
orchestrator state are files on disk. Two CLI invocations coordinate by reading
and atomically replacing those files, not by an RPC service.

## Data directory

`state.DataDir()` resolves in this order:

1. `CARDINAL_DATA_DIR`, if set (cleaned with `filepath.Clean`);
2. `/root/.cardinal` when the effective uid is 0;
3. `$HOME/.cardinal` otherwise.

`state.init()` also forces `HOME=/root` when running as root. This matters for
tools that expand `~` inside the runtime.

`state.EnsureDirs()` creates every top-level directory with mode `0700` and
re-applies the mode on each call:

```
<data-dir>/
  images/                 OCI image rootfs + metadata
  containers/             one <id>.json per container
  logs/                   one <id>.log per container
  overlay/                per-container upper/, work/, merged/
  volumes/                one directory per named volume
  consoles/               console sockets
  cache/layers/           downloaded layer cache
  registry-allowlist      optional persistent registry hostname list
  backups/                default backup root
  audit/                  optional JSON-lines audit log
  cluster/cluster.json    cluster membership and node state
  services/services.json  service definitions
  services/<name>/<id>.json  replica records
  functions/functions.json   FaaS definitions
```

The exact set of leaf files can grow between releases. Treat the directory as
an implementation detail; do not hand-edit JSON unless you are recovering a
host and have a backup.

## Path safety

All path construction goes through `internal/state`:

- `safePart` accepts only a non-empty basename that is not `.` or `..` and
  contains no slash or backslash. Anything else becomes a SHA-256-derived key.
- `ImageDir` validates the repository name and tag. Invalid references are
  routed to `images/.invalid/<hash>` rather than being allowed to steer the
  filesystem path.
- `ResolveVolume` treats a source without `/` or `\` as a named volume under
  `volumes/`; a path with a separator is a host path and must pass
  `validateBindSource`.
- `validateVolumeName` rejects `.`, `..`, and any separator.
- `validateContainerTarget` requires an absolute target, rejects `..`, `/`, and
  `.`.
- `validateBindSource` resolves symlinks and applies the host-path blocklist.

## Atomic writes

`state.WriteFileAtomic` is the persistence primitive:

1. create the parent directory with `0700`;
2. create a temporary file in the **same** directory;
3. write, `fsync`, close;
4. `os.Rename` over the destination;
5. re-apply the requested mode.

JSON state is written with mode `0600`. A crash therefore leaves either the old
file or the new file, never a half-written JSON document. Directory creation and
mode changes are not atomic with the rename, so a crash can still leave an empty
directory.

**Concurrency:** atomic replacement prevents torn reads, but it is not a
cross-process transaction. Two CLI processes that read, modify, and write the
same container JSON can lose one update. The runtime uses in-process mutexes for
a single process; it does not take a per-container file lock. Backup creation is
the exception: it uses a non-blocking `flock` on `<backup-dir>/.lock`.

## Images

`internal/image` stores each image under
`images/<repository>/<tag>/`:

```
image.json      cardinal metadata (name, tag, digest, config, layers, ...)
config.json     OCI image config (Env, WorkingDir, User, Cmd, ...)
manifest.json   legacy metadata read for older images
rootfs/         extracted filesystem used as the overlay lower layer
```

- `SaveToStore` writes `image.json`; `LoadFromStore` falls back to
  `manifest.json` for images created by older releases.
- `RemoveImage` removes the whole image directory.
- Downloaded layers are cached under `cache/layers/<digest>/layer.tar.gz`.
  A digest is stripped of its `sha256:` prefix before being used as a directory
  name, and the remainder goes through `safePart`.
- Layer extraction rejects traversal, unsafe links, devices, hardlinks, NUL
  paths, self-referential symlinks, duplicate metadata, and oversized archives.
- Pull verifies each blob digest. `cardinal verify` checks manifest and layer
  digests without starting a container.

**Partial / risk:** the layer cache is not reference-counted. Removing an image
removes its image directory; cached layers are not automatically garbage
collected by `system prune` unless the image removal path reaches them.

## Containers

Each container is one JSON document at `containers/<id>.json`, written by
`Container.Save()` through `WriteJSON`. The document includes identity, image,
command, environment, mounts, ports, network mode, resource limits, security
settings, restart policy, timestamps, namespace PIDs, cgroup path, IP, and
backup schedule fields.

Runtime companions:

- `logs/<id>.log` — stdout/stderr stream;
- `overlay/<id>/upper`, `work`, `merged` — copy-on-write layers;
- `consoles/<id>.sock` — console socket when enabled.

The container JSON is the contract used by the CLI, the API, the supervisor, and
the orchestrator. Renaming a field breaks remote mode, panel integrations, and
older binaries. Treat field names as public API.

**Recovery:** because state is plain JSON, a host can be inspected and repaired
offline. The usual recovery sequence is: stop the container, inspect the JSON,
remove stale overlay or namespace metadata if the host crashed, then start it
again. Do not copy a data directory between hosts while containers are running.

## Volumes

A named volume is a directory at `volumes/<name>/` plus a JSON metadata file
inside it. `CreateVolume` validates the name, refuses an existing directory,
creates it with `0700`, and rolls back the directory if metadata cannot be
written.

- `RemoveVolume` refuses a volume referenced by any container. It scans both
  running and stopped containers (`List(true)`).
- Bind mounts are host directories and are never copied into the volume store.
- Container mount targets are validated; host sources are resolved and checked
  against the protected-path blocklist.

## Prune

`container.SystemPrune()` performs, in order:

1. remove every container whose status is not `Running`;
2. remove unused named volumes;
3. remove images not referenced by any container;
4. remove overlay directories with no matching container JSON;
5. remove log files with no matching container JSON.

Consequences to know before running it:

- Stopped containers are deleted, not just their stopped state.
- The image reference set is computed from the container list captured at the
  start, so an image used only by a container created concurrently can be
  removed.
- `pruneUnusedVolumes` only protects volumes referenced by a container that
  still exists. A deleted container's volumes become eligible.
- `docs/COMMANDS.md` currently says prune never removes volumes. That statement
  is stale; see [`known-issues.md`](../status/known-issues.md).

## Backups

`cmd/backup.go` implements `create`, `list`, `restore`, `enable`, `disable`,
`status`, `verify`, `remove`, and `generate-key`.

- The default root is `<data-dir>/backups`; a container-specific directory can
  override it with `--dir`.
- Automatic backups default to a 24-hour interval and 7-copy retention, with
  validation bounds of 1–1000 copies.
- An archive is a `.tar.gz`; encrypted output is `.tar.gz.enc`.
- A `.sha256` sidecar is written next to the archive. `verify` checks it;
  `restore` warns loudly when it is missing.
- `--encrypt` uses AES-256-GCM with a key from `CARDINAL_BACKUP_KEY`.
  `generate-key` prints a new key; it does not store it for you.
- Backup creation takes a non-blocking `flock` on `<backup-dir>/.lock` and
  fails with an actionable error if another backup is running.
- The archive includes container writable data and metadata. Host bind mounts
  are **not** included, and overlay whiteout files are skipped.
- Restore validates the archive path, rejects symlinked ancestors, rejects
  absolute and parent-traversing entries, and stages extraction before applying
  it. `--rebind` restores into a newly created container.

**Risk:** the checksum sidecar detects corruption and accidental modification,
not a malicious actor who can rewrite both the archive and the sidecar. Use
encryption and an external trust anchor when archives cross a trust boundary.

## Orchestrator state

All orchestrator state is JSON under the data directory:

| Path | Contents |
|---|---|
| `cluster/cluster.json` | node identity, role, peers, cluster metadata |
| `cluster/cluster.json.bak` | backup copy written during cluster updates |
| `services/services.json` | service definitions |
| `services/<name>/<replica>.json` | per-replica placement record |
| `functions/functions.json` | FaaS function definitions |

Writes use `state.WriteFileAtomic` with `0600`. The cluster file also keeps a
`.bak` copy; the service and function files do not.

## Disk usage and cleanup

`cardinal system df` reports Images, Containers, Overlay, Volumes, Logs,
Backups, and Cache. The safe cleanup order is:

1. stop or remove containers you no longer need;
2. remove unused images explicitly (`cardinal rmi`);
3. remove unused volumes explicitly (`cardinal volume rm`);
4. only then run `cardinal system prune`.

`system prune` is destructive and has no confirmation prompt in the current
implementation. Take a backup first if the data matters.

## Verification

| Check | Command |
|---|---|
| State path and atomic-write tests | `go test ./internal/state/ -count=1` on Linux |
| Volume tests | `go test ./internal/container/ -run Volume -count=1` on Linux |
| Prune tests | `go test ./internal/container/ -run Prune -count=1` on Linux |
| Backup path tests | `go test ./cmd/ -run Backup -count=1` on Linux |
| Image store tests | `go test ./internal/image/ -count=1` on Linux |

On Windows, these packages may compile but the runtime and filesystem
semantics are not exercised.

<!-- cardinal-version:start -->
**Documentation version:** `2.4.11`
**Project release:** `v2.4.11`
<!-- cardinal-version:end -->

# Known issues and documentation drift

This file records places where documentation and code disagree, plus known
technical gaps. It is a register, not a roadmap. Label each item with the fact
status from the root `AGENTS.md` and point to evidence.

Last audited: 2026-10-26, commit `64bf22f`, `VERSION=2.4.11`.

## Documentation drift

### D-01 — `system prune` and volumes

- **Status:** Stale documentation.
- **Documents:** `docs/COMMANDS.md` says `system prune` removes stopped
  containers, unused images, and orphan overlays/logs, "never volumes".
- **Code:** `internal/container/prune.go` calls `pruneUnusedVolumes()` and
  reports removed volumes.
- **Impact:** an operator may run prune expecting volumes to survive.
- **Action:** update the command table in `docs/COMMANDS.md` and both language
  mirrors.

### D-02 — `serve` token requirement

- **Status:** Stale documentation.
- **Documents:** `docs/COMMANDS.md`, `docs/en/commands.md`, and
  `docs/ru/commands.md` say a token is required only for external binds.
- **Code:** `internal/api/server.go` refuses to serve without a token on every
  bind, including loopback. `docs/en/commands.md` and `docs/ru/commands.md`
  still say "external binds require a token" and "the API still requires a
  Bearer token for external binds".
- **Impact:** a user may start `cardinal serve` without a token and get an
  unexpected startup failure, or assume loopback is unauthenticated.
- **Action:** state that a token is mandatory on every bind and loopback is
  included.

### D-03 — `SECURITY.md` seccomp/AppArmor status

- **Status:** Stale documentation.
- **Document:** `SECURITY.md` says JSON profiles ship and overrides are
  accepted, but the filters are "not yet applied".
- **Code:** the default seccomp profile **is** applied in
  `internal/container/init.go`; custom seccomp and AppArmor profiles are
  rejected fail-closed in `internal/container/start.go`; AppArmor is not
  applied.
- **Impact:** readers underestimate current hardening and misunderstand why a
  custom profile fails.
- **Action:** describe the default seccomp filter, the `CARDINAL_SECCOMP=0`
  escape hatch, and the fail-closed custom-profile behavior.

### D-04 — Usage guide seccomp status

- **Status:** Stale documentation.
- **Documents:** `docs/en/usage.md` and `docs/ru/usage.md` say the BPF compiler
  is unimplemented and the filter is not applied.
- **Code:** `internal/container/seccomp.go` implements the cBPF compiler;
  `ApplyDefaultSeccompProfile()` runs before `execve`.
- **Impact:** users may disable or distrust a working control.
- **Action:** update both language mirrors.

### D-05 — `TODO-SECURITY.md` item 2 checkbox

- **Status:** Stale checklist.
- **Document:** item 2 is unchecked but its own text says the default seccomp
  filter is wired in `InitContainer`, with tests.
- **Code:** the implementation and tests exist.
- **Impact:** the checklist understates completed work.
- **Action:** check the box or rewrite the item; do not leave a contradictory
  checkbox.

### D-06 — `docs/AUDIT.md` runtime snapshot

- **Status:** Stale snapshot.
- **Document:** says the default seccomp profile is "accepted but fails closed
  until the BPF compiler lands".
- **Code:** the default profile is applied; only custom profiles fail closed.
- **Impact:** audit readers get the wrong runtime posture.
- **Action:** refresh the snapshot or mark it historical with a date.

### D-07 — Build documentation toolchain and workflow names

- **Status:** Resolved on 2026-10-26.
- **Document:** `docs/build.md` previously said Go 1.25, module `go 1.25.0`,
  toolchain 1.26.6, and referenced `.github/workflows/ci.yml` and
  `release.yml`.
- **Fix:** updated to `go 1.26.0` / toolchain `1.26.10` and the actual workflows
  `build.yml`, `e2e.yml`, `scheduled.yml`, and `branch-guard.yml`.
- **Evidence:** `go.mod`, `.github/workflows/`, `docs/build.md`.

### D-08 — README version blocks are outside the generic marker check

- **Status:** Open check gap; values currently match `2.4.11`.
- **Documents:** the top `cardinal-version` block in `README.md`,
  `docs/README.md`, and `contrib/gentoo/README.md`, `contrib/nix/README.md`,
  `contrib/void/README.md`.
- **Gate:** `scripts/sync-docs-version.sh` excludes basenames `README.md` from
  the generic marker scan. `--check` validates the root README badge and release
  block, but not these top marker blocks.
- **Impact:** a future `VERSION` bump can leave the first visible version line
  stale without failing CI.
- **Action:** either drop the redundant blocks from READMEs or add a
  README-aware check. Until then, update them manually on release.

### D-09 — Root help says "read-only remote mode"

- **Status:** Stale help text.
- **Document:** `cmd/cobra.go` root help says `--host` is for "read-only remote
  mode".
- **Code:** remote support includes `start`, `stop`, `restart`, `exec`, `stats`,
  `top`, `inspect`, `events`, and `logs -f`.
- **Impact:** users may not realize remote mode is writable.
- **Action:** update the help string; keep the flag description accurate.

### D-10 — `cmd/serve.go` token comment

- **Status:** Stale comment.
- **Code comment:** "Token: flag > env var > disabled".
- **Actual behavior:** `api.StartServerWithTLS` refuses to serve with an empty
  token.
- **Impact:** a maintainer may reintroduce a tokenless path believing it is
  supported.
- **Action:** change the comment to "token is mandatory".

### D-11 — `internal/client` package comment

- **Status:** Stale comment.
- **Comment:** describes the client as powering "read-only remote mode".
- **Actual behavior:** the client has mutating methods (`Start`, `Stop`,
  `Restart`, `Exec`) and the CLI uses them.
- **Impact:** low; misleading for contributors.
- **Action:** update the package comment.

## Technical gaps and risks

### T-01 — No cross-process lock for container JSON

- **Status:** Implemented limitation.
- **Evidence:** `state.WriteFileAtomic` prevents torn writes, but container
  read-modify-write paths do not take a per-container file lock. Backup creation
  is the exception (`flock` on `<backup-dir>/.lock`).
- **Impact:** two concurrent CLI/API mutations can lose an update.
- **Action:** add a per-container lock if concurrent writers become a supported
  scenario; document the single-writer assumption until then.

### T-02 — `system prune` race with concurrent creates

- **Status:** Implemented behavior with a race.
- **Evidence:** `SystemPrune` captures the container list, then removes images
  not referenced by that snapshot.
- **Impact:** an image used by a container created during prune can be removed.
- **Action:** re-check references immediately before removal or serialize prune
  with container creation.

### T-03 — Default containers run as host uid 0

- **Status:** Implemented limitation.
- **Evidence:** `ValidateRuntimePolicy` only rejects explicit root users; an
  unset `--user` also runs as uid 0. There is no user namespace by default.
- **Impact:** uid 0 in the container is uid 0 on the host; capability drops and
  seccomp limit but do not eliminate the risk.
- **Action:** document prominently; consider a user namespace by default.

### T-04 — AppArmor and custom seccomp are not applied

- **Status:** Planned.
- **Evidence:** `startInternal` rejects `SeccompProfile` and `AppArmorProfile`;
  `apparmor.go` contains a template and loader helpers, but the start path does
  not call them.
- **Impact:** operators cannot use custom profiles yet.
- **Action:** implement and test profile application, or keep the fail-closed
  rejection and label the feature planned.

### T-05 — Device restrictions are best effort

- **Status:** Partial.
- **Evidence:** `ApplyDeviceRestrictions` failures are logged, not fatal.
- **Impact:** on a kernel that rejects the mounts, `/dev/shm` or `/dev/mqueue`
  may be less restricted than documented.
- **Action:** decide whether these should be fatal on supported kernels; at
  minimum, surface the warning in `doctor`/`security check`.

### T-06 — Layer cache is not reference-counted

- **Status:** Partial.
- **Evidence:** layers live under `cache/layers/<digest>/`; image removal removes
  the image directory. `SystemPrune` does not explicitly garbage-collect the
  layer cache.
- **Impact:** disk usage can grow beyond what `system df` suggests.
- **Action:** add cache accounting and a safe GC pass.

### T-07 — API advertises Docker 1.44 but routes are unversioned

- **Status:** Implemented compatibility choice.
- **Evidence:** `DockerAPIVersion = "1.44"`, but no `/v1.44/...` handler exists.
- **Impact:** a Docker client that prefixes paths with the negotiated version
  will get 404.
- **Action:** document the unversioned contract; add a version-prefix shim only
  if a real client requires it.

### T-08 — Cluster and service reconciliation are not transactional

- **Status:** Partial.
- **Evidence:** cluster state has a `.bak`; service and function state do not.
  Replica start and record persistence are separate operations.
- **Impact:** a crash can leave an untracked container or an orphan record.
- **Action:** add reconciliation that treats the container list as the source
  of truth for orphan detection.

### T-09 — DNS server is minimal

- **Status:** Partial.
- **Evidence:** `internal/orchestrator/dns.go` answers A records over UDP.
- **Impact:** no DNSSEC, recursion, caching semantics, or TCP fallback.
- **Action:** document the limits; use an external DNS server for production
  service discovery.

### T-10 — Rootless `pasta` backend is detected but not wired

- **Status:** Partial.
- **Evidence:** `RootlessNetworkBackend` returns `pasta`, but
  `SetupRootlessNetwork` returns an error for it and asks for `slirp4netns`.
- **Impact:** a host with only `pasta` cannot use rootless bridge networking.
- **Action:** wire `pasta` or remove it from the detection list.

### T-11 — Plain HTTP is the API default

- **Status:** Implemented default.
- **Evidence:** TLS requires both `--tls-cert` and `--tls-key`.
- **Impact:** a Bearer token sent over plain HTTP is visible on the network.
- **Action:** document TLS for any non-loopback deployment; consider requiring
  TLS for external binds.

## Static audit warnings at 2.4.11

`sh scripts/audit.sh` reports `FAIL: 0`, `WARN: 5`:

1. plain-looking password/secret pattern in docs/compose (placeholder review);
2. `install.ps1` builds from source without a clone SHA256;
3. `install-appimage.sh` does not verify a SHA256 (local AppImage install);
4. coverage profile freshness is not enforced;
5. manual review of `SYS_MODULE` handling.

These are tracked design debt, not verified vulnerabilities. Re-run the audit
before claiming they are resolved.

## Environment limitations of this audit

- Windows host, no Linux kernel: native Linux runtime tests, privileged smoke
  tests, and e2e were not run.
- `go test ./... -count=1` passed for the packages buildable on Windows
  (`cmd`, `builder`, `client`, `config`, `image`, `network`, `overlayutil`,
  `portmap`, `state`). Linux-only packages were excluded by build tags.
- Linux test binaries for all 14 packages compiled with
  `GOOS=linux GOARCH=amd64 go test -c`; they were not executed. Cross-compiled
  `go build ./...` and `go vet ./...` also passed.
- `gofmt -l` flags every tracked Go file on this checkout because the files use
  CRLF line endings; no Go file was changed in this documentation pass, so that
  result is not attributable to this work.
- `bash` is not on `PATH`; `sh` and `make` are available. `scripts/audit.sh`
  was run with `sh`. `make audit` was not run because its recipe calls `bash`
  directly; `make docs-check` is viable because it calls `sh` and `python3`.
- `sh scripts/sync-docs-version.sh --check` and
  `python3 scripts/check-docs-links.py` passed after the documentation changes;
  re-run them before closing later edits.

## How to close an item

1. Fix the code or the document.
2. Add or update a test if behavior changed.
3. Update both language mirrors for user-facing docs.
4. Remove the item from this file, or mark it resolved with the commit/PR that
   closed it.
5. Re-run the narrowest verification from the root `AGENTS.md`.

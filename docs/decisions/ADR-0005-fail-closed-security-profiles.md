<!-- cardinal-version:start -->
**Documentation version:** `2.4.13`
**Project release:** `v2.4.13`
<!-- cardinal-version:end -->

# ADR-0005: Fail closed on security configuration

- **Status:** Accepted
- **Date:** 2026-10-26
- **Scope:** Runtime start policy, API startup, custom profiles

## Context

A security flag that is accepted but not applied is worse than a flag that is
rejected: the operator believes a protection exists. Examples include a custom
seccomp profile that is parsed but never installed, an AppArmor profile that is
stored but never loaded, and an API that starts without authentication because
the bind looks local.

`cardinal` cannot yet implement every profile format. The question is what to
do when a request cannot be honored.

## Decision

Fail closed. If a requested security control cannot be applied, refuse the
operation with an explicit error. Do not silently downgrade to a weaker mode.
The only exceptions are documented, loud, operator-controlled escape hatches.

## Evidence

- `internal/container/start.go` rejects `SeccompProfile` and `AppArmorProfile`
  with "not enforced yet" errors.
- `internal/container/security_policy.go` rejects dangerous capabilities and
  explicit root users unless acknowledged.
- `internal/container/init.go` aborts if `applyCapabilities`, `no_new_privs`, or
  `ApplyDefaultSeccompProfile` fails.
- `internal/api/server.go` refuses to serve without a token on every bind,
  including loopback.
- `internal/orchestrator/cluster.go` fails closed when no peer token is
  configured.
- Documented escape hatches: `CARDINAL_SECCOMP=0` (logged warning),
  `CARDINAL_REGISTRY_STRICT=0`, `CARDINAL_ALLOW_INSECURE_REGISTRY=1`,
  `CARDINAL_ALLOWED_HOST_PATHS`, `--allow-dangerous-caps`, `--allow-root`.

## Consequences

Positive:

- an operator cannot believe a control is active when it is not;
- tests can assert a refusal instead of a silent downgrade;
- the default path is the safe path.

Negative:

- some workloads that "worked" under a silently ignored profile now fail;
- users must wait for profile support or use the documented escape hatch;
- error messages become part of the user experience and must be actionable.

## Rules

1. A new security option must either be enforced or rejected. Never accept and
   ignore it.
2. Every escape hatch needs a name, a warning, and documentation.
3. Do not weaken a control to make a test or demo pass.
4. Do not turn a failure to apply a control into a warning unless the control is
   explicitly best effort (for example device restrictions).
5. Security policy belongs in the runtime layer so CLI and API cannot diverge.

## Alternatives rejected

- **Warn and continue.** Rejected because it produces a false sense of safety
  and hides drift.
- **Parse and ignore unknown profiles.** Rejected as the worst case: the
  operator pays the configuration cost without the protection.
- **CLI-only checks.** Rejected because the API would become a bypass.

## Related

- [`../architecture/security.md`](../architecture/security.md)
- [`../status/known-issues.md`](../status/known-issues.md)

<!-- cardinal-version:start -->
**Documentation version:** `2.4.13`
**Project release:** `v2.4.13`
<!-- cardinal-version:end -->

---
name: security-audit
description: Comprehensive security auditing, vulnerability scanning, static analysis (SAST), secret detection, and container runtime vulnerability review for Go and Linux container systems.
---

# Security Audit Skill (Cardinal Container Runtime)

This skill provides procedures, automated tooling workflows, and checklist rubrics for analyzing codebases (specifically Go and container runtime projects) for vulnerabilities, secret leaks, and security regressions.

## 1. Automated Security Tooling

### Go Vulnerability Database (govulncheck)
Checks dependencies and Go standard library against known CVEs:
```powershell
$env:GOOS='linux'; $env:GOARCH='amd64'; govulncheck ./...
```

### Static Application Security Testing (gosec)
Scans Go source code for common security flaws (hardcoded credentials, SQL injection, integer overflow, unsafe reflection, insecure cryptography, file traversal):
```powershell
gosec -fmt=text ./...
```

### Secret Leak Detection (gitleaks)
Detects leaked secrets, API keys, private keys, and tokens in working tree and git commit history:
```powershell
gitleaks detect -v --source .
```

### Project Baseline Static Audit (scripts/audit.sh)
Runs the project-specific static audit suite:
```powershell
& "C:\Users\Anime\scoop\apps\git\current\bin\bash.exe" scripts/audit.sh
```

## 2. Container Runtime Security Checklist

When auditing container engines and runtime code:

1. **Namespace & Isolation**:
   - Verify `pivot_root` is used instead of bare `chroot`.
   - Ensure `/proc` remounting uses a fresh procfs inside a new PID namespace to avoid `/proc/1/root` host escapes.
   - Validate target namespace identities before `nsenter` to prevent PID recycling attacks.

2. **Privilege & Capability Management**:
   - Check bounding set dropping (`PR_CAPBSET_DROP`) and effective/permitted sets.
   - Enforce `PR_SET_NO_NEW_PRIVS`.
   - Guard dangerous capabilities (`CAP_SYS_ADMIN`, `CAP_SYS_MODULE`, `CAP_SYS_RAWIO`, etc.) with fail-closed checks.

3. **Filesystem & Mount Protections**:
   - Inspect bind mount sources against system and credential directories (`/proc`, `/sys`, `/etc`, `~/.ssh`).
   - Guard against symlink traversal and TOCTOU races in archive extractions (`safeTarPath`, `ensureNoSymlinkAncestors`).
   - Limit decompression size and entry counts against zip/tar bombs.

4. **Network & API Security**:
   - Require bearer tokens on all binds (including loopback).
   - Guard against DNS-rebinding (`Host` header validation) and SSRF in registry pulls.
   - Avoid `http.DefaultClient` or unbounded `io.ReadAll(resp.Body)` without context deadlines and size limits.

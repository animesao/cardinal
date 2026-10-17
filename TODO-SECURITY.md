<!-- cardinal-version:start -->
**Documentation version:** `2.4.3`
**Project release:** `v2.4.3`
<!-- cardinal-version:end -->

# TODO — устранение находок аудита безопасности cardinal (16.10.2026)

Источник: аудит container-escape поверхности (Docker-класс).
Статусы: `[ ]` — не начато, `[x]` — сделано + `go build/vet/test` зеленые.

## Этап 1 — Critical (изоляция, малая кровь)

- [x] **1. Минимальный /dev** — `PruneDevtmpfs` + allowlist
  (`dev_restrictions.go`), вызов в `InitContainer` после монтирования,
  `MKNOD` убран из дефолта (`init.go`) с докой про `--cap-add MKNOD`.
  Тесты `dev_prune_test.go` (3 шт). Проверка: `GOOS=linux go vet` + компиляция
  тестов OK (рантайм-тесты — CI на linux).
- [ ] **2. Seccomp default wired — ЗАБЛОКИРОВАНО корневой причиной.**
  Расследование: `buildSeccompFilter` (`seccomp.go:355-376`) — заглушка,
  возвращает пустую BPF-программу. Включение `ApplyDefaultSeccompProfile`
  в init либо убьёт все сисколлы, либо ничего не даст. Нужна настоящая
  компиляция BPF (отдельный большой этап). AppArmor同样: apply есть,
  вызова нет. Зафиксировано как известный разрыв, не чинить полумерой.

## Этап 2 — High (API/сеть/supply chain)

- [x] **3. Токен обязателен + route_localnet за флагом.**
  `api/server.go`: отказ старта без токена на любом bind (тест
  `TestStartServerRefusesTokenlessBind`); `network.go`: `route_localnet=1`
  только при `CARDINAL_ROUTE_LOCALNET=1`. Проверка: `GOOS=linux go vet` +
  компиляция тестов OK (рантайм — CI).
- [x] **4. Realm-пин registry** (`image/pull.go`): `realm`-host обязан совпадать
  с registry (allowlist легитимного сплита: Docker Hub `auth.docker.io`),
  иначе отказ. Тест `TestAllowedTokenHost` (9 кейсов). Проверка: `go vet` +
  компиляция тестов OK (рантайм — CI).
- [x] **5. Капы распаковки слоёв** (`overlayutil/util.go`): 100k записей / 4G
  на entry / 16G всего, как в `Import`. Тесты: lying-header bomb rejected +
  normal layer accepted. Проверка: `go vet` + компиляция тестов OK.
- [x] **6. `update-vps.sh`**: пин тега `CARDINAL_VERSION` (fail closed),
  секреты reuse-or-generate вместо хардкода, `set -e`-safe sed.
  Проверка: чтение файла (bash на Windows нет, синтаксис проверен глазами).

## Этап 3 — Medium (доверие и контроли)

- [x] **7. TOFU/verify.** `verify` честно говорит, что проверяет только
  локальную консистентность (`cmd/verify.go`). Digest-pin/cosign —
  архитектурный этап (parseRef без digest-поддержки), зафиксирован.
- [x] **8. Registry-allowlist.** `CheckRegistryAllowed` wired в pull и push
  (`image/pull.go`, `push.go`); тест `TestCheckRegistryAllowedGate`.
  Проверка: `go vet` + компиляция тестов OK.
- [x] **9. Egress-фильтр.** DROP `127/8` + `169.254/16` из bridge в голове
  FORWARD (`network.go`), gateway/DNS целы. Проверка: `go vet` OK.
- [x] **10. Backup checksum.** Громкий warning при отсутствии sidecar
  (`backup.go`) + хелпер `backupChecksumPresent`. Проверка: `go vet` OK.
- [x] **11. Blueprint.** `http://`-репозитории отклоняются (`blueprint_repo.go`),
  конфиг не затирается, fetch-путь отказывает с понятной ошибкой
  (`blueprint.go`). Проверка: `go vet` OK.
- [x] **12. Кластер.** `requireClusterToken`: join/schedule без токена —
  отказ вместо тихих неавторизованных RPC (`cluster.go`, `scheduler.go`).
  Plaintext транспорта — архитектурно (note). Проверка: `go vet` OK.
- [x] **13. Login без эха.** Пароль без эха через `x/sys/unix` (уже в vendor),
  warning про `-p` (`cmd/login.go`). Токен в `Environment=` unit оставлен:
  0600 + осознанный комментарий в коде, альтернатива (LoadCredential) —
  отдельным этапом. Проверка: `go vet` OK.

## Этап 4 — docs и архитектура (всё по реальности кода)

- [x] **Гейты в runtime-слой**: `internal/container/security_policy.go`
  (`ValidateRuntimePolicy`, `DangerousCapsRequested` с `ALL`, `IsRootUser`);
  `cmd/security.go` делегирует; API create энфорсит + новые поля
  `AllowDangerousCaps`/`AllowRoot`, `Privileged: true` → 403.
  Тесты `security_policy_test.go`. Проверка: `go vet` OK.
- [x] **seccomp/apparmor fail-closed**: `start.go` отказывает при заданных
  профилях вместо тихого игнора. Проверка: `go vet` OK.
- [x] **SECURITY.md переписан под правду**: MKNOD вне дефолта, гейты CLI+API,
  seccomp/AppArmor «planned, not enforced», минимальный /dev, токен обязателен,
  allowlist enforced, честный `verify`, чеклист обновлён.
- [x] **docs en/ru usage + commands + AUDIT + README**: убраны заявления про
  seccomp/AppArmor «по умолчанию», MKNOD из дефолтных списков.
  Проверка: сверены глазами (доки не влияют на сборку).

## Принятые риски (без кода)

- CapAdd `ALL`/host-net через API — by design при однотокенной модели.
- Межконтейнерный трафик открыт — паритет с Docker, есть `--isolated`.
- suid-биты в слоях — приглушено `no_new_privs` (остаток: bind-mount exec с хоста).

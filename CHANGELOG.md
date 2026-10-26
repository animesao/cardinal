<!-- cardinal-version:start -->
**Documentation version:** `2.4.7`
**Project release:** `v2.4.7`
<!-- cardinal-version:end -->

## Unreleased

## 2.4.11 (2026-10-26)

### Fixed

- **Опубликованные порты на `127.0.0.1` теперь действительно открываются.**
  Проброс вида `-p 127.0.0.1:8080:80` создавал правило DNAT, но трафик до
  контейнера не доходил: ядро отбрасывало пакеты с loopback-адресом на
  bridge-интерфейсе, а обратный NAT возвращал ответы на `127.0.0.1` и они
  упирались в INPUT-фильтры. Теперь создаются оба согласованных правила:
  scoped POSTROUTING MASQUERADE (`-s 127.0.0.0/8 -d <подсеть bridge>`) и
  `net.ipv4.conf.<bridge>.route_localnet=1` для конкретного bridge. Оба
  INPUT-guard'а пропускают `ESTABLISHED,RELATED`, поэтому ответы reverse NAT
  не блокируются, а сами guard'ы по-прежнему fail closed. Глобальный
  `CARDINAL_ROUTE_LOCALNET=1` остался необязательным расширенным opt-in.

- **`cardinal run` больше не теряет аргументы после образа.** Парсер
  переставлял флаги cardinal в начало команды и заодно мог утащить аргументы
  контейнера. Теперь аргументы после имени образа сохраняются в исходном
  порядке, если их нельзя однозначно трактовать как флаги cardinal. Команды
  вида `cardinal run alpine echo --help` доходят до контейнера как есть.

### Changed

- Обновлены Go-зависимости: `github.com/prometheus/client_golang` 1.24.1 →
  1.25.0 и `golang.org/x/sys` 0.48.0 → 0.49.0.
- GitHub Actions переведены на Node 24: `checkout@v7`, `setup-go@v7`,
  `upload-artifact@v7`, `download-artifact@v8`. Входные параметры, которые
  используются в workflow, не изменились.

### Removed

- Удалена устаревшая npm-запись из `.github/dependabot.yml`: каталога
  `/compose-examples` и файлов `package.json` в репозитории нет, из-за чего
  каждый запуск Dependabot падал с `dependency_file_not_found`. Остались
  только экосистемы `gomod` и `github-actions`.

### Docs

- Добавлена карта контекста для AI-агентов (`AGENTS.md`, `docs/ai/`) и
  проверка относительных ссылок `scripts/check-docs-links.py` в CI.

## 2.4.10 (2026-10-22)

### Fixed

- **`cardinal system prune` не освобождал тома.** Тома переживают удаление
  контейнера: контейнер уходит, а его тома остаются сиротами. `system prune`
  чистил контейнеры, образы, overlay и пустые логи, но тома не трогал
  вовсе - отдельная команда `cardinal volume prune` существовала, но в общий
  prune не входила.

  На практике это выглядело так: нода без контейнеров держала 2.5 ГБ в
  `/root/.cardinal/volumes`, `cardinal system prune` рапортовал «Nothing to
  prune», а диск при этом был заполнен на 100% и SQLite отдавал «database or
  disk is full» на любую запись (импорт блюпринта, логин, удаление записи).

  Теперь `system prune` удаляет и тома, на которые не ссылается ни один
  контейнер. Занятый том удалить нельзя: `RemoveVolume` перепроверяет
  использование контейнерами.

### Added

- Тесты на чистку томов: сирота удаляется, том в контейнере остаётся, нода
  без контейнеров чистится целиком.


## 2.4.9 (2026-10-22)

### Security

- **Go 1.26.10 — закрыт GO-2026-6809.** Сборка рантайма переведена на
  Go 1.26.10: это первый релиз с исправлением `crypto/x509` (недоверенные
  скомпрометированные промежуточные сертификаты). В Go 1.26.9 уязвимость
  присутствовала, поэтому рантайм на ней оставался под уязвимостью к
  проверке цепочек сертификатов.

  Затронуты `go.mod` (`toolchain go1.26.10`) и все сборочные воркфлоу:
  `build.yml`, `e2e.yml`, `scheduled.yml`.

  Обновление на узле:

      sudo cardinal update

### Changed

- Требование Go в сборке: 1.26.10 вместо 1.26.9.


## 2.4.8 (2026-10-20)

### Fixed

- **`cardinal update` падал на проверке подписи.** Проверка cosign включена
  по умолчанию (это закрывало находку M11 — контрольная сумма бралась с
  того же сервера, что и сам архив), но написана была под несуществующие
  артефакты:
  - подпись искалась рядом с per-file чексуммами
    `cardinal_<ver>_checksums.txt.sig`, которых в релизе нет — воркфлоу
    кладёт только `SHA256SUMS.txt` и `SHA256SUMS.txt.sig`;
  - идентичность сертификата была прибита к воркфлоу `release.yml`,
    которого в репозитории нет — подписывает `build.yml`.

  Проверка не могла пройти ни при каких условиях, а включена она по
  умолчанию — обновление было сломано у всех. Теперь проверяется сам
  манифест `SHA256SUMS.txt` (он покрывает все артефакты релиза сразу), а
  идентичность указывает на `build.yml` и привязана к тегу.
- **Подпись релиза не доезжала до релиза.** Воркфлоу подписывал
  `SHA256SUMS.txt`, но в списке файлов релиза подписи и сертификата не
  было: подпись создавалась и тут же терялась. Теперь
  `SHA256SUMS.txt.sig` и `SHA256SUMS.txt.cert` выкладываются в релиз.
- **Ошибка проверки подписи ничего не объясняла.** Если релиз не подписан,
  сообщение теперь прямо говорит «release vX.Y.Z is not signed» и указывает
  адрес подписи, а не «cosign binary not found».

### Added

- **`CARDINAL_COSIGN_PUBKEY`** — путь к файлу открытого ключа в системе.
  Ключ лежит в репозитории, поэтому взлом релиз-хоста не позволяет подделать
  подпись, а `cosign` в системе не нужен вовсе. Без этого переменной
  проверка идёт по OIDC-идентичности воркфлоу: подпись должна быть сделана
  конкретным запуском `build.yml` на конкретном теге.

### Changed

- Проверка подписи по умолчанию требует подписанного релиза. Пока в
  секретах репозитория не настроен `COSIGN_PRIVATE_KEY`, обновление
  ставится с `CARDINAL_REQUIRE_SIGNATURE=0` — отказ происходит **до** записи
  нового бинарника на диск и до подмены установленного.

## 2.4.7 (2026-10-19)

Релиз по итогам полного аудита безопасности платформы (панель, wings, бот).
Закрыты три критические цепочки и всё, что к ним вело.

### Security

- **Ломающее изменение поведения: выход из контейнера через `cardinal exec`.**
  Команда выполнялась под демоном: uid 0, полный набор capability демона, без
  seccomp и без device-cgroup. Внутри контейнера этого хватало, чтобы выйти
  наружу:

      mknod /tmp/x b 8 0 && mount /tmp/x /mnt && cat /mnt/etc/shadow

  `CAP_MKNOD` создавал ноду блочного устройства, `CAP_SYS_ADMIN` его монтировал,
  а контейнеры работают без собственного user namespace, поэтому capability
  была настоящей.
  Теперь профиль контейнера применяется вторым шагом. Init-процесс
  bind-mount'ит бинарник cardinal внутрь контейнера и кладёт рядом профиль
  безопасности; exec-путь запускает

      nsenter ... -- /cardinal/.cardinal exec-helper -- <команда>

  а helper до `exec` применяет capability-набор, пользователя, `no_new_privs`
  и seccomp — тот же порядок и тот же код, что у init-процесса, поэтому они
  не могут разойтись.
- **Окружение демона больше не попадает в контейнер.** Процесс внутри
  контейнера наследовал `os.Environ()` демона, а в нём живут
  `CARDINAL_TOKEN` (root-эквивалентный токен API демона),
  `CARDINAL_BACKUP_KEY` (ключ шифрования бэкапов) и `DOCKER_PASSWORD`
  (креды реестра). Helper подставляет окружение самого контейнера.
- **Панель: первый OAuth-вход больше не становится админом.** При пустой
  таблице `users` (дефолт после `npm run db:migrate`) первый вошедший через
  OAuth получал `role=admin` — без всякой аутентификации. Первого админа
  теперь создаёт оператор: `npm run create-admin` либо
  `POST /api/setup/bootstrap` с `PANEL_SETUP_TOKEN`.
- **Панель: OAuth-идентичность привязана к `providerAccountId`, а не к
  e-mail.** Смена e-mail у провайдера или переиспользованный корпоративный
  адрес отдавали чужой аккаунт вместе с ролью `admin`. Аккаунт с паролем
  больше не входит через OAuth.
- **Панель: 2FA действует на всех путях входа.** Раньше TOTP проверялся
  только внутри credentials-провайдера, то есть OAuth-вход — самый популярный —
  второй фактор полностью обходил.
- **Панель: троттлинг входа больше не обходится заголовком.** Бакет строился
  как `${ip}:${email}`, где `ip` брался из `X-Forwarded-For`, то есть из
  заголовка, который подделывает клиент. Лимит теперь по идентичности
  (e-mail), а IP-ведро считается по реальному адресу сокета.
- **Панель: расписание больше не даёт владельцу шелл.** Команда в расписании
  уходила в exec-эндпоинт как shell-строка. Теперь это команда игрового
  сервера, доставляемая в stdin, и требует права `control.console`.
- **Панель: legacy-SFTP больше не обходит отзыв прав.** Общий пароль
  владельца перестал работать, как только у сервера появились персональные
  SFTP-креденшелы.
- **Панель: загрузка файлов больше не роняет панель.** Лимит был только на
  размер одного файла: запрос из N файлов по 256 МБ держал в памяти N*256 МБ.
  Добавлены лимиты на суммарный размер и на число файлов.
- **API: `POST /containers/create` проверяет конфликт host-портов.** Второе
  отображение того же порта молча перехватывало DNAT-правило первого
  контейнера — это проверял CLI, но не API, а через API работают wings и
  панель. Теперь API отвечает 409.
- **API: чужие Origin и Host отклоняются.** Раньше CORS только отражал
  заголовок, но никогда не отклонял запрос: «простые» кросс-сайтовые
  запросы (`POST /containers/{id}/start`) выполнялись, просто без чтения
  ответа. Добавлена проверка Host против DNS-rebinding.
- **Реестры: allowlist строгий по умолчанию.** `IsRegistryAllowed`
  возвращал `true` для любого хоста, если `CARDINAL_REGISTRY_STRICT` не был
  задан, — allowlist был документацией, а не политикой. Ссылка на образ
  собирала URL реестра из пользовательского ввода, так что pull мог уйти в
  metadata-сервис или во внутреннюю сеть. Строгий режим включается по
  умолчанию, `CARDINAL_REGISTRY_STRICT=0` его отключает.
- **Обновления: подпись релизов проверяется по умолчанию.** Раньше cosign
  проверялся только при `CARDINAL_REQUIRE_SIGNATURE=1`, а контрольная сумма
  бралась с того же хоста, что и архив. Отключается явно через
  `CARDINAL_REQUIRE_SIGNATURE=0`.
- **Бэкапы: ключ выводится через PBKDF2.** Парольная фраза сводилась к
  ключу одним SHA-256, поэтому её перебор шёл на полной скорости SHA-256.
  Теперь PBKDF2-HMAC-SHA256 (600k итераций) со случайной солью в файле.
  Старые архивы читаются как раньше: формат с префиксом `CARDBAK2` отличает
  новый.
- **`cardinal build` больше не видит корень хоста.** RUN-шаг шёл под
  `unshare` без `--mount-proc`, поэтому `/proc` внутри сборки был procfs
  хоста, а PID 1 — init хоста: `/proc/1/root` указывал на корень хоста, и
  RUN-шаг мог его читать и писать в него. Свежий procfs внутри namespace
  закрывает оба пути.
- **helper-mount больше не смонтировать overlay поверх `/etc`.** Пути из
  командной строки требовались внутри каталога состояния, а helper работает
  через sudo.
- **`cardinal update`:** то же, что и подпись релизов выше.
- **Бот: вебхук GitHub больше не публикует поддельные релизы.** Содержимое
  поста теперь берётся из GitHub API по тегу, а не из тела вебхука, и при
  пустом секрете вебхук отвечает 503 вместо «принять всё». Подпись
  сравнивается по байтам: не-ASCII в заголовке раньше ронял обработчик
  необработанным TypeError.
- **Бот: ReDoS в `/automod_add_word`.** Паттерн от модератора компилировался
  стандартным `re`, который не умеет прерывать матч: `(a+)+$` вешал event loop
  бота навсегда. Паттерны теперь компилируются модулем `regex` с жёстким
  таймаутом, катастрофические отбрасываются до записи в БД.
- **Бот: секреты больше не лежат в файлах.** Ключ Groq был зашит в
  отслеживаемый `ai_relays.py` прямо под комментарием «сюда ключ НЕ писать».
  Теперь секреты читаются только из переменных окружения, а бот не
  стартует, если секрет найден в `config.toml`.

### Fix

- `removeExistingDNAT` не учитывал протокол: UDP-маппинг порта N удалял
  существующее TCP-правило того же порта.
- `RestrictMountOperations` передавал `0x38` (`PR_SET_MDWE`) вместо
  `PR_SET_NO_NEW_PRIVS` (38 = `0x26`): prctl отвергал вызов с EINVAL, то
  есть ограничение молча не применялось.
- Имя контейнера не валидировалось, хотя попадает в `/etc/hosts` контейнера и
  в DNS-регистрацию.
- `handleImagePush` возвращал тело ошибки реестра, в котором может быть эхо
  переданных кредов. Ответ теперь редактируется.
- `privileged: true` в compose-файле разбирался и молча игнорировался. Теперь
  об этом сообщается.
- Квота и cooldown ИИ в боте списывались только после успешного ответа
  провайдера, поэтому сбой или пустой ответ не стоили ничего, а N параллельных
  запросов проходили проверку до списания. Списание теперь до вызова.

### Internal

- Exec: добавлен helper-процесс (`internal/container/exec_helper.go`,
  `internal/container/execprofile.go`, `cardinal exec-helper`), профиль
  безопасности контейнера кладётся в контейнер при старте, а namespace'ы
  по-прежнему входят через `nsenter`.
- API: проверка конфликта портов вынесена в
  `internal/container/hostport.go`, её используют и CLI, и API.
- API: добавлена проверка Origin и Host, редакция ошибок реестра, валидация
  имени контейнера.
- Контейнер: `--isolated`, `--audit-log` и `--encrypted-backup` теперь
  применяются по-настоящему, а не просто сохраняются в состоянии.
- Реестры: `CARDINAL_REGISTRY_STRICT` по умолчанию включён, добавлена
  проверка, что хост реестра не указывает во внутреннюю сеть.
- Бэкапы: ключ выводится через PBKDF2, формат файла получил заголовок
  `CARDBAK2`, файл ключа переехал в каталог состояния.
- Сборка: `unshare` получил `--mount-proc`; `helper-mount` проверяет пути.
- CI: в lint добавлен gitleaks — секрет в дереве репозитория попадает в
  каждую копию и в бэкапы, и `.gitignore` его не спасает.

### Примечание для операторов

- Контейнеры, запущенные до этого релиза, нужно перезапустить один раз:
  helper exec'а разворачивается при старте контейнера. До перезапуска
  `cardinal exec` в такой контейнер откажется с понятной ошибкой.
- `CARDINAL_REGISTRY_STRICT=1` теперь включён по умолчанию: реестры вне
  списка (Docker Hub, ghcr.io, quay.io, gcr.io) отклоняются. Свой реестр
  добавляется через `CARDINAL_REGISTRY_ALLOWLIST`.
- `cardinal update` теперь требует подпись релиза. Если подписи нет или
  проверка недоступна, обновление прерывается; отключить можно через
  `CARDINAL_REQUIRE_SIGNATURE=0`.
- Автоматические бэкапы шифруются, если у контейнера включён
  `--encrypted-backup`, и требуют `CARDINAL_BACKUP_KEY`.

## 2.4.6 (2026-10-17)

### Security

- **Ломающее изменение поведения.** Теперь к каждому контейнеру по умолчанию
  применяется seccomp-фильтр. Раньше фильтр компилировался (v2.4.5), но не
  вызывался — контейнеры работали вообще без ограничения syscall'ов.
  - **Что теперь отбивается с EPERM:** `mount`, `umount2`, `pivot_root`,
    `chroot`, `bpf`, `perf_event_open`, `userfaultfd`, `ptrace`,
    `process_vm_readv/writev`, `keyctl`, `add_key`, `request_key`,
    `sethostname`, `setdomainname`, `clock_settime`, `settimeofday`,
    `adjtimex`, `swapon`, `swapoff`, `syslog`, `init_module`,
    `delete_module`, `finit_module`, `reboot`, `kexec_load`,
    `kexec_file_load`, `open_by_handle_at`, `name_to_handle_at`.
  - **Если контейнеру нужен `mount`/`chroot` внутри:** `CARDINAL_SECCOMP=0`
    отключает фильтр для этого контейнера с явной записью в лог. `--cap-add`
    тут не поможет: фильтр срабатывает независимо от capabilities.
  - Модель allow-by-default: запрещено только перечисленное, остальное
    работает как раньше.
  - Fail-closed: если фильтр применить не удалось, контейнер не стартует,
    а не стартует без защиты.

### Fix

- `IsSeccompFilterActive()` всегда возвращал `false`: сравнение
  `line[:9]` с восьмисимвольным литералом `"Seccomp:"` не может быть
  истинным. `SeccompSupported()` выглядел рабочим только потому, что промахивался
  мимо поля и уходил в `return true`.

### Internal

- Seccomp: профиль по умолчанию применяется в `InitContainer` последним
  шагом перед `execve` — после снятия capabilities и после remount `/` в
  read-only, поскольку этот remount сам является вызовом `mount(2)`.
  Компилятор cBPF с проверкой `AUDIT_ARCH_*` и отбивкой диапазона x32
  добавлен в v2.4.5.
- Тесты: `TestAppliedSeccompFilterDeniesAndAllows` перезапускает тест-бинарник
  как дочерний процесс, ставит настоящий фильтр и проверяет вердикт ядра
  (`mount` и `pivot_root` дают EPERM, `stat` и `open` проходят). Root не
  требуется — `PR_SET_NO_NEW_PRIVS` делает `PR_SET_SECCOMP`
  непривилегированным.


## 2.4.5 (2026-10-17)

### Internal

- Seccomp: профиль по умолчанию теперь действительно компилируется.
  `buildSeccompFilter` был заглушкой, возвращавшей пустую BPF-программу, —
  применить его можно было только с ошибкой, и никто его не вызывал.
  Компилятор собирает cBPF-программу с проверкой `AUDIT_ARCH_*`, отбивкой
  диапазона x32 на amd64 (x32 использует тот же audit-арх, что и x86_64, и
  иначе обходил бы deny-list целиком) и одним сравнением на каждый
  запрещённый syscall. Номера syscall'ов берутся из `unix.SYS_*` в
  per-arch таблицах — так были найдены `SYS_UMOUNT` и `SYS_STIME`,
  которых нет в таблице syscall'ов x86_64. Профиль переведён на
  allow-by-default с `SCMP_ACT_ERRNO` по deny-list: прежний KILL-by-default
  allow-list не содержал `futex`, `epoll_*`, `statx`, `newfstatat` и `rseq`,
  поэтому его честная компиляция убила бы любой контейнер на первом же
  ожидании futex, а девять syscall'ов были указаны одновременно как KILL и
  как ALLOW. `clone3` и `madvise` больше не запрещаются (glibc 2.34+
  использует `clone3` для `pthread_create`).
  **В init пока намеренно не подключено:** непроверенный фильтр либо не
  запрещает ничего, либо валит все контейнеры сразу. `--seccomp-profile`
  по-прежнему честно возвращает ошибку вместо имитации поддержки.

## 2.4.4 (2026-10-17)

### Security

- Scheduled security scan снова зелёный — устранены обе причины падения.
  - **gitleaks**: коммит `9030fd8` содержал захардкоженные `JWT_SECRET`
    и `ADMIN_PASSWORD` в heredoc systemd-юнита в `update-vps.sh`. Скан
    проверяет всю историю, поэтому находка возвращалась на каждом прогоне,
    хотя сам скрипт починили ещё в v2.3.13. Добавлен allowlist, отвязанный
    строго от этого SHA: утечка в любом другом коммите по-прежнему валит
    сборку.
  - **govulncheck**: 9 CVE в стандартной библиотеке (`net/http`,
    `net/textproto`, `crypto/tls`), фиксятся в Go 1.26.9. `scheduled.yml`
    и `e2e.yml` были закреплены на 1.26.6 — подняты до 1.26.9;
    `install.ps1` и `snapcraft.yaml` теперь тоже собирают пропатченный stdlib.
  - **`go.mod`**: добавлена директива `toolchain go1.26.9`. При обычном
    `GOTOOLCHAIN=auto` сборка на Go 1.26.0–1.26.8 подтягивает пропатченный
    тулчейн, а не линкует уязвимый stdlib в релизный бинарник. Директива
    `go 1.26.0` не тронута — версия языка не меняется, поднимается только
    порог тулчейна.
  - Значения из `9030fd8` задокументированы в `SECURITY.md` как
    скомпрометированные, с командами ротации. Allowlist скрывает находку от
    сканера, но не делает ключи снова секретными: хосты, где отработал
    `update-vps.sh` до v2.3.13, обязаны перегенерировать оба значения.
  - `SECURITY.md` убран из path-allowlist gitleaks. Значения там больше не
    цитируются буквально, поэтому любой реальный секрет, добавленный в файл
    позже, не будет молча проигнорирован.

### Fix

- `internal/image` снова собирается на не-Linux. `allowlist.go` был помечен
  `//go:build linux`, хотя код платформенно-нейтрален, а `pull.go`/`push.go`
  вызывают `CheckRegistryAllowed` безусловно — пакет не компилировался вне
  Linux, и `go build`/`go vet`/`govulncheck ./...` падали на любой не-Linux
  машине. Теги сняты с `allowlist.go` и `allowlist_test.go`, так что тесты
  registry-allowlist теперь реально выполняются на всех платформах, а не
  выпадают из сборки.


## 2.4.3 (2026-10-17)

### Fix

- Disk-limit больше не блокирует запуск контейнеров на хостах без
  loop-устройств (LXC/OpenVZ-стиль VDS): если `mount -o loop` падает именно
  из-за отсутствия `/dev/loop*`, cardinal пишет предупреждение и стартует
  контейнер без квоты вместо ошибки 500. Остальные причины падения
  (битый образ, нет места, ошибка `mkfs`) по-прежнему фатальны.
  `CARDINAL_STRICT_DISK_LIMIT=1` возвращает прежнее fail-closed поведение.
- `cardinal doctor` показывает `disk limits`: OK при наличии loop-устройств,
  WARN с объяснением на хостах, где квота не может быть применена.

## 2.4.2 (2026-10-17)

### Fix

- Ошибки disk-limit монтирования теперь объясняют себя: при падении
  `mount -o loop` cardinal добавляет, чего именно не хватает (образ,
  точка маунта, `/dev/loop-control`, свободный loop-девайс) вместо
  голого `No such file or directory`. Ошибка создания каталога образа
  тоже больше не глотается.

## 2.4.1 (2026-10-17)

### Fix

- Cobra wrapper больше не выдёргивает `--host`/`-H`/`--token` из аргументов
  `serve` (флаг токена терялся, serve с обязательным токеном отказывался
  стартовать — ломал автоспавн из wings).
- Disk-limit: ошибки создания каталогов больше не глотаются — вместо
  загадочного `mount failed: No such file or directory` видно реальную
  причину (нет места, иноды, права).

## 2.3.14 (2026-10-16)

### Fix

- `cardinal serve --token` снова доходит до сервера: cobra-обёртка больше не
  выдёргивает `--host`/`-H`/`--token` из аргументов `serve` в remote-переменные
  (флаг приходил пустым, и serve с обязательным токеном отказывался
  стартовать — ломал автоспавн из wings и e2e). Wings дополнительно дублирует
  токен через `CARDINAL_TOKEN` в окружении дочернего процесса.

## 2.3.13 (2026-10-16)

### Security

- Minimal container `/dev` (pruned to pseudo-devices; `MKNOD` removed from the default capability set, grant via `--cap-add MKNOD`).
- API server refuses to start without a token on any bind, loopback included; `route_localnet` is opt-in (`CARDINAL_ROUTE_LOCALNET=1`).
- Registry token-realm pinning (Docker Hub split-host allowlisted); registry allowlist enforced on pull and push under `CARDINAL_REGISTRY_STRICT=1`.
- Layer extraction caps (100k entries / 4G per entry / 16G total, mirroring `Import`).
- Egress DROP for `127.0.0.0/8` and `169.254.0.0/16` from the bridge.
- Backup restore warns loudly without a checksum sidecar; blueprint repos must be https; cluster join/schedule RPCs fail closed without `CARDINAL_TOKEN`; `login` reads the password without echo.

### Stats: live in-place refresh

- `cardinal stats` (local and remote) now redraws the table in place instead of scrolling a new header plus rows every second: cursor-up plus clear plus repaint per tick, with per-tick line counting so skipped rows on errors cannot desync the picture.
- `--no-stream` still prints a single snapshot with no escape codes; piped output and `TERM=dumb` keep scrolling.

### Stats: disk usage includes named volumes

- `DiskUsage` (`disk_usage_bytes`) now sums the overlay upper dir **and** the container's named volumes. Panel servers keep all data in `volumes/srv-<uuid>`, so the old upper-only walk reported kilobytes on gigabyte servers.
- Full tree walks are cached per container with a 60s TTL and opportunistic eviction of stale entries — stats are polled every second and uncached walks of gigabyte trees would trash disk I/O.
- Bind/tmpfs/NFS sources are still excluded (host admin territory, same as Docker).
- Regression tests: `internal/container/stats_disk_test.go` (`dirTreeSize`, `isNamedVolume`, cache TTL expiry).

### Assets only (no runtime changes)

- `img/cardinal.png` replaced (1 376 604 -> 1 670 161 bytes, 2026-10-13); the file is referenced by the `README.md` header logo. No code or docs changes.

## 2.3.12 (2026-10-15)

### Console

- `cardinal attach`: same console-input logging as v2.3.11, errcheck-clean (`Close` checked via closure per codebase style).

## 2.3.11 (2026-10-15)

### Console

- `cardinal attach`: client stdin is teed into the container log as `> cmd` lines — reconnect replays restore commands next to their answers instead of answers alone. Single funnel for every client (CLI, wings); bytes to the container unchanged.

## 2.3.10 (2026-10-15)

### Console

- `cardinal attach`: console tail replay starts at a line boundary (the byte-sliced first fragment could neither dedupe nor sit next to its command).

## 2.3.9 (2026-10-15)

### Console

- `cardinal attach`: same piped-stdin echo as v2.3.8, with checked `Fprintln` returns (errcheck-clean CI).

## 2.3.8 (2026-10-15)

### Console

- `cardinal attach`: echo piped stdin lines locally (container stdin is a pipe with no echo back; terminal input already echoes via the kernel, unchanged there). Socket bytes unchanged.

## 2.3.7 (2026-10-15)

### Security hardening

- `cardinal serve on`: the systemd unit embedding `CARDINAL_TOKEN` is now written `0600` (was world-readable `0644` via umask).
- DNS/hosts: servers in `HostConfig.Dns` must be IPs before landing in `resolv.conf`; `/etc/hosts` entries are validated (no whitespace/control characters), so crafted names cannot inject lines into neighbours' hosts files.
- Port protocol allowlist (`tcp/udp/sctp/icmp`, normalized lowercase): the raw suffix no longer reaches `iptables -p`.
- Regression tests: `internal/portmap` protocol cases.

## 2.3.4 (2026-10-09)

### CI only (no runtime changes)

- `.github/workflows/build.yml`: `actions/setup-go` bumped from Go `1.26.6` to `1.26.9` in three jobs (lint/test, per-package tests, reproducible cross-compile). No Go code changes in this tag (`go.mod` still targets `go 1.26.0`).
- Known drift (not changed by this tag): `e2e.yml`, `scheduled.yml`, `install.ps1` (`$GoVersion`), and `snap/snapcraft.yaml` (`GO_VERSION`) still pin Go `1.26.6`; `docs/build.md` still documents `1.26.6`.
- Release-process note: tag `v2.3.4` was cut on the merge of this CI change without a `VERSION` bump (`VERSION` is still `2.3.1`), so the `cardinal-current-release` marker below still reports `v2.3.1` until `VERSION` is bumped via `scripts/sync-docs-version.sh`.

## 2.3.1 (2026-10-04)

### Green build (no runtime changes)

- Fix `errcheck` failures in `cmd/remote_exec_test.go` (unchecked `fmt.Fprint` in the SSE/logs stub handlers).
- Fix `go test -race` data race in `TestHandleEventsSSE`/`TestHandleContainerLogsFollow`: both tests now read the streaming body through a mutex-guarded recorder instead of racing the handler goroutine on `httptest.ResponseRecorder.Body`.

## 2.3.0 (2026-10-04)

### Remote mode expansion + host-port conflict UX

- Remote `exec` with captured output: `POST /containers/{id}/exec` now returns `Output`/`Stderr`/`ExitCode` (non-zero exits stay HTTP 200; interactive `-i`/`-t` fails loudly with a wings-terminal hint).
- Remote `stats` (CLI polls one-shot `?stream=0` every second), `top`, `inspect` (Docker schema), `events` (new SSE `GET /events` with `?since=` replay + live stream), `start`/`stop`/`restart` (including client-side `stop --all`), and streaming `logs -f` (rotation-safe server follow loop).
- Host-port conflicts: `cardinal run` and `cardinal port add` refuse a host port already mapped by another container (silent DNAT hijack) and warn when the host port is already bound (e.g. cardinal-wings on 8080); README quick-start moved to 8081 with a reserved-ports note.
- E2E: new smoke steps for duplicate-port rejection, occupied-port warning, and an experimental (non-blocking) bridge forward check.
- Docs version checks extended: CHANGELOG current-release marker auto-syncs, README release block is check-enforced.
- `install.ps1`: release pin via `-Version` (default pinned tag) and Go 1.26.6 in the MSI fallback.

## 2.2.0 (2026-10-03)

### Remote mode: query a remote `cardinal serve` from any machine

- New global flags `--host/-H` and `--token` (env `CARDINAL_REMOTE_HOST`/`CARDINAL_TOKEN`; deliberately not `CARDINAL_HOST`, which already selects the serve bind address).
- `cardinal ps`, `cardinal info` and `cardinal logs --tail N` work against a remote endpoint through a new `internal/client` package (plain `net/http`, no new dependencies).
- Remote `logs -f/--previous/--all` fail loudly: the server ignores `follow` and has no previous/rotated logs, so the CLI reports that instead of returning partial data.
- Legacy `DisableFlagParsing` commands receive the flags via `extractRemoteFlags`; `ps` keeps cobra parsing with the flags bound as persistent flags.

## 2.1.5 (2026-09-24)

### Compose: variable interpolation, multi env_file, port ranges

- Compose-style variable substitution in `image`, `ports`, `volumes`, `environment`, `command`, `entrypoint`, `hostname`, `working_dir`, and `env_file` paths — `$VAR`, `${VAR}`, `${VAR:-default}`, `${VAR-default}`, `$$`. Lookup order: host environment first, then a `.env` file next to the compose file.
- `env_file` now accepts a list of files (a plain string still works) and each path is interpolated; `environment` keys without a value (`KEY` / `KEY: null`) inherit from host/`.env`.
- Long-syntax volumes support `type: bind` and `type: tmpfs` with `tmpfs.size` and `read_only`.
- Port mappings support host and container ranges across the shared parser: `cardinal run -p 8000-8010:80`, compose `ports`, and blueprints (`8000-8010:80`, `8000-8010:9000-9002`); invalid specs now fail with a clear error instead of being silently dropped.
- New `internal/portmap` package with `container.ParsePortMapping` shared by `run`, `up`, and blueprints (14 unit-test cases); `run` rejects anonymous host ports (`-p 9000`) with an actionable message.

## 2.1.4 (2026-09-21)

### Re-release on green build

- Same code as 2.1.3, fresh tag so consumers can pin a green build of the release pipeline.

## 2.1.3 (2026-09-21)

### Contributors and cleanup

- README contributors section, `.env.example` removed, `develop` merge.

## 2.1.2 (2026-09-21)

### Uninstall and deps

- `uninstall.sh` rewrite with safer cleanup, `golang.org/x/sys` bump, dependabot merge.

## 2.1.1 (2026-09-21)

### Installer and pull progress

- Linux installer rework: mirror-first downloads with timeouts and IPv4 fallback.
- Pull progress bar fix; `.gitignore` covers build artifacts.

## 2.1.0 (2026-09-20)

### Rootless storage fallback, pull retry, run --replace

- Overlay mount tries native, fuse-overlayfs, sudo helper, then vfs copy, with aggregated actionable errors; containers boot on bare WSL2 without extra packages.
- Registry token fetch: 30s timeout with retries and backoff instead of a single 5s attempt.
- `cardinal run --replace` replaces an existing same-name container; failed starts print retry/remove hints.
- Rootless network backend detection (slirp4netns/pasta) with clear errors and `--network host/none` guidance.
- Branching model: `develop` integration branch, PR guard workflow, CI on `develop`.

### Rename kuranix -> animesao

- GitHub owner renamed `kuranix` -> `animesao`: all repo URLs, raw/API links, `*.github.io` pages, maintainer emails, goreleaser/nfpm metadata and docs updated.
- Blueprint repo migration now accepts both legacy `kuranix/*` and `animesao/*` personal-fork URLs.
- No functional changes.

## 2.0.35 (2026-09-19)

### Rename animesao -> kuranix

- GitHub owner renamed `animesao` -> `kuranix`: all repo URLs, raw/API links, `*.github.io` pages, maintainer emails, goreleaser/nfpm metadata and docs updated.
- No functional changes.

## 2.0.34 (2026-09-16)

### Command audit and help fixes

- Audited all 52 CLI commands: every registration resolves to an existing handler, no duplicates, docs matrix matches code exactly.
- Fix `cardinal backup --help` missing the `generate-key` verb (dispatcher already supported it); refreshed the stale subcommand comment.

## 2.0.33 (2026-09-16)

### Dead code removal and lint cleanup

- Remove 18 unused symbols flagged by the tightened `unused` linter: dead compose refs, capability/namespace/restart-guard helpers superseded by live paths (`applyCapabilities`, `RestartBlocked` field, `containerNamespaceIdentities`), `ignoreErrWriter`, `flushBridgeNeigh`, builder `hashFile`, `ensureBuildDirs`, API `writeOK`/`containerStatus`, backup-restore wrapper, and the unset `system df` count field.
- Fix `ineffassign` in the container router fallback (`err = nil` replaced with a clean rescan).
- Fix `gofmt` alignment/blank-line nits across 16 files; `golangci-lint run ./...` is now clean.

## 2.0.32 (2026-09-16)

### CI, API, docs, and CLI hygiene fixes

- Fix `scheduled.yml` daily audit job: it referenced the deleted `reusable-audit.yml`; now runs `checkout` + `bash scripts/audit.sh` directly.
- Fix `go.mod`: promote `creack/pty` from indirect to direct dependency (matches `vendor/modules.txt`); `go.sum` verified stable.
- Close two `501 not implemented` API stubs: `GET /containers/{id}/export` streams the container filesystem as tar, `GET /images/{name}/get` streams the image store as tar (404 when the data directory is missing).
- Sync docs with reality: `docs/AUDIT.md` no longer references deleted workflows and its snapshot reports SBOM/fuzz as PASS; `docs/COMMANDS.md` uses real make targets (`test-race`, `e2e.yml`); `SECURITY.md` points to compose `secrets:` instead of a nonexistent `cardinal secret` command; `CHANGELOG.md` structural cleanup (removed mid-file logo and duplicate heading).
- Harden input handling: `blueprint install --cpus/--memory` and blueprint template `CPUs`/`Memory` now fail with exit 1 on invalid values instead of silently becoming 0; `/proc/meminfo` parsing no longer ignores `Sscanf` errors; `ps -a` flag read checks its error.
- Tighten `.golangci.yml`: enable `ineffassign`, `gofmt`, `goimports`, `misspell`, `godox`; drop blanket `func/type/field is unused` exclusions.
- Remove dead code (`var _ = time.Now`, `var _ = container.New` + unused import) and deduplicate the `run`/`set` help texts into shared `runLongHelp`/`setLongHelp` constants reused by the root help; `.gitignore` now covers `*.log`/`*.tmp`.

## 2.0.31 (2026-09-08)

### Lint fix

- Fix `staticcheck` SA5011 possible nil pointer dereferences in `cmd/cluster.go` (`clusterJoinToken`, `clusterInfo`, node lookup) and `cmd/consoleserve.go`: added `return` after the `exitFunc(1)` nil guards. The analyzer cannot prove a function variable never returns, and the `return` also keeps stubbed-`exitFunc` paths (tests) from dereferencing nil.

## 2.0.30 (2026-09-08)

### CLI hardening: no more untrappable exits

- Add a process-exit seam (`cmd/exit.go`): every command now terminates via a stubbable `exitFunc` instead of a raw `os.Exit`, plus `failf` and `mustParse` helpers. Production behaviour is unchanged; commands are now unit-testable without killing the test binary.
- Migrate all 25 stdlib flag sets from `flag.ExitOnError` to `flag.ContinueOnError`: the flag package can no longer terminate the process itself. `-h`/`--help` still prints usage to stderr and exits 0; parse failures print `Error parsing <name> options` and exit 1.
- Unify exit codes: the 8 remaining `exit 2` paths (usage errors in `backup`, `doctor`, `registry`) now exit 1, matching cobra and every other command.
- `internal/container.generateID`: `panic` with a stack trace instead of `os.Exit` on `crypto/rand` failure (unrecoverable OS entropy fault; panic stays visible and recoverable for library-style callers).
- Regression tests (`cmd/exit_test.go`): exit-code capture, help/usage path, `stripGlobalFlags` and `hasLongHelpArgument` tables.
- Docs: corrected the outdated "bridge needs `-network host` for DNS" advice — bridge containers resolve external names out of the box (default `8.8.8.8`/`8.8.4.4`, override with `--dns`); `-network host` is only for binding host interfaces directly.

## 2.0.29 (2026-09-08)

### Release hygiene: version sync

- Fix version drift that blocked `scripts/sync-docs-version.sh --check`: root `VERSION` was stuck at `2.0.12` while tags and `README.md` had moved to `v2.0.28`. Bumped to `2.0.29`, re-synchronized all documentation version markers, the README version badge, the README release block, and the changelog current-release marker.
- Removed stale `cardinal-client` installer reference from `README.md` Quick Start (installer ships cardinal only, from GitHub).

### Catch-up since 2.0.12 (previously unreleased notes for 2.0.13-2.0.28)

- Installer: universal installer ships cardinal + wings + bootstrap in one shot; mirror-first downloads with hard timeouts and IPv4 fallback; installer fetches cardinal only from GitHub.
- Registries: pull images from foreign registries (ghcr, quay, gcr); no ghcr placeholder scope for token requests.
- Runtime: startup script runs as the container main process; startup script and env updates apply via `cardinal set`; console-serve lifecycle lines no longer pollute container logs; attach and console stream handling improved.
- Resources: live memory/CPU cgroup and disk-resize updates via `POST /containers/{id}/update`; dropped unused resize2fs output.
- Supervisor: `always` containers pass through recovery after reboot; scheduled backups and systemd bootstrap recovery kept.
- API: `POST /v1/images/{ref}/pull` endpoint for Wings; container API types formatted.
- Docs: site-first install links and correct wings API endpoint.

## 2.0.3 (2026-08-25)

### All commands restored after cobra migration

- Fix cobra migration regression where 90% of flags were broken with `unknown flag` — legacy `flag.NewFlagSet` commands (`stop --all`, `rm -f`, `logs -f`, `exec -it`, `up -f`, `down -a`, `update --check`, `doctor --strict`, `pull --platform`, `push -u/-p`, `login -u/-p`, `volume -d`, `network --subnet`, `events --since`, etc.) now use `DisableFlagParsing` so raw args reach legacy parsers, plus `stripGlobalFlags` for `--json`/`--quiet`/`--log-level` handling in both `cardinal --json <cmd>` and `cardinal <cmd> --json` positions.
- Add `cardinal rm -r` as alias for `-f`/`--force` (with `ContinueOnError`, combined handling, updated usage `[-f] [-r] [--force]`) to satisfy `cardinal rm -r <container>` expectation.
- Fix `cardinal exec -it` and `cardinal run -it` combined shorthands (`-it`/`-ti`/`-dit`/`-tid` etc.) via normalization before flag parse — stdlib `flag` does not support combined bools like `pflag`, fixes Docker-style `exec -it` and `run -it` regressions.
- Improve help handling for disabled commands (`--help`/`-h`/`help` via `hasLongHelpArgument`) and backup/security subcommands (`backup create/enable/etc.` now `DisableFlagParsing` with proper help).
- Verification: `go vet ./cmd/...` ok, `go test ./cmd/... -run TestNewRoot` ok, manual flag matrix for all 52 commands no longer reports `unknown flag` / `flag provided but not defined`; global opts work in both positions.

## 2.0.2 (2026-08-25)

### Blueprint registry fixes

- Fix `cardinal blueprint ls/list` `official: parse error` — `fetchURL` now uses `curl -fsSL` with timeouts and proper HTTP error propagation instead of silent HTML return.
- Improve `fetchBlueprintRegistry` diagnostics: include real `json.Unmarshal` error, 300-char preview, and hint for HTML/empty response; add hint to final error about `cardinal blueprint repo list` and `raw.githubusercontent.com` connectivity.
- Auto-migrate legacy blueprint repository URLs (`dck-organization/dck-blueprints`, `kuranix/dck-blueprints` → `cardinal-organization/cardinal-blueprints`) and normalize URLs on load; fix corrupted/empty `~/.cardinal/blueprint-repos.json` handling.

## 1.60.11 (2026-08-18)

### Backup restore

- Fix path traversal check that incorrectly rejected root directory entries during backup restore.

## 1.60.10 (2026-08-18)

### Backup restore

- Fix `cardinal backup restore --rebind` flag parsing when invoked through Cobra subcommands.

## 1.60.9 (2026-08-18)

### Backup restore safety

- Skip overlayfs whiteout files, character/block devices, fifos, and sockets during backup restore instead of aborting.

## 1.60.8 (2026-08-18)

### Test fix

- Fix backup restore regression test to correctly mark the root `data` directory entry as a tar directory type.

## 1.60.7 (2026-08-18)

### CLI help and CI stability

- Handle `cardinal run --help` and `cardinal serve --help` before forwarding arguments to legacy flag parsers.
- Prevent the legacy flag parser from calling `os.Exit(0)` during Cobra smoke tests.
- Add regression coverage for help handling on commands with disabled Cobra flag parsing.

## 1.60.6 (2026-08-18)

### Backup restore safety

- Accept the explicit root `data` directory entry produced by CARDINAL backup archives during restore.
- Keep archive namespace, traversal, absolute-path, and symlink safety checks enabled.
- Add regression coverage for restoring a backup containing `data` and `data/...` entries.

## 1.60.5 (2026-08-18)

### Backup and transfer support

- Add `cardinal backup restore --rebind` so verified backup data can be restored into a newly created container during cross-machine transfers.
- Include persistent bind-mount data in backups under a target-scoped archive namespace while retaining symlink and host-path safety checks.
- Add `cardinal backup remove` (with `rm` and `delete` aliases) for removing an archive and its checksum sidecar safely.
- Support CARDINAL Desktop backup management and container transfer with configuration, volumes, writable data, and lifecycle state preserved.

## 1.60.4 (2026-08-18)

### Desktop integration

- Document the current container inspection and mount metadata used by CARDINAL Desktop to scope per-container SFTP access.
- Keep the CARDINAL API mount targets as the source of truth for integrations that expose container filesystems.

## 1.60.3 (2026-08-18)

### Interactive console fixes

- Preserve the container stdin when running startup scripts, so Paper, Minecraft, and other interactive servers receive commands from `cardinal attach` and the desktop console.
- Restore command responses such as `tps`, `pl`, and `help` instead of leaving the console with input-only output.

## 1.60.2 (2026-08-18)

### Container startup and file management fixes

- Select the first mounted container directory as the default working directory when the image does not define one, so relative commands such as `java -jar server.jar` run from the application mount.
- Recover older containers that still use `/home/container` by detecting referenced startup files in mounted directories.
- Improve desktop binary uploads with remote-size verification, destination-size verification, cleanup, and bounded SSH/SFTP timeouts.

## 1.60.1 (2026-08-18)

### Runtime and networking fixes

- Gate startup scripts behind network namespace setup so bridge containers always receive their veth, address, and route before application startup.
- Add an init-to-supervisor network readiness handshake and regression coverage for its environment setup.
- Check the network handshake gate close error so the release passes `errcheck` in CI.

## 1.60.0 (2026-08-18)

### Runtime and networking fixes

- Ignore exited or zombie init processes while resolving a container startup target.
- Retry moving the veth peer into the container network namespace during namespace setup races.
- Include the underlying `ip` error output so network startup failures are actionable.

## 1.25.9 (2026-08-18)

### Runtime fixes

- Resolve the container init process through `/proc` process metadata and an exact `cardinal init` command-line fallback, preventing valid stopped containers from failing to start on some kernels.
- Add regression coverage for safe init-process matching.

## 1.25.8 (2026-08-18)

### Runtime fixes

- Pass image-provided environment variables to startup scripts so Java and other tools from image-defined `PATH` values are available.
- Avoid misleading `ip` warnings and unnecessary interface polling for minimal images without iproute2.
- Apply device restrictions before dropping mount capabilities, reducing avoidable mount warnings.

### API and CLI

- Add container startup-command updates through `POST /containers/<id>/update`.
- Add `cardinal set <container> --startup` with inline and `@file` script support.
- Accept canonical and compatibility JSON field names for startup scripts.

<!-- cardinal-current-release:start -->
> Current release: **v2.4.11**. Detailed release notes below are maintained manually.
<!-- cardinal-current-release:end -->

## 1.25.3 (2026-08-17)

### Improvements

- Full codebase audit: 0 FAIL, 52 cobra commands verified.
- Clean `go vet`, `go build`, and `golangci-lint` with no warnings.
- Version synchronized across all documentation files.

## 1.25.2 (2026-08-17)

### Bug fixes

- Fix `cardinal ps -a` / `cardinal ps --all` flag parsing: register `-a`/`--all` via cobra instead of DisableFlagParsing so the flag is correctly handled.
- Fix `cardinal cp` stdin support: `cardinal cp - <container>:/path` now works as source.
- Restore `Ps(args []string)` function signature for audit compliance.

## 1.25.0 (2026-08-17)

### API fixes

- Clean up ghost containers: if `container.Start()` fails after `container.Save()`, the container is now removed so it does not appear permanently in the container list.
- Container lookup fallback: `containers/<id>` API now scans the full container list when the direct ID lookup fails, matching by prefix or name.
- Auto-fix relative mount targets: volume bind targets that are not absolute (e.g. `app` instead of `/app`) are automatically prefixed with `/`.

## 1.24.18 (2026-08-16)

### Builder fixes

- Parse long Dockerfile continuation chains in linear time, preventing fuzz smoke timeouts caused by repeated string concatenation.
- Add a regression test covering thousands of continued lines.

## 1.24.17 (2026-08-16)

### API fixes

- Preserve CPU, memory, and disk resource limits in container inspection and stats responses.
- Return a clear conflict response when a requested container name is already in use instead of allowing ambiguous creation state.

### Desktop integration

- Added support for live resource reporting consumed by CARDINAL Desktop.

## 1.24.16 (2026-08-16)

### CI/CD

- Patch release: full drop-in support for **Nix / NixOS**, **Gentoo**, and **Void Linux** under `contrib/`, plus GH-Actions artifact fallback so nfpm packages still land when the GitHub Release publish step hiccups. Identical upstream build flags across every contributor format so binaries are byte-equivalent.

## 1.24.15 (2026-08-16)

### CI/CD

- Patch release: same code body as v1.24.14 with a refreshed tag so consumers can pin a green build of the goreleaser v2 release pipeline.

## 1.24.14 (2026-08-16)

### CLI

- Migrated the top-level dispatch from a hand-rolled switch / flag-parser in `cmd/root.go` to `spf13/cobra v1.8.1` with `spf13/pflag v1.0.5`. Every previously-working invocation (52 commands + 7 backup sub-commands + `security check`) is preserved as a thin cobra wrapper around the legacy free function, so existing tests and call sites stay green.
- New global flags on every sub-command: `--log-level debug|info|warn|error`, `--json`, `--quiet`.
- `cardinal completion` now generates shell scripts for **bash / zsh / fish / powershell** out of the box; installation instructions added to `docs/en/commands.md` and `docs/ru/commands.md`.
- Discovered and fixed a regression where `cardinal run --rm --network none alpine:latest …` exited silently with empty stderr. The legacy flag-parsing path inside `Run()` is reachable again via `DisableFlagParsing: true` on the `run` cobra command. A regression test (`TestRunCommand_DisablesFlagParsing`) guards against future removal of that knob.
- Surfaced cobra's runtime errors to stderr in `cmd/cobra.go` (`Execute()`), guaranteeing that any future unknown-flag / wrong-arity error is at least actionable instead of leaving a single newline and exit code 1.

### CI/CD

- Rebuilt the GitHub Actions pipeline around a single `build.yml` orchestrator that runs the full `lint → test → build matrix → SBOM → cosign → GitHub Release` flow. Replaces nine reusable workflows plus five orchestrators with three files: `build.yml`, `e2e.yml`, `scheduled.yml`.
- Pinned action versions to known-good majors (`actions/checkout@v4`, `actions/setup-go@v5`, `actions/upload-artifact@v4`, `anchore/sbom-action@v0`, `softprops/action-gh-release@v2`); dependabot now ignores semver-major bumps in `.github/dependabot.yml` so the pipeline does not silently flip to a breaking major.
- `build.yml` runs `golangci-lint`, `go vet`, `govulncheck`, `shellcheck`, `scripts/sync-docs-version.sh --check`, and `bash scripts/audit.sh --strict` on every push and PR; the build matrix produces reproducible `linux/amd64` and `linux/arm64` artefacts with `-trimpath -buildid=`, sha256 sidecars, SPDX-JSON SBOM, optional `cosign` signing, and a GitHub Release on tag pushes.
- `e2e.yml` (privileged) now always uploads a diagnostic bundle (`cardinal-e2e-diag-<run-id>`) with the captured stdout/stderr buffer, preflight dump of id / env / mount / cgroup / overlay / data-dir / df, and `if: always()` artifact upload so a future maintainer can post-mortem a failed run without re-executing it.
- `scheduled.yml` runs daily govulncheck + gitleaks + `scripts/audit.sh`; dependabot.yml groups minor+patch bumps and ignores major version updates.

### Documentation

- Added `docs/COMMANDS.md` mapping every cobra-registered command to its runtime requirements (kernel feature, network, state) and verification path (`cobra`, `unit-test`, `e2e-linux`, `audit`, `audit-by-design`).
- Expanded `SECURITY.md` with a deployment-checklist and a hardening section including: host-path blocklist (`internal/container/hostpath.go`), log-injection protection for line-delimiter / control-character payloads, registry allow-list, image authentication, and dangerous-capability gating (`--allow-dangerous-caps`, `--allow-root`).
- Added `docs/AUDIT.md` summarising every change above with a per-feature rationale and the known design debt.
- Added regression tests: Dockerfile parser fuzz (`internal/builder/fuzz_test.go`), `internal/image/allowlist_test.go`, `internal/log/logger_test.go`, `cmd/security_test.go`, OCI-layer traversal and hardlink/symlink-loop extraction.

### Removed

- Removed the unreachable `internal/ftp/` package (398 lines of plain-TCP FTP server with a single shared password). `SECURITY.md` records the decision; `scripts/audit.sh` carries a prevention check so future regressions are caught at audit time.

## 1.23.28 (2026-08-13)

### Runtime fixes

- Fixed `cardinal run` options placed after the image, including `--workdir`, being passed to the container command.
- Preserved the safe default Linux capability set so standard images such as `nginx:alpine` can perform required startup filesystem operations while dangerous capabilities remain dropped.
- Cleaned up container resources when the init process exits before namespace registration.

## 1.23.17 (2026-08-12)

### Documentation and release automation

- Added a single-source documentation version marker synchronized from `VERSION` across every Markdown file.
- Added `scripts/sync-docs-version.sh --check` and CI validation so stale documentation versions fail before release.
- Updated installation, AppImage, update, backup, verification, restart, and multi-architecture guidance in English and Russian.
- Removed unreliable third-party download mirror instructions and replaced unsafe manual binary replacement examples with `install -D`.
- Fixed duplicate help/bootstrap documentation entries.

## 1.23.0 (2026-08-11)

### Highlights since 1.22.0

- Persistent restart supervisor: detached `always` and `unless-stopped` containers are recovered after reboot or crash via systemd, with configurable `--restart-delay` and crash-loop protection (`--restart-max-attempts`, `--restart-window`, `restart_blocked`).
- Per-container scheduled backups with `cardinal backup enable/disable/status`, configurable intervals and retention, safe destinations, and checksum verification with `cardinal backup verify`.
- Offline image verification with `cardinal verify IMAGE[:TAG]` — config and layer digests are checked against the locally stored manifest.
- Reliable `cardinal update`: five-minute download timeout, per-method error reporting, bounded curl/wget fallbacks.
- Runtime hardening: zombie-exit detection, `cardinal rm` tombstones against supervisor restart races, safe OCI layer extraction (path traversal, absolute and symlink targets), protected bind sources, and volume mount modes (`:ro`/`:rw`, propagation, tmpfs, NFS).
- Instant startup for `--network none`/`host` containers (no eth0 wait).
- Docker-compatible REST API with optional HTTPS and bearer-token auth, cluster orchestration, FaaS, services, blueprints, and Compose support.
- Complete bilingual (EN/RU) documentation: command references, usage guides, practical examples, and per-application guides, including the pull → verify → run workflow and a fully synchronized Russian websites guide.

## 1.22.40 (2026-08-11)

### CLI

- `cardinal --help` no longer duplicates the backup, inspect, doctor, and security check entries in the System section (they remain listed under Container).

### Documentation

- Documented the previously undocumented `cardinal verify IMAGE[:TAG]` (offline config and layer digest verification) and `cardinal backup verify FILE.tar.gz` (checksum verification) commands, plus `-v SRC:DST` mount modes (`:ro`/`:rw`, propagation flags, tmpfs and NFS specs).

## 1.22.39 (2026-08-11)

### Documentation

- The README and running guides document crash-loop protection with `--restart-max-attempts` and `--restart-window` and the `restart_blocked` state.

## 1.22.38 (2026-08-11)

### Update reliability

- `cardinal update` now allows up to five minutes for the release download instead of timing out after ten seconds, which was too short for multi-megabyte binaries on slow links.
- The opaque `all methods failed` message is gone: each download method (Go HTTP client, curl, wget) now reports its own error so a failure can be diagnosed.
- The curl/wget fallbacks get explicit connect and max-time limits so the updater can never hang indefinitely.

## 1.22.37 (2026-08-11)

### Runtime fixes

- Container startup no longer spends up to 20 seconds polling for an `eth0` address. The wait is skipped entirely for `--network none` (no interface exists) and `--network host` (the host interface is already up), and capped at five seconds for bridge mode. Crash-loop restart cycles now run on schedule, and simple containers such as `sh -c sleep 5` start immediately after `cardinal run -d` returns.

## 1.22.36 (2026-08-11)

### Runtime fixes

- Detached container processes that become zombies (defunct, reparented to systemd after the CLI exits) are now detected as dead: process liveness reads `/proc/<pid>/stat` and treats `Z`/`X` states as exited. A plain `/proc/<pid>` existence check counted zombies as alive, stalling exit detection — containers stuck on `running`, cgroup/network cleanup not running, and crash-loop restarts almost never firing.
- `cardinal rm` writes a tombstone marker as its first action, so a supervisor automatic restart racing a slow removal can no longer resurrect a container mid-delete.
- The supervisor re-loads fresh container state before an automatic restart and skips containers being removed; `cardinal start` aborts cleanly (killing spawned processes and releasing the DNS record) if the state file vanished or a removal is in progress.

## 1.22.35 (2026-08-11)

### Documentation

- Added complete English and Russian CLI command references with syntax prefixes, positional arguments, aliases, every user-facing option, and internal-command notes.
- Added separate English and Russian practical command examples for applications, databases, bots, Minecraft, restart recovery, volumes, backups, Compose, registries, clusters, services, and functions.
- Linked the new references and examples from the README and documentation index.

### CI and release workflow

- CI runs Go tests only on the native amd64 runner; arm64 is cross-compiled without executing an incompatible test binary.
- The architecture matrix uses `fail-fast: false`, so one architecture cannot cancel the other job.
- Build & Release checks out the newly created version tag before building amd64, arm64, and armv6 artifacts.
- Build and manual release workflows are serialized to avoid concurrent version and tag updates.

### Added

- Added configurable automatic restart delays with `--restart-delay` / `cardinal set --restart-delay` (for example, `10s` or `1m`).
- Added a persistent systemd supervisor for detached `always` and `unless-stopped` containers; `on-failure` remains a foreground-process policy and is not adopted after the detached CLI exits.
- Added per-container scheduled backups with `cardinal backup enable/disable/status`, configurable intervals, retention, safe destinations, and supervisor-based recovery after reboot.

## 1.22.30 (2026-08-11)

### Documentation

- Added complete bilingual CLI references and command examples.
- Updated documentation navigation and current release references.

## 1.22.28 (2026-08-11)

### Documentation

- Updated English and Russian guides with the current version, automatic backups, supervisor behavior, restart delays, protected bind mounts, log rotation, and CI/build workflow.

## 1.22.26 (2026-08-11)

### Documentation

- Added complete English and Russian running guides covering installation, image/tag syntax, protected bind mounts, `.env` files, Python bots, Java/Minecraft servers, logs, restart policies, updates, and troubleshooting.
- Updated the README, documentation index, command reference, storage notes, and version references.

### Runtime fixes documented

- Container cardinal stdout/stderr logs start fresh on every new container run instead of accumulating across restarts.
- OCI extraction accepts forward symlink targets and standard root entries while preserving traversal protection.
- Bind mounts reject protected host paths and require absolute container targets.

## 1.22.7 (2026-08-06)

- Isolated tests with `CARDINAL_DATA_DIR` and made JSON state writes atomic.
- Secured API defaults, Bearer authentication, and image metadata storage.
- Standardized Go 1.25 across CI and release tooling.

## 1.22.4 (2026-07-14)

- CI auto-bump after install script fix
- Single `VERSION` source of truth (removed `cmd/VERSION`)
- Removed `//go:embed VERSION`, injected via `-X cardinal/cmd.version` only

## 1.22.3 (2026-07-14)

- CI auto-bump after release.yml fix

## 1.22.2 (2026-07-14)

- CI auto-bump

## 1.22.1 (2026-07-14)

- CI auto-bump after first 1.22.0 release

## 1.22.0 (2026-07-14)

### Features
- **Cluster orchestration**: `cardinal cluster init/join/leave/info/ls/node` — multi-node container orchestration
- **Services**: `cardinal service create/ls/rm/scale/update` — replicated services with rolling updates
- **FaaS / Serverless**: `cardinal fn deploy/ls/rm/call` — auto-scaling serverless functions with scale-to-zero
- **Blueprints**: `cardinal blueprint list/info/install` + `blueprint repo add/remove/list` — pre-configured templates
- **Docker-compatible REST API**: `cardinal serve [-d] [--token]` — works with Portainer, VS Code Dev Containers
- **Compose secrets & configs**: Docker-style secret/config injection via `cardinal.toml` / `compose.yaml`
- **Container events**: `cardinal events [--since <time>]` — real-time lifecycle event streaming
- **Dynamic port management**: `cardinal port add/rm` — hot-add/remove port mappings without restart
- **Container FS browser**: `cardinal fs ls/cat/tree/find` — browse stopped/running container filesystems
- **Healthchecks**: `--healthcheck-*` flags — auto-restart on failure
- **Startup scripts**: `--startup` flag with `@file` support, `CARDINAL_*` env vars injection
- **Named volumes**: `cardinal volume create/ls/rm/inspect/prune`
- **Container export/import**: `cardinal export/import` — save and load container images
- **Registry auth**: `cardinal login/logout` — authenticated registry access + `cardinal push`

### Improvements
- Rootless mode support (`internal/container/rootless.go`)
- DNS service discovery for cluster (UDP 5353)
- Systemd bootstrap auto-install on `cardinal run --restart always`
- `cardinal set` now supports `--memory`, `--cpus`, `--disk`, `--restart`, `--workdir`, `-e`, `--entrypoint`, `--user`, `--readonly`, `--no-new-privs`, `-h`, `--network`
- `cardinal up --generate` — generate cardinal.toml from existing containers
- Multi-arch image resolution (`--platform`)
- cgroups v2 resource limits for CPU, memory, disk

## 1.21.0 (2026-07-01)

### Features
- Container commit: `cardinal commit <container> <image>:<tag>`
- `cardinal build` — Dockerfile builder with `--no-cache`, `--build-arg`, multi-stage support
- `cardinal system prune` — cleanup unused containers and images
- `cardinal stop --all` — stop all running containers
- `cardinal exec -i/-t` flags properly handled
- `cardinal console` — auto-detect shell inside container
- Improved attach with Unix socket (Ctrl+C safe)

## 1.20.0 (2026-06-20)

### Features
- Dynamic port management: `cardinal port add/rm` — hot-add/remove iptables DNAT rules
- Russian (ru) documentation mirror
- `--ulimit` support in run flags

### Bug Fixes
- Fixed overlay mount ordering for disk limits
- Fixed `cardinal exec` TTY handling

## 1.19.0 (2026-06-17)

### Features
- Container FS browser: `cardinal fs ls/cat/tree/find`
- `--healthcheck-*` flags with auto-restart on failure
- `--startup` flag with `@file` inline script support
- `CARDINAL_*` environment variables injected for startup scripts

## 1.18.0 (2026-06-15)

### Features
- `cardinal events` — real-time container event streaming
- `cardinal volume create/ls/rm/inspect/prune` — named volume management
- `cardinal export/import` — save/load images as tar.gz
- `cardinal login/logout` — authenticated registry access
- Multi-arch image resolution with `--platform` flag

### Security
- Rootless mode support (experimental)
- No-new-privs flag support

## 1.17.0 (2026-06-24)

### Code Quality
- **Dead code removed**: `internal/container/rcon.go` — неиспользуемый RCON протокол
- **State tests**: 12 unit-тестов для `internal/state` (пути, JSON, FileExists)

### Bug Fixes
- **cardinal-wings**: Исправлен баг валидации container ID — `/` блокировал все action-запросы (start/stop/restart)

## 1.15.0 (2026-06-13)

### Security
- **pivot_root** вместо chroot — исправлен container escape vector
- Build tags (`//go:build linux`) добавлены на Linux-only файлы

### Bug Fixes
- **Disk limit**: исправлен path mismatch — overlay монтируется в правильную merged директорию
- **Race condition StoppedByUser**: `sync.Map` для атомарного флага между Stop() и monitor goroutine
- **Error swallowing**: логируются ошибки cgroup, mount, network, kill, overlay
- **Context/Timeout**: volume и network команды теперь с 30s timeout через `exec.CommandContext`
- **RCON**: мёртвый код помечен комментарием

### Features
- **`cardinal stop --all`**: остановка всех запущенных контейнеров
- **`cardinal exec -i/-t`**: флаги парсятся и применяются корректно
- **`cardinal console`**: TTY handling через ExecOpts

### Maintenance
- Debian control version синхронизирована с VERSION (1.15.0)

## 1.14.0 (Previous)
- DiskLimit support + loop device quota enforcement
- cardinal run --disk human-readable format
- Fix overlay mount ordering
- Multi-arch image resolution

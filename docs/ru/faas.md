<!-- cardinal-version:start -->
**Documentation version:** `2.0.4`
**Project release:** `v2.0.4`
<!-- cardinal-version:end -->

# FaaS / Serverless

cardinal может запускать контейнерные образы как serverless-функции
с авто-масштабированием и scale-to-zero.

## Быстрый старт

```bash
# Развернуть функцию
cardinal fn deploy \
  --name hello \
  --port 8080 \
  --timeout 30 \
  --idle 300 \
  --warm 1 \
  -e GREETING=world \
  ghcr.io/myorg/hello-func:latest

# Вызвать
cardinal fn call hello --data '{"name": "cardinal"}'

# Список функций
cardinal fn ls

# Удалить
cardinal fn rm hello
```

## Команды

### `cardinal fn deploy`

Развернуть контейнерный образ как serverless-функцию.

| Флаг | По умолч. | Описание |
|---|---|---|
| `--name` | — | Имя функции (обязательно) |
| `--port` | `8080` | Порт, который слушает функция внутри контейнера |
| `--handler` | `/handler` | Путь к бинарнику/скрипту внутри контейнера |
| `--timeout` | `30` | Макс. время выполнения одного вызова (сек) |
| `--idle` | `300` | Секунд бездействия до scale-to-zero |
| `--warm` | `0` | Количество всегда горячих реплик |
| `--memory` | — | Лимит памяти на вызов (`128m`, `512m`, `1g`) |
| `--cpus` | — | Лимит CPU на вызов |
| `-e, --env` | — | Переменные окружения (можно повторять) |

```
cardinal fn deploy --name hello --port 8080 --timeout 30 --idle 300 myfunc
cardinal fn deploy --name worker --timeout 120 --memory 1g --cpus 2 worker-image
cardinal fn deploy --name api --port 3000 --warm 3 api-image
```

### `cardinal fn ls`

Список всех развёрнутых функций.

```
NAME   IMAGE       PORT  TIMEOUT  IDLE   WARM  INVOKES
hello  myfunc:lat  8080  30s      300s   1     42
worker worker-img  0     120s     300s   0     7
```

### `cardinal fn rm <имя> [<имя>...]`

Удалить функцию (сначала останавливает все активные контейнеры).

```
cardinal fn rm hello
```

### `cardinal fn call <имя> [--data <данные>]`

Вызвать функцию. Данные можно передать inline или через stdin.

```
cardinal fn call hello --data '{"key":"value"}'
echo '{"key":"value"}' | cardinal fn call hello
```

## Жизненный цикл авто-масштабирования

```
  бездействие     вызов       бездействие > timeout    бездействие
  ───────► [scale-up] ──────► [работа] ────────────► [scale-to-zero]
             │                                         │
             └── warm реплики пропускают этот шаг◄──────┘
```

1. **Warm**: если `--warm N` указан, N контейнеров всегда запущены
2. **Cold start**: при первом запросе контейнер запускается с нуля
3. **Scale-to-zero**: после `--idle` секунд без запросов контейнеры останавливаются
4. **Повторный вызов**: запускает новый scale-up, если нет warm реплик

## Примеры использования

### HTTP API

```bash
cardinal fn deploy --name users-api --port 3000 --warm 3 myorg/users-api
```

Всегда 3 горячих экземпляра; масштабируется при пиках трафика,
возвращается к 3 после бездействия.

### Worker / Очередь

```bash
cardinal fn deploy --name email-worker --timeout 120 --memory 1g myorg/email-worker
```

Cold-start при каждом вызове. Scale-to-zero между сообщениями.

### Webhook handler

```bash
cardinal fn deploy --name github-webhook --port 9000 --timeout 15 --idle 60 myorg/webhook
```

Быстрый вызов (15s timeout), scale-to-zero через минуту бездействия.

## Контракт runtime функции

Контейнер должен:
1. Слушать HTTP порт (`--port`)
2. Принимать POST запросы
3. Читать тело запроса как входные данные вызова
4. Возвращать тело ответа как результат функции

Переменные окружения, доступные в runtime:

| Переменная | Описание |
|---|---|
| `CARDINAL_FN_NAME` | Имя функции |
| `CARDINAL_FN_TIMEOUT` | Макс. время выполнения |
| `CARDINAL_INVOCATION_ID` | Уникальный ID вызова |

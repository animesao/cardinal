<!-- dck-version:start -->
**Documentation version:** `1.24.16`
**Project release:** `v1.24.16`
<!-- dck-version:end -->

# Кластерная оркестрация

dck поддерживает многопоточную кластеризацию с управлением сервисами,
DNS-обнаружением сервисов и rolling updates — всё в одном 5 MB бинарнике
без внешних зависимостей.

## Архитектура

```
┌──────────────────────────────────────────┐
│              dck кластер                  │
│                                          │
│  ┌──────────┐     ┌──────────┐          │
│  │  Узел 1   │     │  Узел 2   │          │
│  │ (лидер)  │◄───►│ (worker)  │          │
│  │          │     │          │          │
│  │ dck run  │     │ dck run  │          │
│  │контейнеры│     │контейнеры│          │
│  └──────────┘     └──────────┘          │
│         ▲                ▲               │
│         └──────┬─────────┘               │
│                │                         │
│         ┌──────┴──────┐                  │
│         │  HTTP gossip │  (heartbeat)     │
│         └─────────────┘                  │
└──────────────────────────────────────────┘
```

- **Лидер**: первый узел в кластере, координирует состояние
- **Worker**: присоединившийся узел, запускает контейнеры и отправляет heartbeat
- **Gossip**: HTTP-репликация состояния каждые 5 секунд

## Команды кластера

### `dck cluster init`

Инициализация нового кластера. Первый узел становится лидером.
`--serve` сразу запускает API-сервер для приёма запросов на реплики.

```
DCK_TOKEN='<strong-random-token>' dck cluster init --name prod --bind 0.0.0.0 --port 7946 --api-port 2375 --serve --token '<strong-random-token>'
```

Вывод:
```
Initialized cluster prod (a1b2c3d4e5f6)
  Node ID: f6e5d4c3b2a1
  Node name: node-01
  Bind address: 0.0.0.0:7946
  API Port: 2375 (for remote replica requests)
Starting API server on 0.0.0.0:2375 (требуется Bearer-токен)...
```

### `dck cluster join <узел>`

Присоединиться к существующему кластеру. `--serve` запускает API-сервер.

```
DCK_TOKEN='<strong-random-token>' dck cluster join 10.0.0.1:7946 --bind 0.0.0.0 --port 2375 --serve --token '<strong-random-token>'
```

Присоединяющийся узел:
1. Отправляет свою информацию пиру
2. Получает полное состояние кластера (узлы + сервисы)
3. Начинает отправлять heartbeat каждые 5 секунд

### `dck cluster leave`

Выйти из кластера. Уведомляет все узлы и сбрасывает локальное состояние.

```
dck cluster leave
```

### `dck cluster serve`

Запуск HTTP API-сервера для приёма запросов на реплики от других нод кластера.
Нужен на каждой ноде, которая должна запускать контейнеры по заданию планировщика.

```
dck cluster serve -p 2375 -H 0.0.0.0 --token '<strong-random-token>'
```

API-сервер обрабатывает:
- `POST /cluster/replicas` — создать и запустить реплику (контейнер)
- `DELETE /cluster/replicas/<id>` — остановить и удалить контейнер
- `GET /cluster/containers` — список контейнеров на этой ноде

### `dck cluster ls`

Список всех узлов кластера.

```
ID        NAME      ADDRESS        ROLE    STATE     LAST SEEN
a1b2c3d4  node-01   10.0.0.1:2375  leader  active    15:04:05
f6e5d4c3  node-02   10.0.0.2:2375  worker  active    15:04:03
e7f8a9b0  node-03   10.0.0.3:2375  worker  active    15:04:01
```

## Управление сервисами

### `dck service create`

Создать сервис с репликами, распределёнными по кластеру.

```
dck service create \
  --name web \
  --replicas 3 \
  --port 80:80 \
  --env NODE_ENV=production \
  nginx:alpine
```

### `dck service ls`

Список всех сервисов.

```
NAME   IMAGE         REPLICAS  PORTS         CREATED
web    nginx:alpine  3         80->80/tcp    2026-07-03 15:00:00
api    myapi:latest  2         3000->3000/tcp 2026-07-03 14:00:00
```

### `dck service scale <имя> <реплики>`

Изменить количество реплик сервиса.

```
dck service scale web 5    # увеличить до 5
dck service scale api 0    # уменьшить до 0 (остановить все)
```

### `dck service update <имя> --image <новый-образ>`

Выполнить rolling update. Планировщик постепенно заменяет реплики
(управляется `UpdateConfig.Parallelism` и `UpdateConfig.Delay`).

```
dck service update web --image nginx:1.25
```

### `dck service rm <имя>`

Удалить сервис и все его реплики.

```
dck service rm web
```

## DNS-обнаружение сервисов

dck включает встроенный DNS-сервер, который преобразует имена сервисов в IP контейнеров.

Формат: `<имя>.svc.cluster.local`

```
# Изнутри контейнера:
curl http://web.svc.cluster.local:80
ping api.svc.cluster.local
```

DNS-сервер слушает UDP порт 5353 и обеспечивает round-robin балансировку
между всеми репликами сервиса.

```
web.svc.cluster.local → 10.0.0.10, 10.0.0.11, 10.0.0.12  (round-robin)
```

## Как работает планировщик

1. **Reconcile loop** запускается каждые 10 секунд
2. Сравнивает желаемое количество реплик с фактическим
3. Scale-up: выбирает наименее загруженный узел (наибольшая доступная память)
4. Scale-down: удаляет самые новые реплики
5. Healthchecks из runtime используются для самовосстановления

## Хранение состояния

Каждый узел хранит состояние локально:

```
~/.dck/
  cluster/
    cluster.json         # конфигурация кластера + список узлов
  services/
    services.json        # определения сервисов
    web/                 # состояние реплик сервиса
      replica-1.json
      replica-2.json
```

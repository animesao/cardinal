<!-- cardinal-version:start -->
**Documentation version:** `2.0.6`
**Project release:** `v2.0.6`
<!-- cardinal-version:end -->

# Cluster Orchestration

cardinal supports multi-node clustering with service management, DNS-based service discovery,
and rolling updates — all in a single 5 MB binary with no external dependencies.

## Architecture

```
┌──────────────────────────────────────────┐
│              cardinal cluster                  │
│                                          │
│  ┌──────────┐     ┌──────────┐          │
│  │  Node 1   │     │  Node 2   │          │
│  │ (leader)  │◄───►│ (worker)  │          │
│  │           │     │           │          │
│  │ cardinal run   │     │ cardinal run   │          │
│  │ containers│     │ containers│          │
│  └──────────┘     └──────────┘          │
│         ▲                ▲               │
│         └──────┬─────────┘               │
│                │                         │
│         ┌──────┴──────┐                  │
│         │  HTTP gossip │  (heartbeat)     │
│         └─────────────┘                  │
└──────────────────────────────────────────┘
```

- **Leader**: first node in the cluster, coordinates state
- **Worker**: joined node, runs containers and reports heartbeats
- **Gossip**: HTTP-based state replication every 5 seconds

## Cluster commands

### `cardinal cluster init`

Initialize a new cluster. The first node becomes the leader.
Use `--serve` to also start the API server for accepting remote replica requests.

```
CARDINAL_TOKEN='<strong-random-token>' cardinal cluster init --name prod --bind 0.0.0.0 --port 7946 --api-port 2375 --serve --token '<strong-random-token>'
```

Output:
```
Initialized cluster prod (a1b2c3d4e5f6)
  Node ID: f6e5d4c3b2a1
  Node name: node-01
  Bind address: 0.0.0.0:7946
  API Port: 2375 (for remote replica requests)
Starting API server on 0.0.0.0:2375 (Bearer token required)...
```

### `cardinal cluster join <peer>`

Join an existing cluster. Use `--serve` to also start the API server.

```
CARDINAL_TOKEN='<strong-random-token>' cardinal cluster join 10.0.0.1:7946 --bind 0.0.0.0 --port 2375 --serve --token '<strong-random-token>'
```

The joining node:
1. Sends its info to the peer
2. Receives full cluster state (nodes + services)
3. Starts heartbeating every 5 seconds

### `cardinal cluster leave`

Gracefully leave the cluster. Notifies all peers and resets local state.

```
cardinal cluster leave
```

### `cardinal cluster serve`

Start the HTTP API server that accepts replica requests from other cluster nodes.
Required on each node that should run containers scheduled by the cluster.

```
cardinal cluster serve -p 2375 -H 0.0.0.0 --token '<strong-random-token>'
```

The API server handles:
- `POST /cluster/replicas` — create and start a container replica
- `DELETE /cluster/replicas/<id>` — stop and remove a container
- `GET /cluster/containers` — list containers on this node
- Built-in Docker API compatibility (Portainer etc.)

### `cardinal cluster ls`

List all nodes in the cluster.

```
ID        NAME      ADDRESS        ROLE    STATE     LAST SEEN
a1b2c3d4  node-01   10.0.0.1:2375  leader  active    15:04:05
f6e5d4c3  node-02   10.0.0.2:2375  worker  active    15:04:03
e7f8a9b0  node-03   10.0.0.3:2375  worker  active    15:04:01
```

## Service management

### `cardinal service create`

Create a service with replicas spread across the cluster.

```
cardinal service create \
  --name web \
  --replicas 3 \
  --port 80:80 \
  --env NODE_ENV=production \
  nginx:alpine
```

### `cardinal service ls`

List all services.

```
NAME   IMAGE         REPLICAS  PORTS         CREATED
web    nginx:alpine  3         80->80/tcp    2026-07-03 15:00:00
api    myapi:latest  2         3000->3000/tcp 2026-07-03 14:00:00
```

### `cardinal service scale <name> <replicas>`

Scale a service up or down.

```
cardinal service scale web 5    # scale up to 5
cardinal service scale api 0    # scale down to 0 (stop all)
```

### `cardinal service update <name> --image <new-image>`

Perform a rolling update. The scheduler gradually replaces replicas
(controlled by `UpdateConfig.Parallelism` and `UpdateConfig.Delay`).

```
cardinal service update web --image nginx:1.25
```

### `cardinal service rm <name>`

Remove a service and all its replicas.

```
cardinal service rm web
```

## DNS-based service discovery

cardinal includes a built-in DNS server that resolves service names to container IPs.

Format: `<name>.svc.cluster.local`

```
# From inside a container:
curl http://web.svc.cluster.local:80
ping api.svc.cluster.local
```

The DNS server listens on UDP port 5353 by default and provides round-robin
load balancing across all replicas of a service.

```
web.svc.cluster.local → 10.0.0.10, 10.0.0.11, 10.0.0.12  (round-robin)
```

## How scheduling works

1. **Reconcile loop** runs periodically (every 10 seconds)
2. Compares desired replica count vs actual running count
3. Scale-up: selects the least loaded node (highest available memory)
4. Scale-down: removes the most recent replicas first
5. Health checks from the runtime are used for self-healing

## State storage

Each node stores cluster state locally:

```
~/.cardinal/
  cluster/
    cluster.json         # cluster config + node list
  services/
    services.json        # service definitions
    web/                 # replica state per service
      replica-1.json
      replica-2.json
```

<!-- cardinal-version:start -->
**Documentation version:** `2.0.3`
**Project release:** `v2.0.3`
<!-- cardinal-version:end -->

# Deploying Websites with cardinal

Complete examples of deploying websites with various stacks — static sites,
Python, Node.js, PHP, Java, and full-stack apps with databases.

All examples assume you have cardinal installed on a Linux server with a public IP.

> **Resource limits:** Add `--memory 512m --cpus 0.5 --disk 2G` to any `cardinal run` command
> to limit RAM, CPU, and disk usage. Prevents one container from eating all VPS resources.
> Adjust values per app: bots → 256m/0.25, sites → 512m/0.5, databases → 1g/1.

---

## Table of Contents

- [Static Site (nginx)](#static-site-nginx)
- [Static Site with HTTPS (Let's Encrypt)](#static-site-with-https)
- [File Operations](#file-operations)
- [Python Flask App](#python-flask-app)
- [Python FastAPI](#python-fastapi)
- [Python Django](#python-django)
- [Node.js Express](#nodejs-express)
- [Node.js Next.js](#nodejs-nextjs)
- [PHP + Nginx](#php--nginx)
- [Java Spring Boot](#java-spring-boot)
- [Go HTTP Server](#go-http-server)
- [Full-Stack with Database](#full-stack-with-database)
- [Multi-Container with Compose](#multi-container-with-compose)
- [Minecraft Server](#minecraft-server)
- [Telegram Bot](#telegram-bot)
- [Discord Bot](#discord-bot)
- [Node Site with Auto-Start](#node-site-with-auto-start)
- [Databases](#databases)
  - [PostgreSQL](#postgresql)
  - [MySQL](#mysql)
  - [phpMyAdmin](#phpmyadmin)
- [Production Checklist](#production-checklist)

---

## Static Site (nginx)

The simplest — serve HTML/CSS/JS files with nginx.

```bash
# Create site directory
mkdir -p /data/www/mysite
echo "<h1>Hello from cardinal!</h1>" > /data/www/mysite/index.html

# Run nginx with mounted files (limit: 256MB RAM, 0.5 CPU, 1GB disk)
cardinal run -d --restart always \
  -n mysite -p 80:80 \
  -v /data/www/mysite:/usr/share/nginx/html \
  --memory 256m --cpus 0.5 --disk 1G \
  nginx:alpine

# Test
curl http://localhost:80
```

### Custom nginx config

```bash
mkdir -p /data/www/mysite /data/nginx-conf

# Create nginx config
cat > /data/nginx-conf/default.conf << 'EOF'
server {
    listen 80;
    server_name mysite.example.com;
    root /usr/share/nginx/html;
    index index.html;
    location / { try_files $uri $uri/ =404; }
    location /api/ { proxy_pass http://backend:3000; }
}
EOF

cardinal run -d --restart always \
  -n mysite -p 80:80 \
  -v /data/www/mysite:/usr/share/nginx/html \
  -v /data/nginx-conf:/etc/nginx/conf.d \
  nginx:alpine
```

---

## Static Site with HTTPS

Using Let's Encrypt / certbot with auto-renewal.

### Option 1: Manual certs

```bash
# Install certbot on host
apt-get install -y certbot

# Get certificate
certbot certonly --standalone -d mysite.example.com

# Create nginx config with SSL
mkdir -p /data/nginx-ssl
cat > /data/nginx-ssl/default.conf << 'EOF'
server {
    listen 80;
    server_name mysite.example.com;
    return 301 https://$host$request_uri;
}
server {
    listen 443 ssl http2;
    server_name mysite.example.com;
    ssl_certificate /etc/letsencrypt/live/mysite.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/mysite.example.com/privkey.pem;
    root /usr/share/nginx/html;
    index index.html;
}
EOF

# Run nginx
cardinal run -d --restart always \
  -n mysite -p 80:80 -p 443:443 \
  -v /data/www/mysite:/usr/share/nginx/html \
  -v /data/nginx-ssl:/etc/nginx/conf.d \
  -v /data/letsencrypt:/etc/letsencrypt \
  nginx:alpine
```

### Option 2: Self-signed (for testing)

```bash
mkdir -p /data/ssl /data/nginx-conf/ssl

# Generate self-signed cert
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /data/ssl/server.key \
  -out /data/ssl/server.crt \
  -subj "/CN=localhost"

# Copy ssl into config dir
cp /data/ssl/server.crt /data/nginx-conf/ssl/
cp /data/ssl/server.key /data/nginx-conf/ssl/

# Nginx config
cat > /data/nginx-conf/default.conf << 'EOF'
server {
    listen 443 ssl http2;
    server_name localhost;
    ssl_certificate /etc/nginx/conf.d/ssl/server.crt;
    ssl_certificate_key /etc/nginx/conf.d/ssl/server.key;
    root /usr/share/nginx/html;
    index index.html;
}
EOF

# Run
cardinal run -d --restart always \
  -n mysite -p 443:443 \
  -v /data/www/mysite:/usr/share/nginx/html \
  -v /data/nginx-conf:/etc/nginx/conf.d \
  nginx:alpine
```

---

## File Operations

Copy files between your host machine and containers using `cardinal cp`:

```bash
# Copy from host into container
cardinal cp ./index.html web:/usr/share/nginx/html/
cardinal cp ./app.py flask-app:/app/
cardinal cp ./config.yml mycontainer:/etc/app/config.yml
cardinal cp ./mybot.py discord-bot:/bot/

# Copy from container to host
cardinal cp web:/etc/nginx/nginx.conf ./nginx.conf
cardinal cp flask-app:/app/app.py ./backup-app.py

# Copy entire directories
cardinal cp ./static/ web:/usr/share/nginx/html/static/
cardinal cp django-app:/app/media/ ./media-backup/

# Copy with container ID instead of name
cardinal cp ./file.txt abc123def456:/tmp/
```

This is useful for:
- Uploading website files (HTML, CSS, JS) into a running web server
- Deploying bot code without rebuilding the image
- Backing up configuration or data from containers
- Injecting config files (nginx, app configs, etc.)

You can also use bind mounts (`-v`) for persistent file sharing — changes on the host are immediately visible inside the container:

```bash
cardinal run -d -n web -p 80:80 \
  -v /data/www/mysite:/usr/share/nginx/html \
  nginx:alpine

# Edit files on the host — nginx serves them instantly
echo "<h1>Updated!</h1>" > /data/www/mysite/index.html
```

---

## Python Flask App

### Basic Flask with Gunicorn

```bash
# Create app
mkdir -p /data/flask-app
cd /data/flask-app

cat > app.py << 'EOF'
from flask import Flask
app = Flask(__name__)

@app.route('/')
def home():
    return '<h1>Hello from Flask on cardinal!</h1>'

@app.route('/api')
def api():
    return {'message': 'Hello from cardinal!'}

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
EOF

cat > requirements.txt << 'EOF'
flask==3.0.0
gunicorn==22.0.0
EOF

# Run with gunicorn via startup script
cardinal run -d --restart always \
  -n flask-app -p 5000:5000 \
  -v /data/flask-app:/app \
  --workdir /app \
  --startup "pip install -r /app/requirements.txt && gunicorn -w 4 -b 0.0.0.0:5000 app:app" \
  python:3.11-slim

# Test
curl http://localhost:5000
```

### Flask with nginx reverse proxy

```bash
# Create nginx config
mkdir -p /data/flask-nginx
cat > /data/flask-nginx/default.conf << 'EOF'
server {
    listen 80;
    server_name example.com;

    location / {
        proxy_pass http://127.0.0.1:5000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    location /static/ {
        alias /data/www/flask-app/static/;
    }
}
EOF

# Run flask app (no port needed - access via nginx)
cardinal run -d --restart always \
  -n flask-backend \
  -v /data/flask-app:/app \
  --workdir /app \
  --startup "pip install -r /app/requirements.txt && gunicorn -w 4 -b 0.0.0.0:5000 app:app" \
  python:3.11-slim

# Run nginx pointing to flask container by IP
cardinal run -d --restart always \
  -n flask-web -p 80:80 \
  -v /data/flask-nginx:/etc/nginx/conf.d \
  nginx:alpine

# Find flask IP
cardinal inspect flask-backend | grep IP
# Then update nginx config with flask IP (10.0.2.x)
```

---

## Python FastAPI

```bash
mkdir -p /data/fastapi-app
cd /data/fastapi-app

cat > main.py << 'EOF'
from fastapi import FastAPI
app = FastAPI()

@app.get("/")
async def root():
    return {"message": "Hello from FastAPI on cardinal!"}

@app.get("/items/{item_id}")
async def read_item(item_id: int):
    return {"item_id": item_id}
EOF

cat > requirements.txt << 'EOF'
fastapi==0.109.0
uvicorn==0.27.0
EOF

cardinal run -d --restart always \
  -n fastapi-app -p 8000:8000 \
  -v /data/fastapi-app:/app \
  --workdir /app \
  --startup "pip install -r /app/requirements.txt && uvicorn main:app --host 0.0.0.0 --port 8000" \
  python:3.11-slim

curl http://localhost:8000
curl http://localhost:8000/docs   # Swagger UI
```

---

## Python Django

```bash
mkdir -p /data/django-app
cd /data/django-app

# Create requirements
cat > requirements.txt << 'EOF'
django==5.0.0
gunicorn==22.0.0
psycopg2-binary==2.9.9
EOF

# Create startup script
cat > start.sh << 'SCRIPT'
#!/bin/sh
set -e
cd /app
pip install -r requirements.txt
python manage.py migrate
python manage.py collectstatic --noinput
gunicorn myproject.wsgi:application -w 4 -b 0.0.0.0:8000
SCRIPT

# Create Django project (or copy existing)
cardinal run --rm \
  -v /data/django-app:/app \
  --workdir /app \
  python:3.11-slim sh -c "pip install django && django-admin startproject myproject ."

# Run with DB
cardinal run -d --restart always \
  -n django-db -p 5432:5432 \
  -v django-pgdata:/var/lib/postgresql/data \
  -e POSTGRES_DB=django \
  -e POSTGRES_USER=django \
  -e POSTGRES_PASSWORD=secret \
  postgres:16

# Wait for DB, then run Django
cardinal run -d --restart always \
  -n django-app -p 8000:8000 \
  -v /data/django-app:/app \
  --workdir /app \
  -e DB_HOST=10.0.2.1 \
  -e DB_NAME=django \
  -e DB_USER=django \
  -e DB_PASSWORD=secret \
  --startup @/app/start.sh \
  python:3.11-slim
```

---

## Node.js Express

```bash
mkdir -p /data/express-app
cd /data/express-app

cat > index.js << 'EOF'
const express = require('express');
const app = express();
const port = 3000;

app.get('/', (req, res) => {
  res.send('<h1>Hello from Express on cardinal!</h1>');
});

app.get('/api', (req, res) => {
  res.json({ message: 'Hello from cardinal!' });
});

app.listen(port, () => {
  console.log(`App listening on port ${port}`);
});
EOF

cat > package.json << 'EOF'
{
  "name": "express-app",
  "scripts": { "start": "node index.js" },
  "dependencies": { "express": "^4.18.2" }
}
EOF

cardinal run -d --restart always \
  -n express-app -p 3000:3000 \
  -v /data/express-app:/app \
  --workdir /app \
  --startup "npm install && npm start" \
  node:20

curl http://localhost:3000
```

---

## Node.js Next.js

```bash
mkdir -p /data/next-app
cd /data/next-app

# Create Next.js project
cat > package.json << 'EOF'
{
  "name": "next-app",
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start -p 3000"
  },
  "dependencies": {
    "next": "^14.0.0",
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  }
}
EOF

mkdir -p pages
cat > pages/index.js << 'EOF'
export default function Home() {
  return <h1>Hello from Next.js on cardinal!</h1>;
}
EOF

# Development mode (with hot reload)
cardinal run -d --restart always \
  -n next-dev -p 3000:3000 \
  -v /data/next-app:/app \
  --workdir /app \
  --startup "npm install && npm run dev" \
  node:20

# Production build
cardinal run --rm \
  -v /data/next-app:/app \
  --workdir /app \
  node:20 sh -c "npm install && npm run build"

cardinal run -d --restart always \
  -n next-prod -p 3000:3000 \
  -v /data/next-app:/app \
  --workdir /app \
  --startup "npm start" \
  node:20
```

---

## PHP + Nginx

```bash
mkdir -p /data/php-app
cd /data/php-app

cat > index.php << 'EOF'
<?php
echo '<h1>Hello from PHP on cardinal!</h1>';
echo '<p>PHP version: ' . phpversion() . '</p>';
EOF

# Create nginx config for PHP-FPM
mkdir -p /data/php-nginx
cat > /data/php-nginx/default.conf << 'EOF'
server {
    listen 80;
    root /var/www/html;
    index index.php;
    location ~ \.php$ {
        fastcgi_pass 127.0.0.1:9000;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
        include fastcgi_params;
    }
}
EOF

# Run PHP-FPM container
cardinal run -d --restart always \
  -n php-fpm \
  -v /data/php-app:/var/www/html \
  php:8.2-fpm

# Run nginx with PHP-FPM (find PHP container IP)
PHP_IP=$(cardinal inspect php-fpm | grep -o '"ip":"[^"]*"' | grep -o '[0-9.]*')
echo "PHP container IP: $PHP_IP"

# Update nginx config with actual PHP IP
sed -i "s/127.0.0.1:9000/$PHP_IP:9000/" /data/php-nginx/default.conf

cardinal run -d --restart always \
  -n php-web -p 80:80 \
  -v /data/php-app:/var/www/html \
  -v /data/php-nginx:/etc/nginx/conf.d \
  nginx:alpine
```

---

## Java Spring Boot

```bash
mkdir -p /data/spring-app
cd /data/spring-app

# Create a simple Spring Boot app (or use your JAR)
# For testing, create a simple JAR or use existing

cat > Dockerfile << 'EOF'
FROM eclipse-temurin:21-jdk AS build
WORKDIR /app
COPY . .
RUN apt-get update && apt-get install -y maven && mvn package -DskipTests

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /app/target/*.jar app.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "app.jar"]
EOF

# Build and run
cardinal build -t spring-app:v1 .
cardinal run -d --restart always \
  -n spring-app -p 8080:8080 \
  spring-app:v1

curl http://localhost:8080
```

### Quick Java test (no build needed)

```bash
cat > /data/spring-app/HttpServer.java << 'EOF'
import com.sun.net.httpserver.*;

public class HttpServer {
    public static void main(String[] args) throws Exception {
        com.sun.net.httpserver.HttpServer server = com.sun.net.httpserver.HttpServer.create(
            new InetSocketAddress(8080), 0);
        server.createContext("/", exchange -> {
            String resp = "<h1>Hello from Java on cardinal!</h1>";
            exchange.sendResponseHeaders(200, resp.length());
            exchange.getResponseBody().write(resp.getBytes());
            exchange.close();
        });
        server.setExecutor(null);
        server.start();
        System.out.println("Server started on port 8080");
        Thread.currentThread().join();
    }
}
EOF

cardinal run -d --restart always \
  -n java-app -p 8080:8080 \
  -v /data/spring-app:/app \
  --workdir /app \
  --startup "javac HttpServer.java && java HttpServer" \
  eclipse-temurin:21-jdk
```

---

## Go HTTP Server

```bash
mkdir -p /data/go-app
cd /data/go-app

cat > main.go << 'EOF'
package main

import (
    "fmt"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "<h1>Hello from Go on cardinal!</h1>")
    })
    http.ListenAndServe(":8080", nil)
}
EOF

# Multi-stage build with cardinal
cat > Dockerfile << 'EOF'
FROM golang:1.22 AS build
WORKDIR /app
COPY main.go .
RUN CGO_ENABLED=0 go build -o server .

FROM alpine:latest
COPY --from=build /app/server /
EXPOSE 8080
CMD ["/server"]
EOF

cardinal build -t go-app:v1 .
cardinal run -d --restart always \
  -n go-app -p 8080:8080 \
  go-app:v1

curl http://localhost:8080
```

---

## Full-Stack with Database

### Flask + PostgreSQL

```bash
# 1. PostgreSQL
mkdir -p /data/postgres-data

cardinal run -d --restart always \
  -n pg -p 5432:5432 \
  -v /data/postgres-data:/var/lib/postgresql/data \
  -e POSTGRES_DB=myapp \
  -e POSTGRES_USER=myapp \
  -e POSTGRES_PASSWORD=secret \
  postgres:16

# 2. Flask app
mkdir -p /data/flask-fullstack
cd /data/flask-fullstack

cat > app.py << 'EOF'
from flask import Flask, jsonify
import psycopg2, os

app = Flask(__name__)

def get_db():
    return psycopg2.connect(
        host=os.environ.get('DB_HOST', '10.0.2.1'),
        dbname=os.environ.get('DB_NAME', 'myapp'),
        user=os.environ.get('DB_USER', 'myapp'),
        password=os.environ.get('DB_PASS', 'secret')
    )

@app.route('/')
def home():
    return '<h1>Flask + PostgreSQL on cardinal!</h1>'

@app.route('/db')
def db_test():
    try:
        conn = get_db()
        cur = conn.cursor()
        cur.execute("SELECT version()")
        ver = cur.fetchone()
        cur.close()
        conn.close()
        return jsonify({'database': ver[0]})
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/users')
def users():
    conn = get_db()
    cur = conn.cursor()
    cur.execute("CREATE TABLE IF NOT EXISTS users (id SERIAL, name TEXT)")
    cur.execute("SELECT id, name FROM users")
    users = [{'id': r[0], 'name': r[1]} for r in cur.fetchall()]
    cur.close()
    conn.close()
    return jsonify(users)
EOF

cat > requirements.txt << 'EOF'
flask==3.0.0
gunicorn==22.0.0
psycopg2-binary==2.9.9
EOF

# Wait for PostgreSQL to be ready, then run Flask
cardinal run -d --restart always \
  -n flask-app -p 5000:5000 \
  -v /data/flask-fullstack:/app \
  --workdir /app \
  -e DB_HOST=10.0.2.1 \
  -e DB_NAME=myapp \
  -e DB_USER=myapp \
  -e DB_PASS=secret \
  --startup "pip install -r /app/requirements.txt && gunicorn -w 4 -b 0.0.0.0:5000 app:app" \
  python:3.11-slim

# Test
curl http://localhost:5000
curl http://localhost:5000/db
```

### Node.js + MySQL

```bash
# 1. MySQL
mkdir -p /data/mysql-data

cardinal run -d --restart always \
  -n mysql -p 3306:3306 \
  -v /data/mysql-data:/var/lib/mysql \
  -e MYSQL_ROOT_PASSWORD=rootpass \
  -e MYSQL_DATABASE=myapp \
  -e MYSQL_USER=myapp \
  -e MYSQL_PASSWORD=secret \
  mysql:8

# 2. Node.js app
mkdir -p /data/node-mysql
cd /data/node-mysql

cat > index.js << 'EOF'
const express = require('express');
const mysql = require('mysql2/promise');
const app = express();

const dbConfig = {
    host: process.env.DB_HOST || '10.0.2.1',
    user: process.env.DB_USER || 'myapp',
    password: process.env.DB_PASS || 'secret',
    database: process.env.DB_NAME || 'myapp'
};

app.get('/', async (req, res) => {
    res.send('<h1>Node.js + MySQL on cardinal!</h1>');
});

app.get('/db', async (req, res) => {
    try {
        const conn = await mysql.createConnection(dbConfig);
        const [rows] = await conn.execute('SELECT VERSION() AS ver');
        await conn.end();
        res.json({ database: rows[0].ver });
    } catch (e) {
        res.status(500).json({ error: e.message });
    }
});

app.listen(3000);
EOF

cat > requirements.txt << 'EOF'
express@^4.18.2
mysql2@^3.6.0
EOF

cardinal run -d --restart always \
  -n node-app -p 3000:3000 \
  -v /data/node-mysql:/app \
  --workdir /app \
  -e DB_HOST=10.0.2.1 \
  -e DB_NAME=myapp \
  -e DB_USER=myapp \
  -e DB_PASS=secret \
  --startup "npm install && node index.js" \
  node:20
```

### FastAPI + PostgreSQL + Redis

```bash
# 1. PostgreSQL
cardinal run -d --restart always \
  -n pg -p 5432:5432 \
  -v pgdata:/var/lib/postgresql/data \
  -e POSTGRES_DB=myapp \
  -e POSTGRES_USER=myapp \
  -e POSTGRES_PASSWORD=secret \
  postgres:16

# 2. Redis
cardinal run -d --restart always \
  -n redis -p 6379:6379 \
  redis:7

# 3. FastAPI app
mkdir -p /data/fastapi-stack
cd /data/fastapi-stack

cat > main.py << 'EOF'
from fastapi import FastAPI, HTTPException
import asyncpg, redis.asyncio as redis_, os

app = FastAPI()

@app.on_event("startup")
async def startup():
    app.db = await asyncpg.connect(
        host=os.environ.get('DB_HOST', '10.0.2.1'),
        database=os.environ.get('DB_NAME', 'myapp'),
        user=os.environ.get('DB_USER', 'myapp'),
        password=os.environ.get('DB_PASS', 'secret')
    )
    app.redis = redis_.Redis(host=os.environ.get('REDIS_HOST', '10.0.2.1'), port=6379, db=0)
    await app.db.execute('CREATE TABLE IF NOT EXISTS items (id SERIAL, name TEXT)')

@app.get('/')
async def root():
    return {'message': 'FastAPI + PostgreSQL + Redis on cardinal!'}

@app.get('/db')
async def db_check():
    ver = await app.db.fetchval('SELECT version()')
    return {'database': ver}

@app.get('/redis')
async def redis_check():
    await app.redis.set('test', 'ok')
    val = await app.redis.get('test')
    return {'redis': val.decode()}
EOF

cat > requirements.txt << 'EOF'
fastapi==0.109.0
uvicorn==0.27.0
asyncpg==0.29.0
redis==5.0.0
EOF

cardinal run -d --restart always \
  -n fastapi-app -p 8000:8000 \
  -v /data/fastapi-stack:/app \
  --workdir /app \
  -e DB_HOST=10.0.2.1 \
  -e DB_NAME=myapp \
  -e DB_USER=myapp \
  -e DB_PASS=secret \
  -e REDIS_HOST=10.0.2.1 \
  --startup "pip install -r /app/requirements.txt && uvicorn main:app --host 0.0.0.0 --port 8000" \
  python:3.11-slim
```

---

## Multi-Container with Compose

Define everything in `compose.yaml` and start with one command:

### WordPress + MySQL

```yaml
services:
  db:
    image: mysql:8
    restart: always
    volumes:
      - wp_data:/var/lib/mysql
    environment:
      MYSQL_ROOT_PASSWORD: rootpass
      MYSQL_DATABASE: wordpress
      MYSQL_USER: wpuser
      MYSQL_PASSWORD: wppass

  wordpress:
    image: wordpress:6-php8.2
    restart: always
    ports:
      - "8080:80"
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_USER: wpuser
      WORDPRESS_DB_PASSWORD: wppass
      WORDPRESS_DB_NAME: wordpress
    depends_on:
      db:
        condition: service_started
    volumes:
      - wp_uploads:/var/www/html/wp-content/uploads

volumes:
  wp_data:
  wp_uploads:
```

```bash
cardinal up
```

### Python API + PostgreSQL + Nginx

```yaml
services:
  db:
    image: postgres:16
    restart: always
    volumes:
      - pgdata:/var/lib/postgresql/data
    environment:
      POSTGRES_DB: myapp
      POSTGRES_USER: myapp
      POSTGRES_PASSWORD: secret
    healthcheck:
      test: pg_isready -U myapp
      interval: 5s
      retries: 10

  api:
    build: ./api
    restart: always
    ports:
      - "8000:8000"
    environment:
      DB_HOST: db
      DB_NAME: myapp
      DB_USER: myapp
      DB_PASS: secret
    depends_on:
      db:
        condition: service_healthy

  web:
    image: nginx:alpine
    restart: always
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/conf.d/default.conf
      - ./static:/usr/share/nginx/html
    depends_on:
      - api

volumes:
  pgdata:
```

```bash
cardinal up
```

---

## Minecraft Server

Run a Minecraft server inside a cardinal container.

### Quick start (itzg/minecraft-server)

The easiest way — use the pre-built `itzg/minecraft-server` image:

```bash
cardinal run -d --restart always \
  -n mc -p 25565:25565 \
  -v mc_data:/data \
  -e EULA=TRUE \
  -e TYPE=PAPER \
  -e VERSION=1.20.4 \
  -e MEMORY=2G \
  -e DIFFICULTY=normal \
  -e MAX_PLAYERS=20 \
  -e MOTD="Welcome to cardinal Minecraft!" \
  itzg/minecraft-server
```

Connect to your server at `your-server-ip:25565`.

### Custom server with --startup

Download and run any Paper/Spigot/vanilla server JAR:

```bash
cat > /data/mc/start.sh << 'EOF'
#!/bin/sh
set -e
SERVER_DIR="/data"
SERVER_JAR="server.jar"
MAX_MEM="${CARDINAL_MEMORY:-2G}"
echo "eula=true" > "$SERVER_DIR/eula.txt"
if [ ! -f "$SERVER_DIR/$SERVER_JAR" ]; then
  curl -fsSL -o "$SERVER_DIR/$SERVER_JAR" \
    "https://api.papermc.io/v2/projects/paper/versions/1.21/builds/100/downloads/paper-1.21-100.jar"
fi
exec java -Xms512M -Xmx$MAX_MEM -jar "$SERVER_DIR/$SERVER_JAR" nogui
EOF

cardinal run -d --restart always \
  -n mc-paper -p 25565:25565 \
  -v /data/mc:/data --memory 4G \
  --startup @/data/mc/start.sh \
  eclipse-temurin:21-jdk
```

### Server.properties via bind mount

```bash
mkdir -p /data/mc-config
cat > /data/mc-config/server.properties << 'EOF'
max-players=50
difficulty=hard
motd=A cardinal Minecraft Server
pvp=true
online-mode=true
EOF

cardinal run -d --restart always \
  -n mc -p 25565:25565 \
  -v /data/mc-config:/data \
  -e EULA=TRUE -e TYPE=PAPER -e VERSION=1.20.4 \
  -e MEMORY=4G \
  itzg/minecraft-server
```

### Modded (Forge/Fabric)

```bash
cardinal run -d --restart always \
  -n mc-forge -p 25565:25565 \
  -v mc_forge_data:/data \
  -e EULA=TRUE -e TYPE=FORGE -e VERSION=1.20.1 \
  -e FORGE_INSTALLER_URL=https://maven.minecraftforge.net/net/minecraftforge/forge/1.20.1-47.1.0/forge-1.20.1-47.1.0-installer.jar \
  itzg/minecraft-server
```

### Backup your world

```bash
# Copy world files from container to host
cardinal cp mc:/data/world ./world-backup

# Or backup the entire volume
tar -czf mc-backup.tar.gz /root/.cardinal/volumes/mc_data/
```

---

## Bots

Run bots (Telegram, Discord, Slack, etc.) in cardinal containers with persistent storage.

### Telegram Bot

```bash
mkdir -p /data/tg-bot
cd /data/tg-bot

cat > bot.py << 'EOF'
import os, logging
from telegram import Update
from telegram.ext import Application, CommandHandler

TOKEN = os.environ["BOT_TOKEN"]

async def start(update: Update, context):
    await update.message.reply_text("Hello from cardinal Telegram bot!")

async def ping(update: Update, context):
    await update.message.reply_text("pong")

def main():
    app = Application.builder().token(TOKEN).build()
    app.add_handler(CommandHandler("start", start))
    app.add_handler(CommandHandler("ping", ping))
    print("Bot started...")
    app.run_polling()

if __name__ == "__main__":
    main()
EOF

cat > requirements.txt << 'EOF'
python-telegram-bot==20.7
EOF

cat > start.sh << 'EOF'
#!/bin/sh
set -e
pip install --no-cache-dir --disable-pip-version-check -r /bot/requirements.txt
exec python /bot/bot.py
EOF

cardinal run -d --restart always \
  -n tg-bot \
  -v /data/tg-bot:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="YOUR_TELEGRAM_BOT_TOKEN" \
  --startup @/bot/start.sh \
  python:3.11-slim
```

### Discord Bot

```bash
mkdir -p /data/discord-bot
cd /data/discord-bot

cat > bot.py << 'EOF'
import os, discord
from discord.ext import commands

TOKEN = os.environ["BOT_TOKEN"]
intents = discord.Intents.default()
intents.message_content = True
bot = commands.Bot(command_prefix="!", intents=intents)

@bot.event
async def on_ready():
    print(f"Logged in as {bot.user}")

@bot.command()
async def ping(ctx):
    await ctx.send("pong")

@bot.command()
async def hello(ctx):
    await ctx.send(f"Hello {ctx.author.mention}!")

bot.run(TOKEN)
EOF

cat > requirements.txt << 'EOF'
discord.py==2.4.0
EOF

cat > start.sh << 'EOF'
#!/bin/sh
set -e
pip install --no-cache-dir --disable-pip-version-check -r /bot/requirements.txt
exec python /bot/bot.py
EOF

cardinal run -d --restart always \
  -n discord-bot \
  -v /data/discord-bot:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="YOUR_DISCORD_BOT_TOKEN" \
  --startup @/bot/start.sh \
  python:3.11-slim
```

### Bot with database

```bash
# 1. Start PostgreSQL (limit: 1GB RAM, 1 CPU, 10GB disk)
cardinal run -d --restart always \
  -n bot-db \
  -v bot_pgdata:/var/lib/postgresql/data \
  --memory 1g --cpus 1 --disk 10G \
  -e POSTGRES_DB=botdb \
  -e POSTGRES_USER=bot \
  -e POSTGRES_PASSWORD=secret \
  postgres:16

# 2. Bot with DB connection
mkdir -p /data/bot-db
cd /data/bot-db

cat > bot.py << 'EOF'
import os, discord, asyncpg
from discord.ext import commands

TOKEN = os.environ["BOT_TOKEN"]
DB_DSN = f"postgres://bot:secret@{os.environ['DB_HOST']}/botdb"

bot = commands.Bot(command_prefix="!", intents=discord.Intents.default())

@bot.event
async def on_ready():
    bot.db = await asyncpg.connect(DB_DSN)
    await bot.db.execute("CREATE TABLE IF NOT EXISTS users (id SERIAL, name TEXT)")
    print(f"Logged in as {bot.user}")

@bot.command()
async def ping(ctx):
    await ctx.send("pong")

bot.run(TOKEN)
EOF

cat > requirements.txt << 'EOF'
discord.py==2.4.0
asyncpg==0.29.0
EOF

cardinal run -d --restart always \
  -n db-bot \
  -v /data/bot-db:/bot \
  --workdir /bot \
  -e BOT_TOKEN="YOUR_TOKEN" \
  -e DB_HOST=10.0.2.1 \
  --startup "pip install -r /bot/requirements.txt && python /bot/bot.py" \
  python:3.11-slim
```

### Bot health monitoring

```bash
# Check bot logs
cardinal logs -f tg-bot

# Check if bot is running
cardinal ps | grep bot

# Restart bot if needed
cardinal restart discord-bot

# Auto-restart on failure (already set with --restart always)
cardinal run -d --restart always \
  --healthcheck-cmd "curl -f http://localhost || exit 1" \
  --healthcheck-interval 30 \
  --healthcheck-retries 3 \
  ...
```

---

## Databases

Run database servers inside cardinal containers with persistent storage.

### PostgreSQL

```bash
# Quick start
cardinal run -d --restart always \
  -n pg -p 5432:5432 \
  -v pgdata:/var/lib/postgresql/data \
  -e POSTGRES_DB=myapp \
  -e POSTGRES_USER=myapp \
  -e POSTGRES_PASSWORD=secret \
  postgres:16

# Connect from another container (host=10.0.2.1)
PGPASSWORD=secret psql -h 10.0.2.1 -U myapp -d myapp

# Import SQL dump
cat dump.sql | cardinal exec -i pg psql -U myapp -d myapp

# Backup
cardinal exec pg pg_dump -U myapp myapp > backup.sql

# Restore backup
cat backup.sql | cardinal exec -i pg psql -U myapp -d myapp
```

### MySQL

```bash
# Quick start
cardinal run -d --restart always \
  -n mysql -p 3306:3306 \
  -v mysqldata:/var/lib/mysql \
  -e MYSQL_ROOT_PASSWORD=rootpass \
  -e MYSQL_DATABASE=myapp \
  -e MYSQL_USER=myapp \
  -e MYSQL_PASSWORD=secret \
  mysql:8

# Connect from another container
mysql -h 10.0.2.1 -u myapp -psecret myapp

# Import SQL
cardinal exec -i mysql mysql -u root -prootpass myapp < dump.sql

# Backup
cardinal exec mysql mysqldump -u root -prootpass myapp > backup.sql

# Interactive shell
cardinal exec -i -t mysql mysql -u root -prootpass
```

### phpMyAdmin

Run phpMyAdmin connected to your MySQL/PostgreSQL container:

```bash
# Connect to MySQL
cardinal run -d --restart always \
  -n phpmyadmin -p 8080:80 \
  -e PMA_HOST=10.0.2.1 \
  -e PMA_PORT=3306 \
  -e UPLOAD_LIMIT=256M \
  phpmyadmin:latest

# Connect to PostgreSQL
cardinal run -d --restart always \
  -n pgadmin -p 8080:80 \
  -e PMA_HOST=10.0.2.1 \
  -e PMA_PORT=5432 \
  phpmyadmin:latest

# With custom server (via environment)
cardinal run -d --restart always \
  -n phpmyadmin -p 8080:80 \
  -e PMA_ARBITRARY=1 \
  phpmyadmin:latest
# Then enter any server IP in the web UI

# phpMyAdmin + MySQL together
cardinal run -d --restart always \
  -n mysql -p 3306:3306 \
  -v mysqldata:/var/lib/mysql \
  -e MYSQL_ROOT_PASSWORD=rootpass \
  mysql:8

# Wait for MySQL, then get its IP and run phpMyAdmin
MYSQL_IP=$(cardinal inspect mysql | grep -o '"ip":"[^"]*"' | grep -o '[0-9.]*')
cardinal run -d --restart always \
  -n phpmyadmin -p 8080:80 \
  -e PMA_HOST=$MYSQL_IP \
  -e PMA_PORT=3306 \
  phpmyadmin:latest

# Access at http://your-server:8080 — login with MySQL credentials
```

**Примечание:** Для PostgreSQL используйте pgAdmin вместо phpMyAdmin:

```bash
cardinal run -d --restart always \
  -n pgadmin -p 8080:80 \
  -e PGADMIN_DEFAULT_EMAIL=admin@example.com \
  -e PGADMIN_DEFAULT_PASSWORD=admin \
  dpage/pgadmin4
```

---

## Dynamic Port Management

Add or remove port mappings on running containers without restart:

```bash
# Add a port
cardinal port add player1 25566:25566

# Add UDP port
cardinal port add player1 27015:27015/udp

# Remove a port
cardinal port remove player1 25566
cardinal port rm player1 25566       # alias
```

- Applies iptables DNAT rules instantly — no restart needed
- Ports persist in container state across restarts
- Useful for donator perks, temporary services, or emergency access

---

## Node Site with Auto-Start

Deploy a Node.js site (like this documentation site) with cardinal in one
command: the project directory is mounted live (content updates without a
rebuild), the container restarts itself after crashes, and the port is opened
automatically.

### One-command deploy (`deploy.sh`)

```bash
./deploy.sh                 # default: name=cardinal, port=443, host network
./deploy.sh --bridge        # bridge network — cardinal opens the port itself
./deploy.sh --port 8080     # deploy on another port
./deploy.sh --force         # force a rebuild
./deploy.sh --restart       # add --restart always (auto-start after reboot)
```

What the script does: removes an existing container with the same name,
creates it (volume `$PWD:/site`, `--workdir /site`, a `--startup` fix for
`/etc/hosts`, command `npm start`), installs dependencies and builds the site
inside the container when needed, opens the port in the host firewall
(`ufw`/`firewalld`/`iptables`), and health-checks the result.

### The equivalent `cardinal run` commands

Host network — the app binds the port directly, cardinal manages nothing:

```bash
cardinal run -d -n cardinal --network host -e PORT=443 -v $PWD:/site \
  --image node:24 --workdir /site \
  --startup "echo '127.0.0.1 localhost' >> /etc/hosts" \
  --cmd "npm start"
```

Bridge network — cardinal manages the port (iptables DNAT + `ufw allow`):

```bash
cardinal run -d -n cardinal -p 443:443 -e PORT=443 -v $PWD:/site \
  --image node:24 --workdir /site \
  --startup "echo '127.0.0.1 localhost' >> /etc/hosts" \
  --cmd "npm start"
```

### Auto-start after reboot

`--restart always` plus the persistent supervisor keeps the site running
across reboots and crashes:

```bash
cardinal bootstrap --install              # install cardinal-bootstrap.service
cardinal set cardinal --restart always    # or add --restart always at creation
```

The supervisor also powers scheduled backups and crash-loop protection.

### Port and firewall notes

- **Bridge mode** — cardinal adds iptables DNAT and `ufw allow <port>/<proto>`
  at start, and removes them on `stop`/`rm`.
- **Host mode** — no mappings exist; open the port yourself:
  `ufw allow 443/tcp` (and check the provider's cloud firewall, which is
  outside the OS).
- Add or remove mappings on a running container with
  `cardinal port add <c> HOST:CONT` / `cardinal port remove <c> HOST`.

### Gotchas

- **`/etc/hosts` in containers is minimal** — builds that resolve `localhost`
  (e.g. Astro) fail with `getaddrinfo ENOTFOUND localhost`. The `--startup`
  line above appends `127.0.0.1 localhost` on every start; you can also set it
  later with `cardinal set <c> --startup "..."`.
- **HTTPS on port 443** — browsers may auto-upgrade `http://...:443` to
  `https://`. Use a plain port (`--port 8080`) or terminate TLS in front
  (Caddy/nginx).

---

## Production Checklist

### Security

- Use `--restart always` for auto-recovery
- Use `--cap-drop ALL --cap-add NET_BIND_SERVICE` for minimal privileges
- Run as non-root user with `--user`
- Use `--readonly` for rootfs when possible
- Use HTTPS with Let's Encrypt
- Keep images updated

### Persistence

- Use named volumes or bind mounts for data
- Database data → mounted volume
- Uploads → mounted volume
- App code → mounted volume (dev) or baked into image (prod)

### Performance

- Set `--memory` and `--cpus` limits for each container
- Use `--startup` for custom init logic (wait for DB, run migrations)
- Use gunicorn/uvicorn with multiple workers for Python
- Use PM2 or cluster mode for Node.js

### Monitoring

```bash
# Check container resource usage
cardinal stats

# Follow logs
cardinal logs -f web

# Check health
cardinal ps
```

### Backup

```bash
# Backup a container's overlay (all changes)
tar -czf container-backup.tar.gz /root/.cardinal/overlay/<container-id>/

# Backup volumes
tar -czf volume-backup.tar.gz /root/.cardinal/volumes/<volume-name>/

# Export an image
cardinal export myapp:v1 -o myapp-backup.tar.gz
```

<!-- dck-version:start -->
**Documentation version:** `1.23.20`
**Project release:** `v1.23.20`
<!-- dck-version:end -->

# Deploying Bots with dck

Run Telegram, Discord, Slack bots and more inside dck containers with persistent storage and auto-restart.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Telegram Bot](#telegram-bot)
- [Discord Bot](#discord-bot)
- [Bot with Database](#bot-with-database)
- [JavaScript Bot (Node.js)](#javascript-bot-nodejs)
- [Bot Health Monitoring](#bot-health-monitoring)
- [Updates & Restarts](#updates--restarts)

---

## Quick Start

All bots follow the same pattern:

```bash
mkdir -p /data/mybot
cd /data/mybot

# 1. Create bot code, requirements.txt, start.sh
# 2. Run (limit: 256MB RAM, 0.25 CPU, 1GB disk):
dck run -d --restart always \
  -n mybot \
  -v /data/mybot:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="your_token" \
  --startup @/bot/start.sh \
  python:3.11-slim
```

The `--startup` script installs dependencies and runs the bot. Changes to files on the host (`/data/mybot/`) are instantly visible inside the container. Restart with `dck restart mybot`.

---

## Telegram Bot

```bash
mkdir -p /data/tg-bot
cd /data/tg-bot

cat > bot.py << 'EOF'
import os
from telegram import Update
from telegram.ext import Application, CommandHandler

TOKEN = os.environ["BOT_TOKEN"]

async def start(update: Update, context):
    await update.message.reply_text("Hello from dck Telegram bot!")

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

dck run -d --restart always \
  -n tg-bot \
  -v /data/tg-bot:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="YOUR_TELEGRAM_BOT_TOKEN" \
  --startup @/bot/start.sh \
  python:3.11-slim

---

## Discord Bot

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

dck run -d --restart always \
  -n discord-bot \
  -v /data/discord-bot:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="YOUR_DISCORD_BOT_TOKEN" \
  --startup @/bot/start.sh \
  python:3.11-slim

---

## Bot with Database

```bash
# 1. Start PostgreSQL (limit: 1GB RAM, 1 CPU, 10GB disk)
dck run -d --restart always \
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

cat > start.sh << 'EOF'
#!/bin/sh
set -e
pip install --no-cache-dir --disable-pip-version-check -r /bot/requirements.txt
exec python /bot/bot.py
EOF

dck run -d --restart always \
  -n db-bot \
  -v /data/bot-db:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="YOUR_TOKEN" \
  -e DB_HOST=10.0.2.1 \
  --startup @/bot/start.sh \
  python:3.11-slim
```

---

## JavaScript Bot (Node.js)

```bash
mkdir -p /data/js-bot
cd /data/js-bot

cat > bot.js << 'EOF'
const { Client, GatewayIntentBits } = require('discord.js');
const client = new Client({ intents: [GatewayIntentBits.Guilds, GatewayIntentBits.GuildMessages] });

client.once('ready', () => console.log(`Logged in as ${client.user.tag}`));
client.on('messageCreate', msg => {
    if (msg.content === '!ping') msg.reply('pong');
});

client.login(process.env.BOT_TOKEN);
EOF

cat > package.json << 'EOF'
{
  "name": "js-bot",
  "dependencies": { "discord.js": "^14.14.1" }
}
EOF

cat > start.sh << 'EOF'
#!/bin/sh
set -e
cd /bot && npm install
exec node bot.js
EOF

dck run -d --restart always \
  -n js-bot \
  -v /data/js-bot:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="YOUR_TOKEN" \
  --startup @/bot/start.sh \
  node:20
```

---

## Bot Health Monitoring

```bash
# Check logs
dck logs -f tg-bot

# Check if running
dck ps | grep bot

# Restart
dck restart discord-bot

# Auto healthcheck (with --startup)
dck run -d --restart always \
  -n tg-bot \
  -v /data/tg-bot:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="token" \
  --startup @/bot/start.sh \
  --healthcheck-cmd "python -c 'import urllib.request; urllib.request.urlopen(\"http://localhost:8080/health\")'" \
  --healthcheck-interval 60 \
  --healthcheck-retries 3 \
  --healthcheck-timeout 10 \
  python:3.11-slim
```

---

## Updates & Restarts

```bash
# Update bot code on host
nano /data/tg-bot/bot.py

# Restart container to apply changes
dck restart tg-bot

# Or if using --startup with pip install, just restart
dck restart discord-bot

# Remove and recreate
dck rm -f tg-bot
dck run -d --restart always ...  # same command as before
```

### Copy files into running container

```bash
# Update code without restart
dck cp ./bot.py tg-bot:/bot/bot.py

# If the bot reloads on file change, it picks up automatically
# Otherwise restart:
dck restart tg-bot
```

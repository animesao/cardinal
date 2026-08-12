<!-- dck-version:start -->
**Documentation version:** `1.23.17`
**Project release:** `v1.23.17`
<!-- dck-version:end -->

# Развёртывание ботов с dck

Запускайте Telegram, Discord, Slack ботов и других в контейнерах dck с постоянным хранилищем и авто-перезапуском.

---

## Содержание

- [Быстрый старт](#быстрый-старт)
- [Telegram бот](#telegram-бот)
- [Discord бот](#discord-бот)
- [Бот с базой данных](#бот-с-базой-данных)
- [JavaScript бот (Node.js)](#javascript-бот-nodejs)
- [Мониторинг бота](#мониторинг-бота)
- [Обновления и перезапуск](#обновления-и-перезапуск)

---

## Быстрый старт

Все боты запускаются по одному шаблону:

```bash
mkdir -p /data/mybot
cd /data/mybot

# 1. Создать код бота, requirements.txt, start.sh
# 2. Запустить (лимит: 256MB RAM, 0.25 CPU, 1GB диск):
dck run -d --restart always \
  -n mybot \
  -v /data/mybot:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="ваш_токен" \
  --startup @/bot/start.sh \
  python:3.11-slim
```

Скрипт `--startup` устанавливает зависимости и запускает бота. Изменения файлов на хосте (`/data/mybot/`) сразу видны внутри контейнера. Перезапуск: `dck restart mybot`.

---

## Telegram бот

```bash
mkdir -p /data/tg-bot
cd /data/tg-bot

cat > bot.py << 'EOF'
import os
from telegram import Update
from telegram.ext import Application, CommandHandler

TOKEN = os.environ["BOT_TOKEN"]

async def start(update: Update, context):
    await update.message.reply_text("Привет от dck Telegram бота!")

async def ping(update: Update, context):
    await update.message.reply_text("pong")

def main():
    app = Application.builder().token(TOKEN).build()
    app.add_handler(CommandHandler("start", start))
    app.add_handler(CommandHandler("ping", ping))
    print("Бот запущен...")
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
  -e BOT_TOKEN="ВАШ_ТОКЕН_TELEGRAM_БОТА" \
  --startup @/bot/start.sh \
  python:3.11-slim

---

## Discord бот

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
    print(f"Зашёл как {bot.user}")

@bot.command()
async def ping(ctx):
    await ctx.send("pong")

@bot.command()
async def hello(ctx):
    await ctx.send(f"Привет {ctx.author.mention}!")

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
  -e BOT_TOKEN="ТОКЕН_ВАШЕГО_DISCORD_БОТА" \
  --startup @/bot/start.sh \
  python:3.11-slim
```

---

## Бот с базой данных

```bash
# 1. PostgreSQL (лимит: 1GB RAM, 1 CPU, 10GB диск)
dck run -d --restart always \
  -n bot-db \
  -v bot_pgdata:/var/lib/postgresql/data \
  --memory 1g --cpus 1 --disk 10G \
  -e POSTGRES_DB=botdb \
  -e POSTGRES_USER=bot \
  -e POSTGRES_PASSWORD=secret \
  postgres:16

# 2. Бот с подключением к БД
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
    print(f"Зашёл как {bot.user}")

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
  -e BOT_TOKEN="ВАШ_ТОКЕН" \
  -e DB_HOST=10.0.2.1 \
  --startup @/bot/start.sh \
  python:3.11-slim
```

---

## JavaScript бот (Node.js)

```bash
mkdir -p /data/js-bot
cd /data/js-bot

cat > bot.js << 'EOF'
const { Client, GatewayIntentBits } = require('discord.js');
const client = new Client({ intents: [GatewayIntentBits.Guilds, GatewayIntentBits.GuildMessages] });

client.once('ready', () => console.log(`Зашёл как ${client.user.tag}`));
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
  -e BOT_TOKEN="ВАШ_ТОКЕН" \
  --startup @/bot/start.sh \
  node:20

---

## Мониторинг бота

```bash
# Логи
dck logs -f tg-bot

# Статус
dck ps | grep bot

# Перезапуск
dck restart discord-bot

# Healthcheck (с --startup)
dck run -d --restart always \
  -n tg-bot \
  -v /data/tg-bot:/bot \
  --workdir /bot \
  --memory 256m --cpus 0.25 --disk 1G \
  -e BOT_TOKEN="токен" \
  --startup @/bot/start.sh \
  --healthcheck-cmd "python -c 'import urllib.request; urllib.request.urlopen(\"http://localhost:8080/health\")'" \
  --healthcheck-interval 60 \
  --healthcheck-retries 3 \
  --healthcheck-timeout 10 \
  python:3.11-slim
```

---

## Обновления и перезапуск

```bash
# Обновить код на хосте
nano /data/tg-bot/bot.py

# Перезапустить контейнер
dck restart tg-bot

# Или удалить и создать заново
dck rm -f tg-bot
dck run -d --restart always ...  # та же команда
```

### Копирование файлов в работающий контейнер

```bash
# Обновить код без перезапуска
dck cp ./bot.py tg-bot:/bot/bot.py

# Если бот перезагружается при изменении файла — подхватит сам
# Иначе перезапустить:
dck restart tg-bot
```

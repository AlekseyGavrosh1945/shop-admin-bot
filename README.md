# Shop Admin Bot

Telegram-бот для магазина на Toflow (Laravel + MySQL): статистика заказов,
товаров и клиентов прямо в телефоне. Только чтение: все запросы к базе —
это `SELECT`.

[![CI](https://github.com/AlekseyGavrosh1945/shop-admin-bot/actions/workflows/ci.yml/badge.svg)](https://github.com/AlekseyGavrosh1945/shop-admin-bot/actions/workflows/ci.yml)

## Что умеет

Всё управление — кнопками, команды не нужны:

```
🏪 Toflow — статистика магазина

[ 📊 Заказы ]        [ 🛒 Товары ]
[ 👥 Клиенты ]       [ 🧾 Последние заказы ]
```

- **📊 Заказы** — количество, оборот, оплаченные/в кредит, средний чек,
  топ способов оплаты. Переключение периода кнопками:
  `Сегодня · 7 дней · Месяц · Всё время`
- **🛒 Товары** — размер каталога, сколько в витрине и сколько позиций
  закончилось, топ-10 продаж, популярные товары «на нуле»
- **👥 Клиенты** — всего и новых за месяц
- **🧾 Последние заказы** — 10 свежих: номер, сумма, статус, метод оплаты

Безопасность:

- **белый список**: бот отвечает только chat ID из `ADMIN_CHAT_IDS`,
  остальные получают отказ (свой ID узнать можно командой `/id`)
- в БД — только `SELECT`; для деплоя рекомендуется отдельный
  MySQL-пользователь без прав на запись (см. ниже)

## Быстрый старт

```bash
git clone https://github.com/AlekseyGavrosh1945/shop-admin-bot
cd shop-admin-bot
cp .env.example .env
# 1) создай бота у @BotFather и впиши токен в BOT_TOKEN
# 2) запусти бота, отправь ему /id, впиши chat ID в ADMIN_CHAT_IDS
# 3) впиши DSN своей базы в DATABASE_URL
# Важно: после правки .env нужен up -d, а не restart —
# restart не перечитывает переменные окружения
docker compose up -d --build
```

## Конфигурация

| Переменная | По умолчанию | Описание |
| --- | --- | --- |
| `BOT_TOKEN` | — | токен от [@BotFather](https://t.me/BotFather), обязателен |
| `ADMIN_CHAT_IDS` | — | chat ID администраторов через запятую |
| `DATABASE_URL` | `root:devroot@tcp(127.0.0.1:3307)/toflow?parseTime=true` | MySQL DSN |
| `HTTP_ADDR` | `:8081` | адрес HTTP-сервера с `/healthz` |

## Архитектура

```
Telegram ──кнопки──▶ Bot (telebot) ──▶ Store (SQL, read-only) ──▶ MySQL магазина
                          │
                          └── /healthz HTTP-сервер
```

- `internal/config` — конфигурация из окружения
- `internal/storage` — запросы к БД, агрегаты заказов/товаров/клиентов
- `internal/telegram` — кнопочное меню, рендеринг, белый список
- `internal/server` — `/healthz` (200/503 по доступности БД)

Пакет `telegram` зависит от узкого интерфейса `Store`, поэтому рендеринг
тестируется на фейковых данных, а SQL — на настоящей схеме.

## Разработка

Схема магазина лежит в `deploy/schema.sql` (структура без данных).
Локальная копия stage-базы:

```bash
docker run -d --name toflow-db -e MYSQL_ROOT_PASSWORD=devroot \
  -e MYSQL_DATABASE=toflow -p 3307:3306 mysql:8
zcat /путь/к/dump.sql.gz | docker exec -i toflow-db mysql -uroot -pdevroot toflow

TEST_DSN='root:devroot@tcp(127.0.0.1:3307)/toflow?parseTime=true' go test ./...
```

- `make test` — юнит-тесты + рендеринг (`go test -race`)
- с `TEST_DSN` добавляются интеграционные SQL-тесты
- в CI поднимается сервис-контейнер MySQL, в него грузится
  `deploy/schema.sql`, и тесты гоняются полностью

## Деплой на сервер магазина

1. Скопируй проект на сервер, заполни `.env` (токен, chat ID, DSN боевой базы).
2. Создай пользователя БД с правами только на чтение:

```sql
CREATE USER 'shopbot'@'%' IDENTIFIED BY 'надёжный_пароль';
GRANT SELECT ON toflow.* TO 'shopbot'@'%';
```

3. `docker compose up -d --build` — и статистика появляется в телефоне.

## Стек

Go 1.27 · telebot v3 · go-sql-driver/mysql · Docker · GitHub Actions

## Лицензия

[MIT](LICENSE)

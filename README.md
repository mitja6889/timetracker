# Beleženje časa

Vue 3 + TypeScript + Vite (frontend), Go (backend), MongoDB.

## Zagon (Docker)

```bash
docker compose up --build
```

Aplikacija: http://localhost:8081 — prijava ob prvem zagonu: `admin` / `admin123`
(geslo takoj spremenite; za drugo vrednost nastavite `ADMIN_USERNAME` / `ADMIN_PASSWORD`
in `JWT_SECRET` v `.env`, preden prvič zaženete).

Admin se ustvari samo, če je zbirka uporabnikov prazna. Hkrati se ustvarita role
**Admin** (sistemska, vse pravice) in **Uporabnik** (samo beleženje časa, vidi samo svoje vnose).

## Razvoj brez Dockerja

```bash
# MongoDB mora teči na localhost:27017
cd backend && go mod tidy && go run .          # API na :8080
cd frontend && npm install && npm run dev      # UI na :5173 (proxy /api -> :8080)
```

Okolijske spremenljivke backenda: `MONGO_URI`, `MONGO_DB`, `JWT_SECRET`, `PORT`,
`ADMIN_USERNAME`, `ADMIN_PASSWORD`.

## Pravice

| Pravica | Pomen |
|---|---|
| `timer.use` | start / pavza / nadaljuj / stop |
| `entries.view_all` | vidi vnose vseh uporabnikov (brez nje samo svoje) |
| `entries.edit_own` | ureja, dodaja in briše lastne vnose |
| `entries.edit_all` | ureja, dodaja in briše vnose vseh |
| `users.manage` | CRUD uporabnikov |
| `roles.manage` | CRUD rol in pravic |

Pravice se berejo iz baze ob vsakem zahtevku, zato sprememba role velja takoj.
Filtriranje po uporabniku se vsili na strežniku: brez `entries.view_all` je
`userId` vedno prijavljeni uporabnik, ne glede na parameter v zahtevku.

## API (povzetek)

- `POST /api/auth/login`, `GET /api/auth/me`
- `GET|POST /api/users`, `PUT|DELETE /api/users/{id}`
- `GET|POST /api/roles`, `PUT|DELETE /api/roles/{id}`, `GET /api/permissions`
- `GET /api/timer/current`, `POST /api/timer/{start|pause|resume|stop}`

Beleženje časa shranjuje **dogodke** `start` / `stop` s časovnim žigom v polje `events`
posameznega vnosa. Delovni čas je vsota intervalov med zaporednima dogodkoma start→stop
(tekoči vnos šteje od zadnjega start do trenutka). Pavza = dogodek `stop`, nadaljevanje = `start`.
- `GET /api/entries?page&pageSize&sort&order&q&status&userId&from&to`
- `POST /api/entries`, `PUT|DELETE /api/entries/{id}`

Zaščite: najmanj en aktiven upravljalec uporabnikov mora ostati, sistemske role
ni mogoče spremeniti ali izbrisati, uporabnik ne more izbrisati samega sebe, največ
en tekoč vnos na uporabnika (unikatni delni indeks).

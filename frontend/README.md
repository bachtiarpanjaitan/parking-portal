# Frontend — Parking Violation Portal

Vite + React 18 + TypeScript + Tailwind + TanStack Query + Zustand.

## Setup

```bash
cd frontend
npm install
```

## Run (dev mode)

The frontend expects the **API Gateway** to be running at `http://localhost:8080`
and the **violation-service** to serve `/uploads/*`.

```bash
# 1. Start the 3 backend services (from `backend/` dir):
go run ./violation-service/cmd/api &
go run ./payment-service/cmd/api &
go run ./gateway/cmd/gateway &

# 2. Start the frontend:
cd ../frontend
npm run dev
```

Open `http://localhost:3000` in your browser.

## What is included

### Stack
- **Vite 5** + **React 18** + **TypeScript strict mode**
- **TailwindCSS 3** (utility-first CSS, no UI library)
- **TanStack Query 5** for data fetching (no raw `fetch` in components)
- **Zustand 5** for auth state (JWT + user)
- **Axios** with interceptors (auto-JWT, error-envelope normalization)
- **react-router-dom 6** for routing
- **Midtrans Snap.js** loaded from CDN in `index.html`

### Modules (7 pages)

| Page | Role | Description |
|------|------|-------------|
| `/login` | public | Email + password sign-in |
| `/` (Dashboard) | both | KPI cards: violations / invoices / pending |
| `/violations` | OFFICER+MEMBER | List + (officer only) create form |
| `/invoices` | OFFICER+MEMBER | List + (member only) Midtrans Snap pay button |
| `/history` | both | Aggregated violation + invoice + payment + rule version |
| `/rules` | OFFICER | Active rule details (base + multipliers) |

### Layout

- Sidebar with role-aware navigation
- Sign-out button
- Per-page heading

### Key features
- Role-aware sidebar (officer sees 5 nav items, member sees 3)
- JWT auto-attached to every API call via axios interceptor
- Snap payment opens `window.snap.pay(snap_token)` with 4 callbacks
- TanStack Query auto-refreshes data after mutations
- Error envelope normalized so `e.message` / `e.code` are accessible

## Test the Midtrans pay flow

1. Open `http://localhost:3000` → log in as `member@example.com` / `password123`
2. Sidebar → **My Invoices** → see the PENDING invoices
3. Click **Pay with Midtrans** on any PENDING invoice
4. Midtrans Snap UI opens → choose **GoPay** or **QRIS** (sandbox mode)
5. For testing without real money, use Midtrans's test card numbers or
   the QRIS simulator that Midtrans provides in sandbox.

> The Snap JS is loaded from `app.sandbox.midtrans.com`. The
> `data-client-key` in `index.html` should match the client key
> configured in the Midtrans dashboard for the server key in `.env`.

## Build for production

```bash
npm run build
```

Output goes to `dist/`. Serve with any static file server (e.g. nginx — the
repo already includes a `frontend/nginx.conf` for this).

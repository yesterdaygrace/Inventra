# Inventra Web — React Frontend

React 19 + TypeScript + Vite + Tailwind 4 + Radix UI + TanStack Query + React Router

See `../docs/frontend.md` for the full 18-screenshot gallery and feature walkthrough.

## Quick start

```bash
npm install
npm run dev     # http://localhost:5173
npm run build
npm run preview
```

Backend: `make docker-up` + `make seed` (API `:8080`, Postgres `:5432`). With `DEMO_MODE=true`, the frontend automatically signs in as `demo@inventory.local`; no login form is required.

Screenshots live in `docs/screenshoot/demo-*.png` — all 18 are tracked and rendered in `docs/frontend.md` and `README.md`.

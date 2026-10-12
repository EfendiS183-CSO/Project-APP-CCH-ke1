# CCH Frontend

React/Vite client for CCH. Frontend code is organized around product capabilities and mirrors the Go backend's domain boundaries. A feature owns its pages, domain components, and API methods; application wiring and generic HTTP transport stay outside feature folders.

## Run

```powershell
npm install
npm run dev
```

The Vite proxy targets the Go API at `http://127.0.0.1:8080` by default, matching `CCH-BE`. Override it with `VITE_CCH_BE_TARGET` when the API runs elsewhere. The AI proxy remains at `http://127.0.0.1:8000`.

```powershell
npm run lint
npm run build
```

## Source Layout

```text
src/
  app/                         # App composition, route table, shell, route guards
  features/
    auth/                      # Staff-token session and its page/context
    intake/
      submission/              # Public intake page, steps, result, submission API
      review/                  # CS queue, draft review, customer picker, ticket API
  shared/
    api/                       # Fetch transport and common API error mapping
  styles/                      # Global design tokens, reset, and responsive styles
  assets/                      # App-wide static assets
```

Keep domain UI and domain API calls beside their feature. `app/` may compose features but must not contain business validation or direct endpoint details. `shared/` is only for behavior genuinely reused by multiple domains; do not turn it into a catch-all components folder. A new backend domain gets a corresponding `features/<domain>/` only when its FE workflow is implemented.

## Current Workflows

- `/intake` creates a guest session and submits consented complaint data. The backend checks the exact `Origin` allowlist, and the submission uses `X-Intake-Session`.
- `/review` and `/review/drafts/:id` use the CS review contract with `X-Authorization: Bearer <staff JWT>`. Draft ticket creation requires explicit identity-verification attestation in the UI; authorization and all business validation remain server-side.
- `/login` is a local token-session form, not an authentication endpoint. CCH-BE does not currently expose staff login; the token is held in the current browser tab's `sessionStorage` and is validated by BE requests.

All business routes are default-off in CCH-BE. The FE does not enable backend flags. Configure the relevant Go flags, JWT/session secrets, database, CCH gateway, and intake Origin allowlist separately before trying these routes.

## Contract Gaps

The guest submission API does not expose public lookup endpoints for complaint types, shipping-service codes, or office codes. The current form accepts CCH codes rather than inventing labels or values. A public, allowlisted lookup contract should be agreed in CCH-BE before this form is used for production intake. There is no FE parity yet for complaint dashboard/list, status, follow-ups, documents, customer/master administration, reports, or AI workflows.

The FE does not access the live `cch` schema. CCH-BE remains responsible for authentication, authorization, validation, office scoping, and persistence.

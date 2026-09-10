# Frontend — Slice 1 Design (Recruiter Shell + Candidate Landing)

**Date:** 2026-09-10
**Status:** Approved for build
**Depends on:** backend-go slices 1–5 (through the join endpoint, `f295dbd` + slice 5)
**Scope:** The React shell — recruiter auth + dashboard + jobs/candidates/schedule
+ interview detail with state advance, and the public candidate landing page.
No call/WebRTC UI.

## 1. Goal

`docs/PLAN.md` Phase 0: "React frontend shell: recruiter dashboard, scheduling
flow, basic call UI shell." Plus the candidate landing page that shows whether
the interview has started or the candidate is late.

## 2. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | `frontend/` at repo root; Vite + React 19 + TS, **npm** | User directive; matches `/backend-go`, `/ai-service-python` |
| D2 | **Tailwind v4 + shadcn/ui** (`@/` alias, `cn`, Radix) | User directive; fast, accessible primitives without a heavy design system |
| D3 | React Router 7, TanStack Query v5, react-hook-form + zod | Standard, small; server state and forms are the bulk of a shell |
| D4 | Access token in `localStorage`; `401` → clear + redirect `/login` | Backend has no refresh token (slice 1 decision); keep it simple |
| D5 | Vite dev proxy `/api` → `http://localhost:8080` | Avoids CORS fiddling in dev; one env var for prod |
| D6 | Hand-written TS types mirroring the Go JSON | No codegen for a shell; the surface is small and stable |
| D7 | No enum baked in — state labels + legal transitions come from `GET /session-states` | `docs/ARCHITECTURE.md` §4 |
| D8 | Candidate `phase` comes from the server; the page only renders it (+ a local countdown for `before`) | The server clock is authoritative |

## 3. Stack / setup

```
frontend/
  package.json  vite.config.ts  tsconfig.json  tsconfig.app.json  tsconfig.node.json
  components.json                  # shadcn config
  index.html  src/index.css        # @import "tailwindcss"; shadcn tokens
  .env.example                     # VITE_API_BASE_URL=/api  VITE_WS_BASE_URL=ws://localhost:8090
  eslint.config.js
  src/
    main.tsx  App.tsx
    lib/
      utils.ts                     # cn()
      api.ts                       # apiFetch(path, opts): base url + bearer + 401 handling + JSON
      queryClient.ts
    api/
      types.ts                     # User, Job, Candidate, Interview, InterviewDetail, StateGraph, JoinStatus
      recruiter.ts                 # typed endpoint fns (login, me, jobs, candidates, interviews, advance, sessionStates)
      candidate.ts                 # getJoinStatus(token)
      hooks.ts                     # TanStack Query hooks wrapping the above
    auth/
      AuthProvider.tsx             # { token, user, login(), logout() }; persists token
      useAuth.ts
      RequireAuth.tsx              # <Outlet/> guard -> redirect /login
    components/
      ui/                          # shadcn: button card input label select table badge sonner skeleton dialog form
      AppShell.tsx                 # sidebar/topbar + <Outlet/>
      StateBadge.tsx               # colour by state (terminal = muted/red, active = blue, done = green)
      DataError.tsx  Loading.tsx
    routes.tsx                     # createBrowserRouter
    pages/
      Login.tsx
      Dashboard.tsx                # interviews table
      Jobs.tsx
      Candidates.tsx
      ScheduleInterview.tsx
      InterviewDetail.tsx
      Join.tsx                     # public /join/:token
      NotFound.tsx
  src/__tests__/                   # vitest + @testing-library/react + jsdom
  vitest.config.ts  src/test/setup.ts
```

Dependencies: `react` `react-dom` `react-router-dom` `@tanstack/react-query`
`react-hook-form` `zod` `@hookform/resolvers` `class-variance-authority` `clsx`
`tailwind-merge` `lucide-react` `sonner` + Radix packages shadcn pulls.
Dev: `vite` `@vitejs/plugin-react` `typescript` `tailwindcss` `@tailwindcss/vite`
`eslint` + plugins, `vitest` `@testing-library/react` `@testing-library/user-event`
`@testing-library/jest-dom` `jsdom` `@types/*`.

## 4. API client (`src/lib/api.ts`)

```ts
export async function apiFetch<T>(path: string, opts?: RequestInit & { auth?: boolean }): Promise<T>
```
- base = `import.meta.env.VITE_API_BASE_URL ?? "/api"`; final URL = `${base}/v1${path}`.
- when `opts.auth !== false`, attach `Authorization: Bearer <token from storage>`.
- `Content-Type: application/json` for bodies; parse JSON responses.
- non-2xx → throw `ApiError { status, message }` (message from `{message}` or
  `{error:{message}}` in the body). On `401` (and `auth` was on): call the
  registered `onUnauthorized()` (AuthProvider wires this to `logout()` +
  `navigate("/login")`).

## 5. Types (`src/api/types.ts`)

Mirror the Go JSON exactly:
```ts
type Role = "scheduler" | "candidate";
interface LoginResult { access_token: string; token_type: string; expires_in: number;
  user: { id: string; email: string; role: Role; org_id: string } }
interface Job { id; org_id; title; description; created_at; updated_at }
interface Candidate { id; org_id; email; name; resume_text; created_at; updated_at }
interface JobRef { id; title }        interface CandRef { id; name; email }
interface InterviewDetail { id; state; state_label; scheduled_at; job: JobRef;
  candidate: CandRef; created_at; join_token?: string; join_url?: string }
interface StateGraph { states: { name; label; is_terminal }[];
  transitions: { from; to }[] }
interface JoinStatus { job_title; scheduled_at; server_now;
  phase: "before" | "open" | "late" | "closed"; late_by_seconds: number }
```
List endpoints return `{ data: T[] }`.

## 6. Auth (`src/auth/`)

`AuthProvider` holds `{ token, user }`, hydrates from `localStorage`
(`oriva.token`, `oriva.user`), exposes `login(email,password)` (calls the
endpoint, stores, sets state) and `logout()` (clears, resets query cache).
`RequireAuth` renders `<Outlet/>` when `token` is set, else `<Navigate to="/login" replace/>`.
The API client's `onUnauthorized` is registered in a `useEffect` in `AuthProvider`.

## 7. Routes (`src/routes.tsx`)

```
/login                      Login              (public; redirects to / if already authed)
/join/:token                Join               (public, standalone — no AppShell)
/                           RequireAuth > AppShell
  /                (index)  Dashboard
  /jobs                     Jobs
  /candidates               Candidates
  /schedule                 ScheduleInterview
  /interviews/:id           InterviewDetail
*                           NotFound
```

## 8. Pages

- **Login** — email/password form (react-hook-form + zod); on success →
  `navigate(from ?? "/")`. shadcn `Card` + `Input` + `Button`; error via inline
  alert.
- **Dashboard** — `useInterviews({ state })`. shadcn `Table`: candidate, job,
  `<StateBadge>`, scheduled time, → `/interviews/:id`. A `Select` filters by
  state (options from `useSessionStates`). Empty state + `Loading` skeleton.
- **Jobs** — table of jobs + a "New job" `Dialog` (title, description) →
  `useCreateJob`, toast on success.
- **Candidates** — table + "New candidate" `Dialog` (email, name, resume_text) →
  `useCreateCandidate`; 409 → "a candidate with that email already exists".
- **ScheduleInterview** — `Select` job, `Select` candidate, `datetime-local`
  input (converted to RFC3339, must be future) → `useScheduleInterview`. On
  success → `navigate("/interviews/" + id)` and a toast "invite emailed to
  <email>".
- **InterviewDetail** — `useInterview(id)`. Shows job/candidate, `<StateBadge>`,
  scheduled time, and a **Join link card**: the `join_url` with a Copy button and
  "invite emailed to <candidate.email>". **Advance** control: `Select` of legal
  target states — computed from `useSessionStates` (`transitions` where
  `from === state`) — + optional reason `Input` + Confirm → `useAdvanceState`;
  disabled when the current state is terminal. Errors (409 illegal / 409
  concurrent) surface as toasts.
- **Join** (`/join/:token`) — `useJoinStatus(token)` with
  `refetchInterval: 15_000`. Standalone centered `Card`, no shell. By `phase`:
  - `before` → "Your interview hasn't started yet." + a live countdown to
    `scheduled_at` (client `setInterval`, seeded from `server_now` vs local now to
    correct clock skew) + the job title.
  - `open` → "You're on time — you can begin." + **Start interview** button →
    routes to an in-page placeholder ("The interview experience is coming
    soon."). Call UI is deferred.
  - `late` → "You're **{mm} minutes** late." (`Math.round(late_by_seconds/60)`) +
    a muted **Start interview** button (same placeholder).
  - `closed` → "This interview has ended."
  - 404 from the API → "This interview link is invalid or has expired."

## 9. shadcn setup

`npx shadcn@latest init` (Tailwind v4, `@/` alias, CSS variables). Add:
`button card input label select table badge sonner skeleton dialog form`.
`<Toaster/>` mounted once in `App.tsx`.

## 10. Testing (`vitest`, `jsdom`)

- `apiFetch` — attaches the bearer header; a `401` triggers `onUnauthorized`;
  parses `{message}` and `{error:{message}}` error bodies. (`fetch` mocked.)
- `RequireAuth` — no token → redirects to `/login`; token → renders children.
- `Login` — submitting valid creds calls the login endpoint and navigates.
- `Join` — render with each mocked `phase`: asserts the right copy ("hasn't
  started", "X minutes late", "has ended", "Start interview"); 404 → invalid-link
  message.
- `StateBadge` — renders the label; terminal states get the muted variant.

`npm test` runs vitest; `npm run build` type-checks + builds. No e2e.

## 11. Out of scope (later slices)

- The call/WebRTC experience the "Start interview" button opens.
- Candidate authentication beyond the token.
- Interview transcript / scores views (no data yet).
- Editing/deleting jobs & candidates (backend has PATCH; the shell only lists +
  creates + the interview advance).
- Pagination, search, bulk actions.
- Real-time updates (dashboard is fetch-on-load + manual refresh; Join polls).
- Theming / dark mode toggle (ship shadcn defaults).
- CI, deployment, Dockerfile.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, describe, expect, it, vi } from "vitest"

import type { JoinStatus } from "@/api/types"
import { Join } from "@/pages/Join"

function mockJoin(status: number, body: Partial<JoinStatus> | { message: string }) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: status < 300,
      status,
      text: () => Promise.resolve(JSON.stringify(body)),
    }),
  )
}

function renderJoin() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={["/join/tok"]}>
        <Routes>
          <Route path="/join/:token" element={<Join />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const base = {
  job_title: "Backend Engineer",
  scheduled_at: "2099-01-01T10:00:00Z",
  server_now: "2098-12-31T10:00:00Z",
  late_by_seconds: 0,
}

describe("Join page", () => {
  afterEach(() => vi.unstubAllGlobals())

  it("shows 'hasn't started' when before", async () => {
    mockJoin(200, { ...base, phase: "before" })
    renderJoin()
    expect(await screen.findByText(/hasn't started yet/i)).toBeInTheDocument()
  })

  it("shows Start interview when open", async () => {
    mockJoin(200, { ...base, phase: "open" })
    renderJoin()
    expect(await screen.findByText(/on time/i)).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /start interview/i })).toBeInTheDocument()
  })

  it("shows minutes late when late", async () => {
    mockJoin(200, { ...base, phase: "late", late_by_seconds: 300 })
    renderJoin()
    expect(await screen.findByText(/5 minutes late/i)).toBeInTheDocument()
  })

  it("shows ended when closed", async () => {
    mockJoin(200, { ...base, phase: "closed" })
    renderJoin()
    expect(await screen.findByText(/has ended/i)).toBeInTheDocument()
  })

  it("shows invalid-link message on 404", async () => {
    mockJoin(404, { message: "invalid or expired interview link" })
    renderJoin()
    expect(await screen.findByText(/invalid or has expired/i)).toBeInTheDocument()
  })
})

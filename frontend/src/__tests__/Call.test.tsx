import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { act, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import type { JoinStatus } from "@/api/types"

// --- mock the Pipecat SDK ---
const connect = vi.fn().mockResolvedValue(undefined)
const disconnect = vi.fn().mockResolvedValue(undefined)
let lastCallbacks: Record<string, (arg?: unknown) => void> = {}
let lastTransportOpts: { wsUrl?: string } = {}

vi.mock("@pipecat-ai/client-js", () => ({
  PipecatClient: class {
    constructor(opts: { callbacks: Record<string, (arg?: unknown) => void> }) {
      lastCallbacks = opts.callbacks
    }
    connect = connect
    disconnect = disconnect
  },
}))
vi.mock("@pipecat-ai/websocket-transport", () => ({
  WebSocketTransport: class {
    constructor(opts: { wsUrl: string }) {
      lastTransportOpts = opts
    }
  },
}))

import { Call } from "@/pages/Call"

function mockJoin(body: Partial<JoinStatus>) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: () => Promise.resolve(JSON.stringify(body)),
    }),
  )
}

function renderCall() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={["/interview/tok-1"]}>
        <Routes>
          <Route path="/interview/:token" element={<Call />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

const open: Partial<JoinStatus> = {
  job_title: "Backend Engineer",
  phase: "open",
  ai_ws_url: "ws://ai.test/ws",
  session_id: "s1",
  scheduled_at: "2099-01-01T10:00:00Z",
  server_now: "2099-01-01T10:00:00Z",
  late_by_seconds: 0,
}

describe("Call page", () => {
  beforeEach(() => {
    connect.mockClear()
    disconnect.mockClear()
    vi.stubGlobal("navigator", {
      mediaDevices: { getUserMedia: vi.fn().mockResolvedValue({}) },
    })
  })
  afterEach(() => vi.unstubAllGlobals())

  it("shows the lobby for an open interview", async () => {
    mockJoin(open)
    renderCall()
    expect(await screen.findByText(/Interview for Backend Engineer/i)).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /start interview/i })).toBeInTheDocument()
  })

  it("shows ended for a closed interview", async () => {
    mockJoin({ ...open, phase: "closed" })
    renderCall()
    expect(await screen.findByText(/has ended/i)).toBeInTheDocument()
  })

  it("connects to ai_ws_url with the token and renders transcript", async () => {
    mockJoin(open)
    renderCall()
    await userEvent.click(await screen.findByRole("button", { name: /start interview/i }))

    expect(lastTransportOpts.wsUrl).toBe("ws://ai.test/ws?token=tok-1")
    expect(connect).toHaveBeenCalled()

    await act(async () => {
      lastCallbacks.onConnected?.()
      lastCallbacks.onBotTranscript?.({ text: "Tell me about a project." })
      lastCallbacks.onUserTranscript?.({ text: "I built a payments system.", final: true })
    })

    expect(await screen.findByText(/Tell me about a project/i)).toBeInTheDocument()
    expect(screen.getByText(/I built a payments system/i)).toBeInTheDocument()
  })

  it("shows a mic-permission error when getUserMedia is denied", async () => {
    mockJoin(open)
    vi.stubGlobal("navigator", {
      mediaDevices: { getUserMedia: vi.fn().mockRejectedValue(new Error("denied")) },
    })
    renderCall()
    await userEvent.click(await screen.findByRole("button", { name: /start interview/i }))
    expect(await screen.findByText(/Microphone access is required/i)).toBeInTheDocument()
    expect(connect).not.toHaveBeenCalled()
  })

  it("ends the call on the End button", async () => {
    mockJoin(open)
    renderCall()
    await userEvent.click(await screen.findByRole("button", { name: /start interview/i }))
    await act(async () => lastCallbacks.onConnected?.())
    await userEvent.click(await screen.findByRole("button", { name: /end interview/i }))
    expect(disconnect).toHaveBeenCalled()
  })
})

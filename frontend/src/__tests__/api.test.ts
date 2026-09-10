import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { apiFetch, ApiError, setUnauthorizedHandler, TOKEN_KEY } from "@/lib/api"

describe("apiFetch", () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })
  afterEach(() => vi.unstubAllGlobals())

  function mockFetch(status: number, body: unknown) {
    const f = vi.fn().mockResolvedValue({
      ok: status >= 200 && status < 300,
      status,
      text: () => Promise.resolve(body == null ? "" : JSON.stringify(body)),
    })
    vi.stubGlobal("fetch", f)
    return f
  }

  it("attaches the bearer token", async () => {
    localStorage.setItem(TOKEN_KEY, "abc123")
    const f = mockFetch(200, { ok: true })
    await apiFetch("/jobs")
    const headers = (f.mock.calls[0][1] as RequestInit).headers as Headers
    expect(headers.get("Authorization")).toBe("Bearer abc123")
  })

  it("omits the bearer when auth:false", async () => {
    localStorage.setItem(TOKEN_KEY, "abc123")
    const f = mockFetch(200, {})
    await apiFetch("/join/x", { auth: false })
    const headers = (f.mock.calls[0][1] as RequestInit).headers as Headers
    expect(headers.get("Authorization")).toBeNull()
  })

  it("calls onUnauthorized and throws on 401", async () => {
    localStorage.setItem(TOKEN_KEY, "abc123")
    mockFetch(401, { message: "nope" })
    const handler = vi.fn()
    setUnauthorizedHandler(handler)
    await expect(apiFetch("/jobs")).rejects.toBeInstanceOf(ApiError)
    expect(handler).toHaveBeenCalled()
  })

  it("parses {message} and {error:{message}} error bodies", async () => {
    mockFetch(409, { message: "already exists" })
    await expect(apiFetch("/candidates", { method: "POST" })).rejects.toThrow("already exists")

    mockFetch(400, { error: { message: "bad input" } })
    await expect(apiFetch("/candidates", { method: "POST" })).rejects.toThrow("bad input")
  })
})

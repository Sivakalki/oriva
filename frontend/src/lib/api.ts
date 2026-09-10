const BASE = import.meta.env.VITE_API_BASE_URL ?? "/api"

export const TOKEN_KEY = "oriva.token"

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

let onUnauthorized: (() => void) | null = null
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

interface Options extends RequestInit {
  auth?: boolean
  json?: unknown
}

export async function apiFetch<T>(path: string, opts: Options = {}): Promise<T> {
  const { auth = true, json, headers, ...rest } = opts
  const h = new Headers(headers)
  if (json !== undefined) {
    h.set("Content-Type", "application/json")
  }
  if (auth) {
    const token = localStorage.getItem(TOKEN_KEY)
    if (token) h.set("Authorization", `Bearer ${token}`)
  }

  const res = await fetch(`${BASE}/v1${path}`, {
    ...rest,
    headers: h,
    body: json !== undefined ? JSON.stringify(json) : rest.body,
  })

  if (res.status === 401 && auth) {
    onUnauthorized?.()
    throw new ApiError(401, "Your session has expired. Please sign in again.")
  }

  const text = await res.text()
  const body: unknown = text ? JSON.parse(text) : null

  if (!res.ok) {
    throw new ApiError(res.status, errorMessage(body) ?? `Request failed (${res.status})`)
  }
  return body as T
}

function errorMessage(body: unknown): string | undefined {
  if (body && typeof body === "object") {
    const b = body as Record<string, unknown>
    if (typeof b.message === "string") return b.message
    if (b.error && typeof b.error === "object") {
      const e = b.error as Record<string, unknown>
      if (typeof e.message === "string") return e.message
    }
  }
  return undefined
}

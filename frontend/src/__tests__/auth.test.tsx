import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { beforeEach, describe, expect, it } from "vitest"

import { AuthProvider } from "@/auth/AuthProvider"
import { RequireAuth } from "@/auth/RequireAuth"
import { StateBadge } from "@/components/StateBadge"
import { TOKEN_KEY } from "@/lib/api"

function renderApp() {
  const qc = new QueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <AuthProvider>
        <MemoryRouter initialEntries={["/secret"]}>
          <Routes>
            <Route element={<RequireAuth />}>
              <Route path="/secret" element={<div>secret content</div>} />
            </Route>
            <Route path="/login" element={<div>login page</div>} />
          </Routes>
        </MemoryRouter>
      </AuthProvider>
    </QueryClientProvider>,
  )
}

describe("RequireAuth", () => {
  beforeEach(() => localStorage.clear())

  it("redirects to /login without a session", () => {
    renderApp()
    expect(screen.getByText("login page")).toBeInTheDocument()
  })

  it("renders children with a session", () => {
    localStorage.setItem(TOKEN_KEY, "tok")
    localStorage.setItem("oriva.user", JSON.stringify({ id: "u", email: "a@b.com", role: "scheduler", org_id: "o" }))
    renderApp()
    expect(screen.getByText("secret content")).toBeInTheDocument()
  })
})

describe("StateBadge", () => {
  it("renders the label", () => {
    render(<StateBadge state="in_progress" label="In progress" />)
    expect(screen.getByText("In progress")).toBeInTheDocument()
  })
})

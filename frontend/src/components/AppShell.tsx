import { Briefcase, CalendarPlus, LayoutDashboard, LogOut, Users } from "lucide-react"
import { NavLink, Outlet } from "react-router-dom"

import { useAuth } from "@/auth/useAuth"
import { ThemeToggle } from "@/components/ThemeToggle"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

const nav = [
  { to: "/", label: "Interviews", icon: LayoutDashboard, end: true },
  { to: "/schedule", label: "Schedule", icon: CalendarPlus },
  { to: "/jobs", label: "Jobs", icon: Briefcase },
  { to: "/candidates", label: "Candidates", icon: Users },
]

export function AppShell() {
  const { user, logout } = useAuth()
  return (
    <div className="flex min-h-screen bg-background">
      <aside className="hidden w-56 shrink-0 flex-col border-r px-3 py-5 sm:flex">
        <div className="flex items-center gap-2 px-2 pb-6">
          <div className="flex size-7 items-center justify-center rounded-md bg-primary text-sm font-bold text-primary-foreground">
            O
          </div>
          <span className="text-base font-semibold">Oriva</span>
        </div>
        <nav className="flex flex-1 flex-col gap-0.5">
          {nav.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-2.5 rounded-md px-2.5 py-2 text-sm transition-colors",
                  isActive
                    ? "bg-accent font-medium text-accent-foreground"
                    : "text-muted-foreground hover:bg-accent/60 hover:text-foreground",
                )
              }
            >
              <Icon className="size-4" />
              {label}
            </NavLink>
          ))}
        </nav>
      </aside>

      <div className="flex flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-end gap-3 border-b px-4 sm:px-6">
          <ThemeToggle />
          <div className="h-5 w-px bg-border" />
          <span className="hidden truncate text-sm text-muted-foreground sm:inline">
            {user?.email}
          </span>
          <Button variant="ghost" size="icon" onClick={logout} aria-label="Sign out">
            <LogOut className="size-4" />
          </Button>
        </header>
        <main className="flex-1 px-4 py-6 sm:px-8 sm:py-8">
          <div className="mx-auto max-w-6xl">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}

import { Briefcase, CalendarPlus, LayoutDashboard, LogOut, Users } from "lucide-react"
import { NavLink, Outlet } from "react-router-dom"

import { useAuth } from "@/auth/useAuth"
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
    <div className="flex min-h-screen">
      <aside className="flex w-56 flex-col border-r bg-card px-3 py-4">
        <div className="px-2 pb-4 text-lg font-semibold">Oriva</div>
        <nav className="flex flex-1 flex-col gap-1">
          {nav.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  "flex items-center gap-2 rounded-md px-2 py-2 text-sm",
                  isActive ? "bg-accent font-medium" : "text-muted-foreground hover:bg-accent/60",
                )
              }
            >
              <Icon className="size-4" />
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="border-t pt-3">
          <div className="truncate px-2 pb-2 text-xs text-muted-foreground">{user?.email}</div>
          <Button variant="ghost" size="sm" className="w-full justify-start" onClick={logout}>
            <LogOut className="size-4" /> Sign out
          </Button>
        </div>
      </aside>
      <main className="flex-1 bg-background p-8">
        <Outlet />
      </main>
    </div>
  )
}

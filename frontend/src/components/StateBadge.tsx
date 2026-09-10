import { Badge } from "@/components/ui/badge"

const TERMINAL = new Set(["scored", "declined", "abandoned", "failed"])
const DONE = new Set(["completed", "scoring", "scored"])

export function StateBadge({ state, label }: { state: string; label: string }) {
  const variant = TERMINAL.has(state)
    ? state === "failed" || state === "abandoned" || state === "declined"
      ? "destructive"
      : "success"
    : DONE.has(state)
      ? "info"
      : state === "scheduled"
        ? "muted"
        : "default"
  return <Badge variant={variant}>{label}</Badge>
}

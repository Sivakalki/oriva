import { CalendarClock, CheckCircle2, ListChecks, Radio } from "lucide-react"
import { useState } from "react"
import { Link } from "react-router-dom"

import type { InterviewDetail } from "@/api/types"
import { useInterviews, useSessionStates } from "@/api/hooks"
import { PageHeader } from "@/components/PageHeader"
import { StateBadge } from "@/components/StateBadge"
import { Card, CardContent } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { fmtDateTime } from "@/lib/format"

const UPCOMING_STATES = new Set(["scheduled", "invited", "ready"])
const LIVE_STATES = new Set(["dispatched", "in_progress"])

function countBy(interviews: InterviewDetail[], states: Set<string>): number {
  return interviews.filter((iv) => states.has(iv.state)).length
}

function StatCard({
  label,
  value,
  icon: Icon,
}: {
  label: string
  value: number
  icon: typeof CalendarClock
}) {
  return (
    <Card>
      <CardContent className="flex items-center gap-4 p-5">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-md bg-accent text-accent-foreground">
          <Icon className="size-5" />
        </div>
        <div>
          <div className="text-2xl font-semibold leading-none">{value}</div>
          <div className="mt-1 text-sm text-muted-foreground">{label}</div>
        </div>
      </CardContent>
    </Card>
  )
}

export function Dashboard() {
  const [state, setState] = useState<string>("all")
  const all = useInterviews()
  const filtered = useInterviews(state === "all" ? undefined : state)
  const graph = useSessionStates()

  const stats = all.data ?? []

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Interviews" description="Every interview scheduled across your jobs." />

      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard label="Total" value={stats.length} icon={ListChecks} />
        <StatCard label="Upcoming" value={countBy(stats, UPCOMING_STATES)} icon={CalendarClock} />
        <StatCard label="Live now" value={countBy(stats, LIVE_STATES)} icon={Radio} />
        <StatCard
          label="Scored"
          value={countBy(stats, new Set(["scored"]))}
          icon={CheckCircle2}
        />
      </div>

      <Card>
        <CardContent className="p-5">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-sm font-medium text-muted-foreground">All interviews</h2>
            <Select value={state} onValueChange={setState}>
              <SelectTrigger className="w-48">
                <SelectValue placeholder="All states" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All states</SelectItem>
                {graph.data?.states.map((s) => (
                  <SelectItem key={s.name} value={s.name}>
                    {s.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {filtered.isPending ? (
            <Skeleton className="h-48 w-full" />
          ) : filtered.error ? (
            <p className="text-sm text-destructive">{(filtered.error as Error).message}</p>
          ) : filtered.data.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No interviews yet.{" "}
              <Link className="underline" to="/schedule">
                Schedule one.
              </Link>
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Candidate</TableHead>
                  <TableHead>Job</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead>Scheduled</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.data.map((iv) => (
                  <TableRow key={iv.id}>
                    <TableCell>
                      <Link className="font-medium hover:underline" to={`/interviews/${iv.id}`}>
                        {iv.candidate.name}
                      </Link>
                      <div className="text-xs text-muted-foreground">{iv.candidate.email}</div>
                    </TableCell>
                    <TableCell>{iv.job.title}</TableCell>
                    <TableCell>
                      <StateBadge state={iv.state} label={iv.state_label} />
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {fmtDateTime(iv.scheduled_at)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

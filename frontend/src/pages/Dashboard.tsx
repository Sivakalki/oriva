import { useState } from "react"
import { Link } from "react-router-dom"

import { useInterviews, useSessionStates } from "@/api/hooks"
import { StateBadge } from "@/components/StateBadge"
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

export function Dashboard() {
  const [state, setState] = useState<string>("all")
  const interviews = useInterviews(state === "all" ? undefined : state)
  const graph = useSessionStates()

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Interviews</h1>
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

      {interviews.isPending ? (
        <Skeleton className="h-48 w-full" />
      ) : interviews.error ? (
        <p className="text-sm text-destructive">{(interviews.error as Error).message}</p>
      ) : interviews.data.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No interviews yet. <Link className="underline" to="/schedule">Schedule one.</Link>
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
            {interviews.data.map((iv) => (
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
    </div>
  )
}

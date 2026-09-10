import { Copy } from "lucide-react"
import { useMemo, useState } from "react"
import { useParams } from "react-router-dom"
import { toast } from "sonner"

import { useAdvanceState, useInterview, useSessionStates } from "@/api/hooks"
import { StateBadge } from "@/components/StateBadge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { fmtDateTime } from "@/lib/format"

export function InterviewDetail() {
  const { id = "" } = useParams()
  const interview = useInterview(id)
  const graph = useSessionStates()
  const advance = useAdvanceState(id)

  const [target, setTarget] = useState("")
  const [reason, setReason] = useState("")

  const legalTargets = useMemo(() => {
    const g = graph.data
    const iv = interview.data
    if (!g || !iv) return []
    const labelOf = (n: string) => g.states.find((s) => s.name === n)?.label ?? n
    return g.transitions
      .filter((t) => t.from === iv.state)
      .map((t) => ({ name: t.to, label: labelOf(t.to) }))
  }, [graph.data, interview.data])

  if (interview.isPending) return <Skeleton className="h-64 w-full" />
  if (interview.error)
    return <p className="text-sm text-destructive">{(interview.error as Error).message}</p>

  const iv = interview.data
  const terminal = legalTargets.length === 0

  const doAdvance = async () => {
    try {
      await advance.mutateAsync({ to_state: target, reason })
      toast.success("State advanced")
      setTarget("")
      setReason("")
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Could not advance state")
    }
  }

  return (
    <div className="flex max-w-2xl flex-col gap-6">
      <div>
        <h1 className="text-2xl font-semibold">{iv.candidate.name}</h1>
        <p className="text-muted-foreground">
          {iv.job.title} · {fmtDateTime(iv.scheduled_at)}
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            State <StateBadge state={iv.state} label={iv.state_label} />
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {terminal ? (
            <p className="text-sm text-muted-foreground">This interview is in a final state.</p>
          ) : (
            <>
              <div className="grid gap-1.5">
                <Label>Advance to</Label>
                <Select value={target} onValueChange={setTarget}>
                  <SelectTrigger className="w-64">
                    <SelectValue placeholder="Choose next state" />
                  </SelectTrigger>
                  <SelectContent>
                    {legalTargets.map((t) => (
                      <SelectItem key={t.name} value={t.name}>
                        {t.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="reason">Reason (optional)</Label>
                <Input id="reason" value={reason} onChange={(e) => setReason(e.target.value)} />
              </div>
              <Button
                className="w-fit"
                disabled={!target || advance.isPending}
                onClick={doAdvance}
              >
                Advance
              </Button>
            </>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Candidate link</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <Input readOnly value={iv.join_url ?? ""} className="font-mono text-xs" />
            <Button
              variant="outline"
              size="icon"
              onClick={() => {
                void navigator.clipboard.writeText(iv.join_url ?? "")
                toast.success("Link copied")
              }}
            >
              <Copy className="size-4" />
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            Invite emailed to {iv.candidate.email}. The link works any time — the page lets the
            candidate in at {fmtDateTime(iv.scheduled_at)}.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"

import { useCandidates, useJobs, useScheduleInterview } from "@/api/hooks"
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
import { toRFC3339 } from "@/lib/format"

export function ScheduleInterview() {
  const jobs = useJobs()
  const candidates = useCandidates()
  const schedule = useScheduleInterview()
  const navigate = useNavigate()

  const [jobId, setJobId] = useState("")
  const [candidateId, setCandidateId] = useState("")
  const [when, setWhen] = useState("")

  const canSubmit = jobId && candidateId && when && new Date(when) > new Date()

  const submit = async () => {
    try {
      const iv = await schedule.mutateAsync({
        job_id: jobId,
        candidate_id: candidateId,
        scheduled_at: toRFC3339(when),
      })
      toast.success(`Invite emailed to ${iv.candidate.email}`)
      navigate(`/interviews/${iv.id}`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to schedule")
    }
  }

  return (
    <div className="mx-auto max-w-lg">
      <Card>
        <CardHeader>
          <CardTitle>Schedule an interview</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="grid gap-1.5">
            <Label>Job</Label>
            <Select value={jobId} onValueChange={setJobId}>
              <SelectTrigger>
                <SelectValue placeholder="Select a job" />
              </SelectTrigger>
              <SelectContent>
                {jobs.data?.map((j) => (
                  <SelectItem key={j.id} value={j.id}>
                    {j.title}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-1.5">
            <Label>Candidate</Label>
            <Select value={candidateId} onValueChange={setCandidateId}>
              <SelectTrigger>
                <SelectValue placeholder="Select a candidate" />
              </SelectTrigger>
              <SelectContent>
                {candidates.data?.map((c) => (
                  <SelectItem key={c.id} value={c.id}>
                    {c.name} — {c.email}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="when">Scheduled time</Label>
            <Input
              id="when"
              type="datetime-local"
              value={when}
              onChange={(e) => setWhen(e.target.value)}
            />
          </div>
          <Button onClick={submit} disabled={!canSubmit || schedule.isPending}>
            {schedule.isPending ? "Scheduling…" : "Schedule & send invite"}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}

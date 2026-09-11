import { useState } from "react"
import { Link, useParams } from "react-router-dom"
import { toast } from "sonner"

import { useCreateCandidate, useInterviewsByJob, useJob, useScheduleInterview } from "@/api/hooks"
import { PageHeader } from "@/components/PageHeader"
import { StateBadge } from "@/components/StateBadge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { fmtDateTime, toRFC3339 } from "@/lib/format"

export function JobDetail() {
  const { id = "" } = useParams()
  const job = useJob(id)
  const interviews = useInterviewsByJob(id)
  const createCandidate = useCreateCandidate()
  const schedule = useScheduleInterview()

  const [open, setOpen] = useState(false)
  const [email, setEmail] = useState("")
  const [name, setName] = useState("")
  const [resume, setResume] = useState("")
  const [when, setWhen] = useState("")
  const [durationMinutes, setDurationMinutes] = useState(30)

  const canSubmit =
    email.trim() &&
    name.trim() &&
    when &&
    new Date(when) > new Date() &&
    durationMinutes >= 5 &&
    durationMinutes <= 120 &&
    !createCandidate.isPending

  const submit = async () => {
    try {
      const candidate = await createCandidate.mutateAsync({
        email,
        name,
        resume_text: resume,
      })
      await schedule.mutateAsync({
        job_id: id,
        candidate_id: candidate.id,
        scheduled_at: toRFC3339(when),
        duration_minutes: durationMinutes,
      })
      toast.success(`Scheduled — invite emailed to ${candidate.email}`)
      setOpen(false)
      setEmail("")
      setName("")
      setResume("")
      setWhen("")
      setDurationMinutes(30)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to add candidate")
    }
  }

  if (job.isPending) return <Skeleton className="h-64 w-full" />
  if (job.error) return <p className="text-sm text-destructive">{(job.error as Error).message}</p>

  const j = job.data

  return (
    <div className="flex max-w-3xl flex-col gap-6">
      <PageHeader
        title={j.title}
        description={j.description ? j.description : undefined}
      />

      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle>Candidates for this job</CardTitle>
            <Dialog open={open} onOpenChange={setOpen}>
              <DialogTrigger asChild>
                <Button>Add candidate &amp; schedule</Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>New candidate for {j.title}</DialogTitle>
                </DialogHeader>
                <div className="grid gap-1.5">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="name">Name</Label>
                  <Input id="name" value={name} onChange={(e) => setName(e.target.value)} />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="resume">Resume (text)</Label>
                  <textarea
                    id="resume"
                    className="min-h-24 rounded-md border border-input bg-transparent p-2 text-sm"
                    value={resume}
                    onChange={(e) => setResume(e.target.value)}
                  />
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
                <div className="grid gap-1.5">
                  <Label htmlFor="duration">Interview length (minutes)</Label>
                  <Input
                    id="duration"
                    type="number"
                    min={5}
                    max={120}
                    value={durationMinutes}
                    onChange={(e) => setDurationMinutes(Number(e.target.value))}
                  />
                </div>
                <div className="flex justify-end gap-2">
                  <DialogClose asChild>
                    <Button variant="outline">Cancel</Button>
                  </DialogClose>
                  <Button onClick={submit} disabled={!canSubmit}>
                    {createCandidate.isPending || schedule.isPending
                      ? "Scheduling…"
                      : "Create & schedule"}
                  </Button>
                </div>
              </DialogContent>
            </Dialog>
          </div>
        </CardHeader>
        <CardContent>
          {interviews.isPending ? (
            <Skeleton className="h-32 w-full" />
          ) : (interviews.data ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">
              No one scheduled for this job yet — add a candidate above.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Candidate</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Scheduled</TableHead>
                  <TableHead>State</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(interviews.data ?? []).map((iv) => (
                  <TableRow key={iv.id}>
                    <TableCell className="font-medium">
                      <Link to={`/interviews/${iv.id}`} className="hover:underline">
                        {iv.candidate.name}
                      </Link>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{iv.candidate.email}</TableCell>
                    <TableCell className="text-muted-foreground">
                      {fmtDateTime(iv.scheduled_at)}
                    </TableCell>
                    <TableCell>
                      <StateBadge state={iv.state} label={iv.state_label} />
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

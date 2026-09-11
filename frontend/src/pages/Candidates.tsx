import { useState } from "react"
import { toast } from "sonner"

import { useCandidates, useCreateCandidate } from "@/api/hooks"
import { PageHeader } from "@/components/PageHeader"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
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

export function Candidates() {
  const candidates = useCandidates()
  const create = useCreateCandidate()
  const [open, setOpen] = useState(false)
  const [email, setEmail] = useState("")
  const [name, setName] = useState("")
  const [resume, setResume] = useState("")

  const submit = async () => {
    try {
      await create.mutateAsync({ email, name, resume_text: resume })
      toast.success("Candidate added")
      setOpen(false)
      setEmail("")
      setName("")
      setResume("")
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to add candidate")
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Candidates"
        description="Everyone you can schedule for an interview."
        action={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button>New candidate</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>New candidate</DialogTitle>
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
              <div className="flex justify-end gap-2">
                <DialogClose asChild>
                  <Button variant="outline">Cancel</Button>
                </DialogClose>
                <Button
                  onClick={submit}
                  disabled={!email.trim() || !name.trim() || create.isPending}
                >
                  Create
                </Button>
              </div>
            </DialogContent>
          </Dialog>
        }
      />

      <Card>
        <CardContent className="p-5">
          {candidates.isPending ? (
            <Skeleton className="h-40 w-full" />
          ) : (candidates.data ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">No candidates yet.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(candidates.data ?? []).map((c) => (
                  <TableRow key={c.id}>
                    <TableCell className="font-medium">{c.name}</TableCell>
                    <TableCell className="text-muted-foreground">{c.email}</TableCell>
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

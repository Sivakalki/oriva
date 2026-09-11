import { useState } from "react"
import { Link } from "react-router-dom"
import { toast } from "sonner"

import { useCreateJob, useJobs } from "@/api/hooks"
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

export function Jobs() {
  const jobs = useJobs()
  const create = useCreateJob()
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")

  const submit = async () => {
    try {
      await create.mutateAsync({ title, description })
      toast.success("Job created")
      setOpen(false)
      setTitle("")
      setDescription("")
    } catch (e) {
      toast.error(e instanceof Error ? e.message : "Failed to create job")
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Jobs"
        description="Open roles candidates are interviewed against."
        action={
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button>New job</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>New job</DialogTitle>
              </DialogHeader>
              <div className="grid gap-1.5">
                <Label htmlFor="title">Title</Label>
                <Input id="title" value={title} onChange={(e) => setTitle(e.target.value)} />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="desc">Description</Label>
                <textarea
                  id="desc"
                  className="min-h-24 rounded-md border border-input bg-transparent p-2 text-sm"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                />
              </div>
              <div className="flex justify-end gap-2">
                <DialogClose asChild>
                  <Button variant="outline">Cancel</Button>
                </DialogClose>
                <Button onClick={submit} disabled={!title.trim() || create.isPending}>
                  Create
                </Button>
              </div>
            </DialogContent>
          </Dialog>
        }
      />

      <Card>
        <CardContent className="p-5">
          {jobs.isPending ? (
            <Skeleton className="h-40 w-full" />
          ) : (jobs.data ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">No jobs yet.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Title</TableHead>
                  <TableHead>Description</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(jobs.data ?? []).map((j) => (
                  <TableRow key={j.id}>
                    <TableCell className="font-medium">
                      <Link to={`/jobs/${j.id}`} className="hover:underline">
                        {j.title}
                      </Link>
                    </TableCell>
                    <TableCell className="max-w-md truncate text-muted-foreground">
                      {j.description || "—"}
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

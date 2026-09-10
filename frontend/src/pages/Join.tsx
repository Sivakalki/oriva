import type { ReactNode } from "react"
import { useEffect, useState } from "react"
import { Link, useParams } from "react-router-dom"

import { useJoinStatus } from "@/api/hooks"
import type { JoinStatus } from "@/api/types"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { ApiError } from "@/lib/api"

function Frame({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md text-center">{children}</Card>
    </div>
  )
}

function Countdown({ target, serverNow }: { target: string; serverNow: string }) {
  const [skewMs] = useState(() => Date.now() - new Date(serverNow).getTime())
  const [now, setNow] = useState(() => Date.now() - skewMs)
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now() - skewMs), 1000)
    return () => clearInterval(t)
  }, [skewMs])
  const ms = Math.max(0, new Date(target).getTime() - now)
  const h = Math.floor(ms / 3_600_000)
  const m = Math.floor((ms % 3_600_000) / 60_000)
  const s = Math.floor((ms % 60_000) / 1000)
  return <span className="tabular-nums">{h > 0 ? `${h}h ${m}m ${s}s` : `${m}m ${s}s`}</span>
}

function StartButton({ token, muted = false }: { token: string; muted?: boolean }) {
  return (
    <Button className="w-full" variant={muted ? "outline" : "default"} asChild>
      <Link to={`/interview/${token}`}>Start interview</Link>
    </Button>
  )
}

function PhaseView({ s, token }: { s: JoinStatus; token: string }) {
  if (s.phase === "before") {
    return (
      <>
        <CardHeader>
          <CardTitle>Your interview hasn't started yet</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">Role: {s.job_title}</p>
          <p className="text-3xl font-semibold">
            <Countdown target={s.scheduled_at} serverNow={s.server_now} />
          </p>
          <p className="text-xs text-muted-foreground">
            This page will let you in automatically at the scheduled time.
          </p>
        </CardContent>
      </>
    )
  }
  if (s.phase === "open") {
    return (
      <>
        <CardHeader>
          <CardTitle>You're on time</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            Interview for {s.job_title}. You can begin now.
          </p>
          <StartButton token={token} />
        </CardContent>
      </>
    )
  }
  if (s.phase === "late") {
    const mins = Math.max(1, Math.round(s.late_by_seconds / 60))
    return (
      <>
        <CardHeader>
          <CardTitle>
            You're {mins} minute{mins === 1 ? "" : "s"} late
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-sm text-muted-foreground">
            Interview for {s.job_title}. You can still start.
          </p>
          <StartButton token={token} muted />
        </CardContent>
      </>
    )
  }
  return (
    <>
      <CardHeader>
        <CardTitle>This interview has ended</CardTitle>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground">Nothing more to do here.</CardContent>
    </>
  )
}

export function Join() {
  const { token = "" } = useParams()
  const q = useJoinStatus(token)

  if (q.isPending) {
    return (
      <Frame>
        <CardContent className="p-8">
          <Skeleton className="mx-auto h-6 w-48" />
        </CardContent>
      </Frame>
    )
  }

  if (q.error) {
    const msg =
      q.error instanceof ApiError && q.error.status === 404
        ? "This interview link is invalid or has expired."
        : "Something went wrong loading your interview."
    return (
      <Frame>
        <CardContent className="p-8 text-muted-foreground">{msg}</CardContent>
      </Frame>
    )
  }

  return (
    <Frame>
      <PhaseView s={q.data} token={token} />
    </Frame>
  )
}

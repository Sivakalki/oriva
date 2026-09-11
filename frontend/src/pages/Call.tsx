import { Mic, PhoneOff } from "lucide-react"
import { useEffect, useRef } from "react"
import { useParams } from "react-router-dom"

import { useJoinStatus } from "@/api/hooks"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { formatCountdown } from "@/lib/format"
import { cn } from "@/lib/utils"
import { useCall } from "@/pages/call/useCall"
import { useCountdown } from "@/pages/call/useCountdown"

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-lg">{children}</Card>
    </div>
  )
}

export function Call() {
  const { token = "" } = useParams()
  const q = useJoinStatus(token)
  const wsUrl = q.data ? `${q.data.ai_ws_url}?token=${encodeURIComponent(token)}` : ""
  const { state, start, end, cleanup } = useCall(wsUrl)
  const remainingSeconds = useCountdown(q.data?.duration_minutes ?? 30, state.phase === "live")

  const transcriptRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = transcriptRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [state.turns])
  useEffect(() => cleanup, [cleanup])

  if (q.isPending) {
    return (
      <Frame>
        <CardContent className="p-8">
          <Skeleton className="h-6 w-40" />
        </CardContent>
      </Frame>
    )
  }

  if (q.error || !q.data) {
    return (
      <Frame>
        <CardContent className="p-8 text-center text-muted-foreground">
          This interview link is invalid or has expired.
        </CardContent>
      </Frame>
    )
  }

  const s = q.data
  if (s.phase === "closed") {
    return (
      <Frame>
        <CardHeader>
          <CardTitle>This interview has ended</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">Nothing more to do here.</CardContent>
      </Frame>
    )
  }

  if (state.phase === "lobby") {
    return (
      <Frame>
        <CardHeader>
          <CardTitle>Interview for {s.job_title}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-sm text-muted-foreground">
            When you're ready, start the interview. You'll be asked to allow microphone access.
          </p>
          <Button onClick={start}>
            <Mic className="size-4" /> Start interview
          </Button>
          <p className="text-xs text-muted-foreground">
            Note: in this environment you may not hear spoken replies yet — the voice models
            aren't wired up. You'll still see the conversation transcript.
          </p>
        </CardContent>
      </Frame>
    )
  }

  if (state.phase === "error") {
    return (
      <Frame>
        <CardHeader>
          <CardTitle>Couldn't start the interview</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-sm text-destructive">{state.error}</p>
          <Button variant="outline" onClick={start}>
            Try again
          </Button>
        </CardContent>
      </Frame>
    )
  }

  if (state.phase === "ended") {
    return (
      <Frame>
        <CardHeader>
          <CardTitle>Thanks — the interview has ended</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">
          You can close this tab.
        </CardContent>
      </Frame>
    )
  }

  // connecting | live
  const status =
    state.phase === "connecting"
      ? { label: "Connecting…", variant: "muted" as const }
      : state.botSpeaking
        ? { label: "Interviewer speaking", variant: "info" as const }
        : state.userSpeaking
          ? { label: "Listening", variant: "success" as const }
          : { label: "Connected", variant: "default" as const }

  return (
    <Frame>
      <CardHeader className="flex-row items-center justify-between">
        <CardTitle className="text-base">{s.job_title}</CardTitle>
        <div className="flex items-center gap-2">
          <span
            className={cn(
              "font-mono text-sm tabular-nums",
              remainingSeconds <= 60 ? "text-destructive" : "text-muted-foreground",
            )}
            aria-label="Time remaining"
          >
            {formatCountdown(remainingSeconds)}
          </span>
          <Badge variant={status.variant}>{status.label}</Badge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div
          className={cn(
            "mx-auto flex size-24 items-center justify-center rounded-full border-4 transition-colors",
            state.botSpeaking
              ? "animate-pulse border-info bg-info/10"
              : state.userSpeaking
                ? "border-success bg-success/10"
                : "border-muted bg-muted/40",
          )}
        >
          <Mic className="size-8 text-muted-foreground" />
        </div>

        <div
          ref={transcriptRef}
          className="flex max-h-64 flex-col gap-2 overflow-y-auto rounded-md border p-3 text-sm"
        >
          {state.turns.length === 0 ? (
            <p className="text-muted-foreground">The conversation will appear here.</p>
          ) : (
            state.turns.map((t) => (
              <div key={t.id} className={cn(t.role === "bot" ? "text-foreground" : "text-info")}>
                <span className="mr-1 text-xs font-medium uppercase text-muted-foreground">
                  {t.role === "bot" ? "Interviewer" : "You"}
                </span>
                {t.text}
              </div>
            ))
          )}
        </div>

        <Button variant="destructive" onClick={end}>
          <PhoneOff className="size-4" /> End interview
        </Button>
      </CardContent>
    </Frame>
  )
}

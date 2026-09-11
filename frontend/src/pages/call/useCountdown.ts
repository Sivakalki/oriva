import { useEffect, useRef, useState } from "react"

/**
 * Seconds remaining in an interview of `durationMinutes`, counting down from
 * the moment `active` first becomes true (the call connecting, not the
 * scheduled time -- the backend paces/wraps up the interview from the same
 * moment, see ai-service's QueueInterviewer). The backend's own deadline
 * check is what actually ends the call; this is display-only and floors at 0
 * rather than going negative.
 */
export function useCountdown(durationMinutes: number, active: boolean): number {
  const totalSeconds = Math.max(durationMinutes, 0) * 60
  const [elapsed, setElapsed] = useState(0)
  const startedAt = useRef<number | null>(null)

  useEffect(() => {
    if (!active) {
      startedAt.current = null
      return
    }
    startedAt.current = Date.now()
    const start = startedAt.current
    // Reset synchronously so a restart (e.g. "Try again" after an error)
    // doesn't render one frame of the previous run's stale elapsed time.
    setElapsed(0)
    const id = setInterval(() => {
      setElapsed(Math.floor((Date.now() - start) / 1000))
    }, 1000)
    return () => clearInterval(id)
  }, [active])

  if (!active) return totalSeconds
  return Math.max(totalSeconds - elapsed, 0)
}

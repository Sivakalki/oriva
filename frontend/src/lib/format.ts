export function fmtDateTime(iso: string): string {
  return new Date(iso).toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  })
}

export function toRFC3339(localValue: string): string {
  // <input type="datetime-local"> gives "YYYY-MM-DDTHH:mm" in local time.
  return new Date(localValue).toISOString().replace(/\.\d{3}Z$/, "Z")
}

/** "MM:SS", or "H:MM:SS" once an hour or more remains (duration caps at 120 min). */
export function formatCountdown(totalSeconds: number): string {
  const s = Math.max(Math.round(totalSeconds), 0)
  const hours = Math.floor(s / 3600)
  const minutes = Math.floor((s % 3600) / 60)
  const seconds = s % 60
  const mm = hours > 0 ? String(minutes).padStart(2, "0") : String(minutes)
  const ss = String(seconds).padStart(2, "0")
  return hours > 0 ? `${hours}:${mm}:${ss}` : `${mm}:${ss}`
}

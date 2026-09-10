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

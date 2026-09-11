import { describe, expect, it } from "vitest"

import { formatCountdown } from "@/lib/format"

describe("formatCountdown", () => {
  it("formats under a minute as 0:SS", () => {
    expect(formatCountdown(45)).toBe("0:45")
  })

  it("formats minutes and seconds as M:SS", () => {
    expect(formatCountdown(5 * 60 + 9)).toBe("5:09")
  })

  it("formats an hour or more as H:MM:SS", () => {
    expect(formatCountdown(60 * 60 + 3 * 60 + 7)).toBe("1:03:07")
  })

  it("floors at 0 rather than going negative", () => {
    expect(formatCountdown(-42)).toBe("0:00")
  })

  it("rounds fractional seconds", () => {
    expect(formatCountdown(59.6)).toBe("1:00")
  })
})

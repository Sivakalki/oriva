import { act, renderHook } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { useCountdown } from "@/pages/call/useCountdown"

describe("useCountdown", () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it("shows the full duration while inactive", () => {
    const { result } = renderHook(() => useCountdown(5, false))
    expect(result.current).toBe(5 * 60)
  })

  it("counts down once active, one tick per second", () => {
    const { result, rerender } = renderHook(({ active }) => useCountdown(5, active), {
      initialProps: { active: false },
    })
    rerender({ active: true })
    expect(result.current).toBe(5 * 60)

    act(() => vi.advanceTimersByTime(3000))
    expect(result.current).toBe(5 * 60 - 3)
  })

  it("floors at 0 once the duration elapses", () => {
    const { result, rerender } = renderHook(({ active }) => useCountdown(1, active), {
      initialProps: { active: false },
    })
    rerender({ active: true })

    act(() => vi.advanceTimersByTime(90_000))
    expect(result.current).toBe(0)
  })

  it("resets to the full duration when it goes inactive again", () => {
    const { result, rerender } = renderHook(({ active }) => useCountdown(2, active), {
      initialProps: { active: false },
    })
    rerender({ active: true })
    act(() => vi.advanceTimersByTime(10_000))
    expect(result.current).toBe(2 * 60 - 10)

    rerender({ active: false })
    expect(result.current).toBe(2 * 60)
  })

  it("restarts from the full duration on a fresh activation", () => {
    const { result, rerender } = renderHook(({ active }) => useCountdown(2, active), {
      initialProps: { active: false },
    })
    rerender({ active: true })
    act(() => vi.advanceTimersByTime(30_000))
    rerender({ active: false })
    rerender({ active: true })

    expect(result.current).toBe(2 * 60) // no stale flash from the previous run
  })
})

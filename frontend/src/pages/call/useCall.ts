import { PipecatClient } from "@pipecat-ai/client-js"
import { WebSocketTransport } from "@pipecat-ai/websocket-transport"
import { useCallback, useRef, useState } from "react"

export type CallPhase = "lobby" | "connecting" | "live" | "ended" | "error"

export interface Turn {
  id: number
  role: "user" | "bot"
  text: string
}

export interface CallState {
  phase: CallPhase
  error: string | null
  botSpeaking: boolean
  userSpeaking: boolean
  turns: Turn[]
}

const INITIAL: CallState = {
  phase: "lobby",
  error: null,
  botSpeaking: false,
  userSpeaking: false,
  turns: [],
}

export function useCall(wsUrl: string) {
  const [state, setState] = useState<CallState>(INITIAL)
  const clientRef = useRef<PipecatClient | null>(null)
  const turnId = useRef(0)
  const userDraft = useRef("")

  const patch = useCallback((p: Partial<CallState>) => setState((s) => ({ ...s, ...p })), [])

  const addTurn = useCallback((role: Turn["role"], text: string) => {
    setState((s) => ({ ...s, turns: [...s.turns, { id: ++turnId.current, role, text }] }))
  }, [])

  const start = useCallback(async () => {
    patch({ phase: "connecting", error: null })
    try {
      await navigator.mediaDevices.getUserMedia({ audio: true })
    } catch {
      patch({ phase: "error", error: "Microphone access is required to start the interview." })
      return
    }

    const client = new PipecatClient({
      transport: new WebSocketTransport({ wsUrl }),
      enableMic: true,
      enableCam: false,
      callbacks: {
        onConnected: () => patch({ phase: "live" }),
        onDisconnected: () => patch({ phase: "ended" }),
        onError: (msg) => {
          const d = msg.data
          const text =
            d && typeof d === "object" && "message" in d
              ? String((d as { message: unknown }).message)
              : "The interview connection failed."
          patch({ phase: "error", error: text })
        },
        onUserStartedSpeaking: () => patch({ userSpeaking: true }),
        onUserStoppedSpeaking: () => patch({ userSpeaking: false }),
        onBotStartedSpeaking: () => patch({ botSpeaking: true }),
        onBotStoppedSpeaking: () => patch({ botSpeaking: false }),
        onUserTranscript: (d) => {
          if (d.final) {
            userDraft.current = ""
            if (d.text.trim()) addTurn("user", d.text)
          } else {
            userDraft.current = d.text
          }
        },
        onBotTranscript: (d) => {
          if (d.text.trim()) addTurn("bot", d.text)
        },
      },
    })
    clientRef.current = client

    try {
      await client.connect()
    } catch (e) {
      patch({
        phase: "error",
        error: e instanceof Error ? e.message : "Could not connect to the interview.",
      })
    }
  }, [wsUrl, patch, addTurn])

  const end = useCallback(async () => {
    await clientRef.current?.disconnect().catch(() => {})
    patch({ phase: "ended" })
  }, [patch])

  const cleanup = useCallback(() => {
    void clientRef.current?.disconnect().catch(() => {})
    clientRef.current = null
  }, [])

  return { state, start, end, cleanup }
}

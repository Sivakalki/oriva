import { apiFetch } from "@/lib/api"

import type {
  Candidate,
  InterviewDetail,
  Job,
  JoinStatus,
  ListResponse,
  LoginResult,
  Role,
  StateGraph,
} from "./types"

// --- auth ---
export const login = (email: string, password: string) =>
  apiFetch<LoginResult>("/auth/login", { method: "POST", auth: false, json: { email, password } })

export const me = () =>
  apiFetch<{ id: string; org_id: string; role: Role }>("/auth/me")

// --- jobs ---
export const listJobs = () => apiFetch<ListResponse<Job>>("/jobs").then((r) => r.data)
export const createJob = (title: string, description: string) =>
  apiFetch<Job>("/jobs", { method: "POST", json: { title, description } })

// --- candidates ---
export const listCandidates = () =>
  apiFetch<ListResponse<Candidate>>("/candidates").then((r) => r.data)
export const createCandidate = (email: string, name: string, resume_text: string) =>
  apiFetch<Candidate>("/candidates", { method: "POST", json: { email, name, resume_text } })

// --- interviews ---
export const listInterviews = (state?: string) =>
  apiFetch<ListResponse<InterviewDetail>>(
    `/interviews${state ? `?state=${encodeURIComponent(state)}` : ""}`,
  ).then((r) => r.data)

export const getInterview = (id: string) => apiFetch<InterviewDetail>(`/interviews/${id}`)

export const scheduleInterview = (input: {
  job_id: string
  candidate_id: string
  scheduled_at: string
}) => apiFetch<InterviewDetail>("/interviews", { method: "POST", json: input })

export const advanceState = (id: string, to_state: string, reason: string) =>
  apiFetch<InterviewDetail>(`/interviews/${id}/advance`, {
    method: "POST",
    json: { to_state, reason },
  })

// --- session state graph ---
export const sessionStates = () => apiFetch<StateGraph>("/session-states")

// --- candidate (public) ---
export const joinStatus = (token: string) =>
  apiFetch<JoinStatus>(`/join/${encodeURIComponent(token)}`, { auth: false })

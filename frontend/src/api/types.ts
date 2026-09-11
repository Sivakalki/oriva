export type Role = "scheduler" | "candidate"

export interface LoginResult {
  access_token: string
  token_type: string
  expires_in: number
  user: { id: string; email: string; role: Role; org_id: string }
}

export interface Job {
  id: string
  org_id: string
  title: string
  description: string
  created_at: string
  updated_at: string
}

export interface Candidate {
  id: string
  org_id: string
  email: string
  name: string
  resume_text: string
  created_at: string
  updated_at: string
}

export interface JobRef {
  id: string
  title: string
}
export interface CandRef {
  id: string
  name: string
  email: string
}

export interface TurnScore {
  turn_index: number
  question: string
  answer: string
  value: number
  rationale: string
}

export interface ScoreSummary {
  value: number
  rationale: string
  model: string
  scored_at: string
}

export interface InterviewDetail {
  id: string
  state: string
  state_label: string
  scheduled_at: string
  job: JobRef
  candidate: CandRef
  created_at: string
  join_token?: string
  join_url?: string
  // Recruiter-only: GET /interviews/{id} sits behind the scheduler role.
  overall_score?: ScoreSummary
  turn_scores?: TurnScore[]
}

export interface StateGraph {
  states: { name: string; label: string; is_terminal: boolean }[]
  transitions: { from: string; to: string }[]
}

export type JoinPhase = "before" | "open" | "late" | "closed"

export interface JoinStatus {
  job_title: string
  scheduled_at: string
  server_now: string
  phase: JoinPhase
  late_by_seconds: number
  session_id: string
  ai_ws_url: string
}

export interface ListResponse<T> {
  data: T[]
}

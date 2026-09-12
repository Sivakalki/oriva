# AI Interview Platform — Project Context

## What this is
A platform where a recruiter schedules a **live, AI-conducted phone interview** with a candidate. The AI asks questions generated from the job description + candidate resume, and asks real-time interactive follow-ups based on the candidate's spoken answers. Version 1 scope is the live phone-call interview, not async/recorded.

## Core user flow
1. Recruiter (role: `scheduler`) adds a candidate profile and schedules an interview by attaching the candidate's email + a job description.
2. Candidate receives an email and joins the interview call.
3. The AI conducts the interview: asks JD/resume-informed questions, listens in real time, and asks interactive follow-ups based on the answer just given.
4. Interview is scored against a rubric and surfaced to the recruiter afterward.

## Tech stack
- **Frontend:** React
- **Backend (business logic):** Go — auth, scheduling, candidate/recruiter CRUD, session state, interview persistence
- **AI service:** Python — owns the real-time voice pipeline (STT, LLM orchestration, TTS), built on Pipecat
- **Database:** Postgres (jobs/candidates/interviews, session-state tables). No pgvector —
  job description and resume are short text, fetched in full and used as-is; no corpus to
  search, so a retrieval/embedding layer was evaluated and dropped.
- **Auth:** JWT, two roles — `scheduler` and `candidate`

## Architecture decisions (see docs/ARCHITECTURE.md for full detail)
- **Cascaded pipeline (STT → LLM → TTS), not speech-to-speech.** Chosen for auditable transcripts, component-level rubric scoring, and compliance.
- **Orchestrator:** Pipecat (Python, open source).
- **Tool boundary (Go ↔ Python):** MCP over Streamable HTTP. Go implements the MCP server and owns the tool schemas (`get_interview_plan`, `record_turn`, `advance_state`); Python is the only consumer, with no direct database access of its own.
- **Frontend ↔ Go:** plain REST + JWT. No LLM or tool concept involved — this is ordinary CRUD (auth, scheduling, listing candidates/interviews).
- **LLM, STT, TTS are all pluggable**, not locked to a specific vendor. LLM calls route through a proxy (self-hosted LiteLLM for dev, OpenRouter as a hosted alternative later) so the model is a config value, not a code dependency. Any LLM candidate must have solid function/tool-calling support before it's compared on latency, since the MCP tool boundary depends on it.
- **Session state:** 12 canonical states, stored as Postgres data (not generated code). See docs/ARCHITECTURE.md for the full state list and drift-prevention approach.
- **Latency target:** ~800ms median voice-to-voice for the live pipeline (loosen to ~1,500ms acceptable for early prototype).

## Current phase: experimentation, not full build
We have **not yet locked** the specific STT/LLM/TTS vendor combo. See docs/PLAN.md for the bake-off sequence before the real interview logic gets built.

**Dev-mode constraint: prefer free/open-source resources over paid ones** while experimenting. Save Twilio, paid TTS tiers, and production-tier STT for after a combo wins the bake-off.

## What's safe to build now, in parallel (doesn't depend on the AI stack winner)
- Go backend: JWT auth (scheduler/candidate roles), interview scheduling, candidate/recruiter CRUD, session state machine.
- React frontend shell: recruiter dashboard, candidate scheduling flow, basic call UI shell.
- Postgres schema: organizations → jobs → candidates → interview_sessions → responses → scores (draft, refine as AI service solidifies).

## What's NOT decided yet — don't assume
- Final STT/LLM/TTS vendor choice (pending bake-off results).
- Production telephony provider details beyond "likely Twilio."
- Final scoring rubric schema.
- Deployment target (explicitly deferred).

## Repo structure
Keep the Go and Python services in separate top-level directories (`/backend-go`, `/ai-service-python`) so each can be worked on independently.

## Non-goals for v1
- Async/recorded interview mode (may come later, not now).
- Multi-language support (English only for v1).
- Self-serve org onboarding — assume manually provisioned orgs for now.

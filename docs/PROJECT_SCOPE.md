# Project scope — AI interview platform

## Product
A recruiter schedules a live, AI-conducted phone interview with a candidate. The AI asks
questions derived from the job description and the candidate's resume, and asks real-time
interactive follow-up questions based on what the candidate actually says — not a fixed
script.

## v1 scope: live phone-call interview

### User roles
- **Scheduler** (recruiter): adds candidate profiles, attaches a job description, schedules
  the interview against a candidate email.
- **Candidate**: receives an email invite, joins the call at the scheduled time.

### Core flow
1. Scheduler adds a candidate profile and job description, schedules an interview.
2. Candidate receives an email invite and joins the call.
3. The AI conducts the interview live:
   - Opens with questions generated from the JD + resume.
   - Listens to each answer in real time.
   - Generates interactive follow-up questions based on the specific answer just given,
     not a pre-written list.
4. After the call, responses are scored against a rubric.
5. Scheduler reviews the transcript, score, and rationale.

### What "interactive" means concretely
- The AI must be able to interrupt/handle interruption (barge-in) like a real interviewer.
- Follow-ups are generated dynamically per-answer, informed by the JD, the resume, and the
  running conversation history — not selected from a static question bank.
- The AI should probe gaps between the resume's claims and the JD's requirements.

## Non-goals for v1
- **Async/recorded interviews.** May be a future mode; not built now.
- **Multi-language support.** English only for v1.
- **Self-serve org onboarding.** Orgs are provisioned manually for now.
- **Deployment target.** Explicitly deferred until after the AI-stack bake-off.

## Compliance considerations (carried into scoring/rubric design later)
Automated hiring tools are subject to bias-audit and disclosure requirements in some
jurisdictions (e.g. NYC Local Law 144) and to disparate-impact liability under employment
law generally (e.g. Title VII in the US). This is why the architecture requires an
auditable text transcript and per-question scoring rationale rather than an opaque
end-to-end score — see docs/ARCHITECTURE.md.

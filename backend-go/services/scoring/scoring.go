// Package scoring judges candidate answers with an LLM: one score per turn
// as the interview happens, then one overall score once every turn is in
// (docs/PLAN.md Phase 2).
package scoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	"oriva/backend-go/models/interview"
	"oriva/backend-go/repositories/llmjudge"
	"oriva/backend-go/repositories/postgres/interview_repo"
	"oriva/backend-go/repositories/postgres/score_repo"

	"go.uber.org/zap"
)

// Consumed contracts.
type scoreRepo interface {
	RecordTurnScore(ctx context.Context, sessionID string, turnIndex int, value float64, rationale, model string) error
	RecordOverallScore(ctx context.Context, sessionID string, value float64, rationale, model string) error
	TurnScores(ctx context.Context, sessionID string) ([]score_repo.TurnScore, error)
}

type planReader interface {
	PlanData(ctx context.Context, orgID, sessionID string) (*interview_repo.PlanData, error)
}

type stateAdvancer interface {
	Advance(ctx context.Context, orgID, sessionID, toState, reason, actor string) (*interview.Detail, error)
}

// Service scores interview turns and the overall interview via an LLM judge.
type Service struct {
	scores   scoreRepo
	plans    planReader
	judge    llmjudge.Client
	sessions stateAdvancer
	logger   *zap.Logger
}

// NewService constructs a scoring Service.
func NewService(
	scores scoreRepo, plans planReader, judge llmjudge.Client, sessions stateAdvancer, logger *zap.Logger,
) *Service {
	return &Service{scores: scores, plans: plans, judge: judge, sessions: sessions, logger: logger}
}

const turnSystemPrompt = `You are an expert technical interviewer grading one answer from a live interview.
Score the candidate's answer on a 0-100 scale for relevance to the question, technical depth, and clarity.
Be specific and fair -- do not default to a middle score. Respond with ONLY a JSON object, no other text:
{"score": <0-100 number>, "reasoning": "<one or two sentence explanation>"}`

const overallSystemPrompt = `You are an expert technical interviewer producing the final verdict for a completed interview.
You will see every turn already scored individually (question, answer, per-turn score, reasoning).
Weigh all turns together and produce one overall 0-100 score for the candidate's performance, with a short written
summary justifying it (strengths, weaknesses, and whether they seem right for the role). Respond with ONLY a JSON
object, no other text: {"score": <0-100 number>, "reasoning": "<summary, 3-5 sentences>"}`

// ScoreTurn scores one Q&A turn against the job's requirements. Meant to be
// called as `go scoring.ScoreTurn(...)` right after the turn is recorded
// (mcp/tools.go), so a judge-LLM round trip never adds latency to the live
// conversation. There's no caller left to hand an error to, so failures are
// logged and counted (oriva_scoring_errors_total), not returned.
func (s *Service) ScoreTurn(ctx context.Context, orgID, sessionID string, turnIndex int, question, answer string) {
	start := time.Now()
	plan, err := s.plans.PlanData(ctx, orgID, sessionID)
	if err != nil {
		scoringErrors.WithLabelValues("turn").Inc()
		s.logger.Warn("score turn: plan lookup failed",
			zap.String("session_id", sessionID), zap.Error(err))
		return
	}

	user := fmt.Sprintf(
		"Job: %s\nJob description: %s\n\nQuestion: %s\nCandidate's answer: %s",
		plan.JobTitle, plan.JobDescription, question, answer,
	)
	res, err := s.judge.Score(ctx, turnSystemPrompt, user)
	scoringDuration.WithLabelValues("turn").Observe(time.Since(start).Seconds())
	if err != nil {
		scoringErrors.WithLabelValues("turn").Inc()
		s.logger.Warn("score turn: judge call failed",
			zap.String("session_id", sessionID), zap.Int("turn_index", turnIndex), zap.Error(err))
		return
	}

	value := clamp(res.Value)
	turnScoreHist.Observe(value)
	if err := s.scores.RecordTurnScore(ctx, sessionID, turnIndex, value, res.Rationale, res.Model); err != nil {
		scoringErrors.WithLabelValues("turn").Inc()
		s.logger.Warn("score turn: store failed",
			zap.String("session_id", sessionID), zap.Int("turn_index", turnIndex), zap.Error(err))
	}
}

// ScoreOverall aggregates every recorded turn score into one overall
// interview score, stores it, then advances the session scoring -> scored.
// Safe to call more than once for the same session (RecordOverallScore
// upserts, and re-advancing an already-scored session is a no-op error the
// caller can ignore).
func (s *Service) ScoreOverall(ctx context.Context, orgID, sessionID string) error {
	start := time.Now()
	turns, err := s.scores.TurnScores(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load turn scores: %w", err)
	}
	if len(turns) == 0 {
		return fmt.Errorf("no scored turns for session %s yet", sessionID)
	}

	var sb strings.Builder
	for _, t := range turns {
		fmt.Fprintf(&sb, "Turn %d (score %.0f/100):\nQ: %s\nA: %s\nReasoning: %s\n\n",
			t.TurnIndex, t.Value, t.Question, t.Answer, t.Rationale)
	}

	res, err := s.judge.Score(ctx, overallSystemPrompt, sb.String())
	scoringDuration.WithLabelValues("overall").Observe(time.Since(start).Seconds())
	if err != nil {
		scoringErrors.WithLabelValues("overall").Inc()
		return fmt.Errorf("judge call: %w", err)
	}

	value := clamp(res.Value)
	overallScoreHist.Observe(value)
	if err := s.scores.RecordOverallScore(ctx, sessionID, value, res.Rationale, res.Model); err != nil {
		scoringErrors.WithLabelValues("overall").Inc()
		return fmt.Errorf("store overall score: %w", err)
	}

	if _, err := s.sessions.Advance(ctx, orgID, sessionID, "scored", "overall score computed", "scoring-service"); err != nil {
		return fmt.Errorf("advance to scored: %w", err)
	}
	return nil
}

func clamp(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 100:
		return 100
	default:
		return v
	}
}

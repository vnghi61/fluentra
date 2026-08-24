package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/shared/apperr"
)

func TestNormalizeSkill(t *testing.T) {
	cases := []struct {
		input    string
		wantNorm string
		wantOK   bool
	}{
		{"vocabulary", "vocabulary", true},
		{"GRAMMAR", "grammar", true},
		{"  Reading  ", "reading", true},
		{"listening", "listening", true},
		{"Speaking", "speaking", true},
		{"writing", "writing", true},
		{"unknown", "", false},
		{"", "", false},
		{"math", "", false},
	}

	for _, tc := range cases {
		norm, ok := domain.NormalizeSkill(tc.input)
		if ok != tc.wantOK {
			t.Errorf("NormalizeSkill(%q) ok = %v, want %v", tc.input, ok, tc.wantOK)
		}
		if norm != tc.wantNorm {
			t.Errorf("NormalizeSkill(%q) norm = %q, want %q", tc.input, norm, tc.wantNorm)
		}
	}
}

func TestEstimateMastery_EWMA(t *testing.T) {
	// 1. Initial attempt with score 90 -> Level C1, confidence 0.20
	level, conf, est := domain.EstimateMastery(nil, 90.0)
	if level != domain.LevelC1 {
		t.Errorf("got level %s, want C1", level)
	}
	if conf != 0.20 {
		t.Errorf("got confidence %v, want 0.20", conf)
	}
	if est != 90.0 {
		t.Errorf("got estimate %v, want 90.0", est)
	}

	// 2. Second attempt: score 100 on existing C1 (ScoreFromCEFR = 90.0)
	// est = (0.35 * 100) + (0.65 * 90) = 35 + 58.5 = 93.5 -> C1
	// conf = 0.20 + 0.10 = 0.30
	existing := &domain.SkillMastery{
		Level:      domain.LevelC1,
		Confidence: 0.20,
	}
	level, conf, est = domain.EstimateMastery(existing, 100.0)
	if level != domain.LevelC1 {
		t.Errorf("got level %s, want C1", level)
	}
	if conf != 0.30 {
		t.Errorf("got confidence %v, want 0.30", conf)
	}
	if est < 93.4 || est > 93.6 {
		t.Errorf("got estimate %v, want ~93.5", est)
	}

	// 3. Confidence caps at 1.00
	existingMax := &domain.SkillMastery{
		Level:      domain.LevelC2,
		Confidence: 0.95,
	}
	_, conf, _ = domain.EstimateMastery(existingMax, 100.0)
	if conf != 1.00 {
		t.Errorf("got confidence %v, want 1.00", conf)
	}
}

func TestCEFRFromScore(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.0, domain.LevelA1},
		{25.0, domain.LevelA1},
		{35.0, domain.LevelA2},
		{55.0, domain.LevelB1},
		{75.0, domain.LevelB2},
		{90.0, domain.LevelC1},
		{98.0, domain.LevelC2},
	}

	for _, tt := range tests {
		got := domain.CEFRFromScore(tt.score)
		if got != tt.want {
			t.Errorf("CEFRFromScore(%v) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestScoreFromCEFR(t *testing.T) {
	levels := []string{
		domain.LevelA1,
		domain.LevelA2,
		domain.LevelB1,
		domain.LevelB2,
		domain.LevelC1,
		domain.LevelC2,
		"UNKNOWN",
	}

	for _, l := range levels {
		score := domain.ScoreFromCEFR(l)
		if score <= 0 {
			t.Errorf("ScoreFromCEFR(%s) should be positive, got %v", l, score)
		}
	}
}

func TestEnrollment_Predicates(t *testing.T) {
	e := &domain.Enrollment{
		Status: domain.StatusEnrollmentActive,
	}
	if !e.IsActive() {
		t.Errorf("expected IsActive to be true")
	}
	if e.IsCompleted() {
		t.Errorf("expected IsCompleted to be false")
	}

	e.Status = domain.StatusEnrollmentCompleted
	if e.IsActive() {
		t.Errorf("expected IsActive to be false")
	}
	if !e.IsCompleted() {
		t.Errorf("expected IsCompleted to be true")
	}
}

func TestSession_Predicates(t *testing.T) {
	s := &domain.LearningSession{
		ID:        uuid.New(),
		StartedAt: time.Now(),
	}
	if s.IsCompleted() {
		t.Errorf("session without ended_at should not be completed")
	}

	now := time.Now()
	s.EndedAt = &now
	if !s.IsCompleted() {
		t.Errorf("session with ended_at should be completed")
	}
}

func TestErrorPredicates(t *testing.T) {
	if !domain.IsAlreadyEnrolled(domain.ErrAlreadyEnrolled) {
		t.Errorf("IsAlreadyEnrolled failed on ErrAlreadyEnrolled")
	}
	if domain.IsAlreadyEnrolled(errors.New("other")) {
		t.Errorf("IsAlreadyEnrolled false positive")
	}

	if !domain.IsLessonLocked(domain.ErrLessonLocked) {
		t.Errorf("IsLessonLocked failed on ErrLessonLocked")
	}
	if domain.IsLessonLocked(errors.New("other")) {
		t.Errorf("IsLessonLocked false positive")
	}

	if !domain.IsNotEnrolled(domain.ErrNotEnrolled) {
		t.Errorf("IsNotEnrolled failed on ErrNotEnrolled")
	}
	if domain.IsNotEnrolled(errors.New("other")) {
		t.Errorf("IsNotEnrolled false positive")
	}

	if !domain.IsSessionNotFound(domain.ErrSessionNotFound) {
		t.Errorf("IsSessionNotFound failed on ErrSessionNotFound")
	}
	if domain.IsSessionNotFound(apperr.New(apperr.NotFound, "OTHER_NOT_FOUND", "other")) {
		t.Errorf("IsSessionNotFound false positive")
	}
}

// TestEstimateMastery_RecentPerformanceDominates is BR-LEARNING-09's actual
// claim: "not a raw average — recent performance must dominate". Ten failures
// followed by three strong attempts average to 23, which is A1. If the estimator
// ever becomes a mean, this is the test that says so.
func TestEstimateMastery_RecentPerformanceDominates(t *testing.T) {
	scores := make([]float64, 0, 13)
	for i := 0; i < 10; i++ {
		scores = append(scores, 0)
	}
	for i := 0; i < 3; i++ {
		scores = append(scores, 100)
	}

	var current *domain.SkillMastery
	for _, score := range scores {
		level, confidence, _ := domain.EstimateMastery(current, score)
		current = &domain.SkillMastery{Level: level, Confidence: confidence}
	}

	var mean float64
	for _, score := range scores {
		mean += score
	}
	mean /= float64(len(scores))
	if meanLevel := domain.CEFRFromScore(mean); meanLevel != domain.LevelA1 {
		t.Fatalf("fixture no longer discriminates: the mean %.1f maps to %s, not A1", mean, meanLevel)
	}

	if domain.ScoreFromCEFR(current.Level) <= domain.ScoreFromCEFR(domain.LevelA2) {
		t.Errorf("got level %s after three strong attempts; a running average would say A1", current.Level)
	}
	if current.Confidence <= 0.20 {
		t.Errorf("got confidence %v; it should have grown over 13 attempts", current.Confidence)
	}
}

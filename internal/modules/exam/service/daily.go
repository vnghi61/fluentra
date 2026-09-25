package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
)

// DailyGenerationMargin is how many extra groups beyond one test's worth the
// daily job asks for, so a verification doubt does not leave the next test
// short (WO 22 Stage O).
const DailyGenerationMargin = 1

// DailyExamRotation is the exams the daily job generates for, in order. The day
// of the year picks one, so each exam gains a test every three days.
var DailyExamRotation = []string{
	"TOEIC_LR_2026",
	"IELTS_ACADEMIC_2026_R2",
	"VSTEP_3_5",
}

// DailyExamForDay picks the exam a given day generates for.
func DailyExamForDay(day time.Time) string {
	index := day.YearDay() % len(DailyExamRotation)
	if index < 0 {
		index += len(DailyExamRotation)
	}
	return DailyExamRotation[index]
}

// GenerateDaily is the daily job: generate one test's worth for the day's exam
// through the bank author, then compose the next numbered test the published
// bank can fill. A day with no bank author is a no-op, so the job is safe to
// register while the switch is off.
func (s *Service) GenerateDaily(ctx context.Context) error {
	if s.bankAuthor == nil {
		return nil
	}
	_, err := s.GenerateDailyExam(ctx, DailyExamForDay(s.clock.Now().UTC()))
	return err
}

// GenerateDailyExam generates one test's worth (plus a margin) for every part
// of a version, then composes the next numbered tests. It returns how many
// fixed tests it composed.
func (s *Service) GenerateDailyExam(ctx context.Context, versionCode string) (int, error) {
	if s.repo == nil {
		return 0, errors.New("repository not configured")
	}
	if s.bankAuthor == nil {
		return 0, errors.New("no bank author is configured, so nothing can be generated")
	}

	version, err := s.repo.GetExamVersionByCode(ctx, versionCode)
	if err != nil || version == nil {
		return 0, fmt.Errorf("exam version %q not found", versionCode)
	}
	parts, err := s.repo.ListExamPartsByVersionID(ctx, version.ID)
	if err != nil {
		return 0, fmt.Errorf("list parts for %s: %w", versionCode, err)
	}
	if len(parts) == 0 {
		return 0, fmt.Errorf("exam version %s has no parts to generate for", versionCode)
	}
	blueprints, err := s.repo.ListBlueprintsByVersionID(ctx, version.ID)
	if err != nil {
		return 0, fmt.Errorf("list blueprints for %s: %w", versionCode, err)
	}
	level := blueprintLevel(blueprints)

	for _, part := range parts {
		groups := groupsPerTest(part) + DailyGenerationMargin
		partID := part.ID
		_, genErr := s.bankAuthor.GenerateQuestions(ctx, questionbankcontract.GenerateRequest{
			ExamVersion: &versionCode,
			ExamPartID:  &partID,
			Kind:        part.Kind,
			Skill:       part.Section,
			CEFRLevel:   level,
			Count:       groups,
		})
		if genErr != nil {
			return 0, fmt.Errorf("generate %s part %d: %w", versionCode, part.PartNumber, genErr)
		}
	}

	composed := 0
	for _, blueprint := range blueprints {
		count, composeErr := s.ComposeNextFixedTests(ctx, blueprint.ID)
		if composeErr != nil {
			return composed, composeErr
		}
		composed += count
	}
	return composed, nil
}

// blueprintLevel is the level most of a blueprint's items should be written at:
// the largest share in its CEFR distribution, or B2 when it states none.
func blueprintLevel(blueprints []*domain.Blueprint) string {
	const fallback = "B2"
	if len(blueprints) == 0 {
		return fallback
	}
	mix := parseCEFRMix(blueprints[0].CefrDistribution)
	best, bestShare := fallback, 0.0
	for level, share := range mix {
		if share > bestShare {
			best, bestShare = level, share
		}
	}
	return best
}

// ComposeAllFixedTests composes the next numbered fixed tests of every exam in
// the rotation that its published bank can fill (WO 22 Stage O).
//
// The daily job composes right after it generates, but most questions reach the
// bank later: a batch a person approves, or a verifier's publish consumed after
// the job returned. This sweep is what makes Test N+1 appear the day a full
// disjoint test's worth exists, whoever published the last question. One exam
// failing does not stop the others.
func (s *Service) ComposeAllFixedTests(ctx context.Context) error {
	if s.repo == nil {
		return errors.New("repository not configured")
	}
	var firstErr error
	for _, code := range DailyExamRotation {
		version, err := s.repo.GetExamVersionByCode(ctx, code)
		if err != nil || version == nil {
			continue
		}
		blueprints, err := s.repo.ListBlueprintsByVersionID(ctx, version.ID)
		if err != nil {
			firstErr = errors.Join(firstErr, fmt.Errorf("list blueprints for %s: %w", code, err))
			continue
		}
		for _, blueprint := range blueprints {
			count, err := s.ComposeNextFixedTests(ctx, blueprint.ID)
			if err != nil {
				firstErr = errors.Join(firstErr, fmt.Errorf("compose fixed tests for %s: %w", code, err))
				continue
			}
			if count > 0 {
				slog.InfoContext(ctx, "exam: composed fixed tests", "exam", code, "count", count)
			}
		}
	}
	return firstErr
}

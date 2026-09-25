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
	code := DailyExamForDay(s.clock.Now().UTC())
	if skip, err := s.reviewBacklogged(ctx, code); err != nil {
		return err
	} else if skip {
		slog.InfoContext(ctx, "exam: daily generation skipped; earlier doubts wait for review", "exam", code)
		return nil
	}
	_, err := s.GenerateDailyExam(ctx, code)
	return err
}

// backlogDays is how many earlier generation days' doubts may wait before the
// daily job stops adding to them (WO 22 Stage O trap 1).
const backlogDays = 2

// reviewBacklogged reports an exam whose escalated batches from two earlier
// days nobody has reviewed. Days the verifier confirms everything leave no
// batch behind, so they never trip it.
func (s *Service) reviewBacklogged(ctx context.Context, versionCode string) (bool, error) {
	if s.backlog == nil {
		return false, nil
	}
	days, err := s.backlog.PendingBatchDays(ctx, questionbankcontract.BatchPrefix(versionCode))
	if err != nil {
		return false, fmt.Errorf("read the review backlog for %s: %w", versionCode, err)
	}
	return days >= backlogDays, nil
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

	asked := 0
	for _, part := range parts {
		groups := groupsPerTest(part) + DailyGenerationMargin
		// The per-run cap bounds spend: parts after it wait for the next run.
		if s.dailyCap > 0 {
			groups = min(groups, s.dailyCap-asked)
			if groups <= 0 {
				break
			}
		}
		generated, genErr := s.generatePart(ctx, versionCode, part, level, groups)
		if genErr != nil {
			return 0, fmt.Errorf("generate %s part %d: %w", versionCode, part.PartNumber, genErr)
		}
		asked += generated
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

// generatePart asks the bank author for one part's groups and returns how many
// it asked for. A photo part is written one photograph at a time, from the
// photographs the source has left; with none, the part waits and says so.
func (s *Service) generatePart(
	ctx context.Context, versionCode string, part *domain.ExamPart, level string, groups int,
) (int, error) {
	partID := part.ID
	req := questionbankcontract.GenerateRequest{
		ExamVersion: &versionCode,
		ExamPartID:  &partID,
		Kind:        part.Kind,
		Skill:       part.Section,
		CEFRLevel:   level,
		// The generator tags every item to a spine node and refuses a request
		// with none; an exam item is tagged to its section's skill.
		NodeCodes: []string{sectionNode(part.Section)},
		Count:     groups,
	}
	if part.Kind != kindPhotoDescription {
		_, err := s.bankAuthor.GenerateQuestions(ctx, req)
		return groups, err
	}

	var photos []questionbankcontract.Photo
	if s.photos != nil {
		var err error
		if photos, err = s.photos.NextPhotos(ctx, groups); err != nil {
			return 0, fmt.Errorf("read part 1 photographs: %w", err)
		}
	}
	if len(photos) == 0 {
		slog.WarnContext(ctx, "exam: photo part skipped; no unused photograph is left to write it from",
			"exam", versionCode, "part", part.PartNumber)
		return 0, nil
	}
	for i := range photos {
		req.Count = 1
		req.Photo = &photos[i]
		if _, err := s.bankAuthor.GenerateQuestions(ctx, req); err != nil {
			return i, err
		}
	}
	return len(photos), nil
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

// sectionNode is the skill spine node an exam part's items are tagged to.
func sectionNode(section string) string {
	switch section {
	case skillListening:
		return "LISTENING"
	case "writing":
		return "WRITING"
	case "speaking":
		return "SPEAKING"
	default:
		// Reading, and Use of English, are read.
		return "READING"
	}
}

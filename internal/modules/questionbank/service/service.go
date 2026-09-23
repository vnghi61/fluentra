package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank/contract"
	"github.com/fluentra/fluentra/internal/modules/questionbank/domain"
	"github.com/fluentra/fluentra/internal/modules/questionbank/repository"
	rbaccontract "github.com/fluentra/fluentra/internal/modules/rbac/contract"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/eventbus"
)

// Service implements the questionbank Author and Reader interfaces.
type Service struct {
	repo          *repository.Repository
	rbac          rbaccontract.Authorizer
	contentReader contentcontract.Reader
	tagIndex      contentcontract.TagIndex
	lessonAuthor  lessoncontract.Author
	generator     learningcontract.Generator
	events        eventbus.EventBus
	bankCourses   *bankCourseManager
}

// Config configures a new questionbank Service.
type Config struct {
	Repo          *repository.Repository
	RBAC          rbaccontract.Authorizer
	ContentReader contentcontract.Reader
	TagIndex      contentcontract.TagIndex
	LessonAuthor  lessoncontract.Author
	Generator     learningcontract.Generator
	Events        eventbus.EventBus
}

// New constructs a questionbank Service.
func New(cfg Config) *Service {
	var bm *bankCourseManager
	if cfg.LessonAuthor != nil {
		bm = newBankCourseManager(cfg.LessonAuthor)
	}
	return &Service{
		repo:          cfg.Repo,
		rbac:          cfg.RBAC,
		contentReader: cfg.ContentReader,
		tagIndex:      cfg.TagIndex,
		lessonAuthor:  cfg.LessonAuthor,
		generator:     cfg.Generator,
		events:        cfg.Events,
		bankCourses:   bm,
	}
}

// ListQuestions searches and filters question bank items.
func (s *Service) ListQuestions(ctx context.Context, filter contract.Filter) ([]*contract.Question, int, error) {
	if s.rbac != nil {
		if err := s.rbac.Require(ctx, rbaccontract.PermQuestionBankRead); err != nil {
			return nil, 0, err
		}
	}

	var tagged []uuid.UUID
	if filter.NodeCode != nil {
		if s.tagIndex == nil {
			return nil, 0, errors.New("filtering by spine node needs content's tag index")
		}
		ids, tagErr := s.tagIndex.ItemIDsTaggedWith(ctx, *filter.NodeCode)
		if tagErr != nil {
			return nil, 0, fmt.Errorf("resolve node %s: %w", *filter.NodeCode, tagErr)
		}
		tagged = ids
	}

	items, total, err := s.repo.ListQuestions(ctx, filter, tagged)
	if err != nil {
		return nil, 0, fmt.Errorf("list questions: %w", err)
	}

	res := make([]*contract.Question, 0, len(items))
	for _, it := range items {
		res = append(res, toContractQuestion(it))
	}
	return res, total, nil
}

// GetQuestion fetches a question item by ID.
func (s *Service) GetQuestion(ctx context.Context, id uuid.UUID) (*contract.Question, error) {
	if s.rbac != nil {
		if err := s.rbac.Require(ctx, rbaccontract.PermQuestionBankRead); err != nil {
			return nil, err
		}
	}

	item, err := s.repo.GetQuestionByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrQuestionNotFound) {
			return nil, apperr.New(apperr.NotFound, "QUESTION_NOT_FOUND", "Question not found.")
		}
		return nil, fmt.Errorf("get question %s: %w", id, err)
	}
	return toContractQuestion(item), nil
}

// GetQuestionStats returns empirical statistics for a question.
func (s *Service) GetQuestionStats(ctx context.Context, id uuid.UUID) (*contract.QuestionStats, error) {
	if s.rbac != nil {
		if err := s.rbac.Require(ctx, rbaccontract.PermQuestionBankRead); err != nil {
			return nil, err
		}
	}

	stats, err := s.repo.GetQuestionStats(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrQuestionNotFound) {
			return nil, apperr.New(apperr.NotFound, "QUESTION_STATS_NOT_FOUND", "Question statistics not found.")
		}
		return nil, fmt.Errorf("get stats for %s: %w", id, err)
	}

	return &contract.QuestionStats{
		QuestionID:     stats.QuestionID,
		Attempts:       stats.Attempts,
		PValue:         stats.PValue,
		Discrimination: stats.Discrimination,
		AvgTimeMs:      stats.AvgTimeMs,
		LastComputedAt: stats.LastComputedAt,
	}, nil
}

// DrawableForPart returns the questions an exam may draw for a part.
func (s *Service) DrawableForPart(ctx context.Context, examPartID uuid.UUID) ([]*contract.Question, error) {
	items, err := s.repo.ListDrawableQuestionsForPart(ctx, examPartID)
	if err != nil {
		return nil, err
	}
	res := make([]*contract.Question, 0, len(items))
	for _, it := range items {
		res = append(res, toContractQuestion(it))
	}
	return res, nil
}

// SampleQuestions draws random published questions matching criteria.
func (s *Service) SampleQuestions(ctx context.Context, criteria contract.SampleCriteria) ([]*contract.Question, error) {
	items, err := s.repo.SamplePublishedQuestions(ctx, criteria)
	if err != nil {
		return nil, fmt.Errorf("sample questions: %w", err)
	}

	res := make([]*contract.Question, 0, len(items))
	for _, it := range items {
		res = append(res, toContractQuestion(it))
	}
	return res, nil
}

// CreateQuestion manually records a question bank item.
func (s *Service) CreateQuestion(ctx context.Context, q *contract.Question) (*contract.Question, error) {
	if s.rbac != nil {
		if err := s.rbac.Require(ctx, rbaccontract.PermQuestionBankCreate); err != nil {
			return nil, err
		}
	}

	dom := toDomainQuestion(q)
	if err := dom.Validate(); err != nil {
		return nil, apperr.New(apperr.Validation, "INVALID_QUESTION", err.Error())
	}

	created, err := s.repo.CreateQuestion(ctx, dom)
	if err != nil {
		return nil, fmt.Errorf("create question: %w", err)
	}
	return toContractQuestion(created), nil
}

// GenerateQuestions generates N items for a part and nodes using learning Generator.
func (s *Service) GenerateQuestions(ctx context.Context, req contract.GenerateRequest) ([]*contract.Question, error) {
	if s.rbac != nil {
		if err := s.rbac.Require(ctx, rbaccontract.PermQuestionBankCreate); err != nil {
			return nil, err
		}
	}

	if s.generator == nil {
		return nil, fmt.Errorf("learning generator dependency is required for question generation")
	}

	count := req.Count
	if count <= 0 {
		count = 1
	}

	skill := req.Skill
	if skill == "" {
		skill = determineSkill(req.Kind)
	}

	genItems, err := s.generator.Generate(ctx, learningcontract.GenerateRequest{
		Kind:      req.Kind,
		CEFRLevel: req.CEFRLevel,
		NodeCodes: req.NodeCodes,
		Count:     count,
		Purpose:   "bank",
		// One batch id per run, so the doubts of one generation are reviewed
		// together (WO 22 Stage A.4).
		Batch: fmt.Sprintf("bank:%s:%s", req.Kind, time.Now().UTC().Format("20060102T150405")),
	})
	if err != nil {
		return nil, fmt.Errorf("learning generator: %w", err)
	}

	createdQuestions := make([]*contract.Question, 0, len(genItems))
	for _, it := range genItems {
		// The row points at the content item, never a version id standing in for
		// one: no foreign key checks that column (DB4), so a wrong id would be a
		// question whose body nobody can find.
		if s.contentReader == nil {
			return nil, errors.New("content reader is required to record a generated question")
		}
		ver, vErr := s.contentReader.GetVersion(ctx, it.ContentVersionID)
		if vErr != nil {
			return nil, fmt.Errorf("resolve generated version %s: %w", it.ContentVersionID, vErr)
		}
		if ver == nil {
			return nil, fmt.Errorf("generated version %s not found", it.ContentVersionID)
		}
		contentItemID := ver.ItemID

		fp, fpErr := domain.FingerprintFromBody(req.Kind, it.Body)
		if fpErr != nil {
			slog.WarnContext(ctx, "could not compute fingerprint from body", "error", fpErr)
			continue
		}

		// Avoid duplicate item insertion if duplicate already exists.
		if existing, _ := s.repo.GetQuestionByFingerprint(ctx, fp); existing != nil {
			createdQuestions = append(createdQuestions, toContractQuestion(existing))
			continue
		}

		qCount := determineQuestionCount(req.Kind, it.Body)
		examPartID := req.ExamPartID

		domQ := &domain.Question{
			ID:            uuid.New(),
			ContentItemID: contentItemID,
			ExamPartID:    examPartID,
			Kind:          req.Kind,
			Skill:         skill,
			CEFRLevel:     req.CEFRLevel,
			QuestionCount: qCount,
			Fingerprint:   fp,
			Provenance: map[string]any{
				"prompt_version":     it.PromptVersion,
				"model":              it.Model,
				"ai_request_id":      it.AIRequestID.String(),
				"content_version_id": it.ContentVersionID.String(),
			},
			Status:    domain.StatusDraft,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		created, cErr := s.repo.CreateQuestion(ctx, domQ)
		if cErr != nil {
			slog.WarnContext(ctx, "could not create question bank row", "error", cErr)
			continue
		}
		createdQuestions = append(createdQuestions, toContractQuestion(created))
	}

	return createdQuestions, nil
}

const skillReading = "reading"

// ErrNotReviewed refuses to put a question in the bank before a person has
// approved its content version: brief §8 forbids publishing exam questions
// without review, and the review queue is the only door.
var ErrNotReviewed = apperr.New(apperr.Conflict, "QUESTION_NOT_REVIEWED",
	"The question's content has not been reviewed and published yet.")

// contentStatusPublished is the wire value of contract.Version.Status for
// published content.
const contentStatusPublished = "published"

// PublishQuestion appends a reviewed question to the bank course. Its content
// version must already be published through the review queue: this does not
// publish content, it makes published content drawable.
func (s *Service) PublishQuestion(ctx context.Context, id uuid.UUID) (*contract.Question, error) {
	if s.rbac != nil {
		if err := s.rbac.Require(ctx, rbaccontract.PermQuestionBankReview); err != nil {
			return nil, err
		}
	}

	q, err := s.repo.GetQuestionByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrQuestionNotFound) {
			return nil, apperr.New(apperr.NotFound, "QUESTION_NOT_FOUND", "Question not found.")
		}
		return nil, fmt.Errorf("get question %s: %w", id, err)
	}

	raw, _ := q.Provenance["content_version_id"].(string)
	versionID, _ := uuid.Parse(raw)
	return s.publishIntoBank(ctx, q, versionID)
}

// HandleContentPublished is the content.published consumer. Approving a bank
// question's content in the review queue is what puts it in the bank; content
// that is not a bank question is ignored.
func (s *Service) HandleContentPublished(ctx context.Context, event contentcontract.Published) error {
	q, err := s.repo.GetQuestionByContentItemID(ctx, event.ItemID)
	if err != nil {
		if errors.Is(err, domain.ErrQuestionNotFound) {
			return nil
		}
		return fmt.Errorf("find question for content item %s: %w", event.ItemID, err)
	}
	if q.Status == domain.StatusRetired {
		return nil
	}
	_, err = s.publishIntoBank(ctx, q, event.VersionID)
	return err
}

// HandleContentArchived is the content.archived consumer: an item pulled from
// publication stops being drawn (WO 22 Stage A.5).
//
// A sample a person rejected, or an item a learner reported, is archived in
// content; this retires the question so no future test draws it. A stored
// composition is never rewritten — an attempt already built keeps its item.
func (s *Service) HandleContentArchived(ctx context.Context, event contentcontract.Archived) error {
	q, err := s.repo.GetQuestionByContentItemID(ctx, event.ItemID)
	if err != nil {
		if errors.Is(err, domain.ErrQuestionNotFound) {
			return nil
		}
		return fmt.Errorf("find question for content item %s: %w", event.ItemID, err)
	}
	if q.Status == domain.StatusRetired {
		return nil
	}
	if _, err := s.repo.UpdateQuestionStatus(ctx, q.ID, domain.StatusRetired); err != nil {
		return fmt.Errorf("retire question %s: %w", q.ID, err)
	}
	return nil
}

func (s *Service) publishIntoBank(
	ctx context.Context, q *domain.Question, versionID uuid.UUID,
) (*contract.Question, error) {
	if q.ActivityID != nil && q.Status == domain.StatusPublished {
		return toContractQuestion(q), nil
	}
	if s.bankCourses == nil || s.contentReader == nil {
		return nil, errors.New("lesson author and content reader are required to publish into the bank")
	}
	if versionID == uuid.Nil {
		return nil, ErrNotReviewed
	}
	ver, err := s.contentReader.GetVersion(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("get content version %s: %w", versionID, err)
	}
	if ver == nil || ver.ItemID != q.ContentItemID || ver.Status != contentStatusPublished {
		return nil, ErrNotReviewed
	}

	lessonID, err := s.bankCourses.ensureBankLesson(ctx, resolveExamVersion(q.Kind), q.Kind)
	if err != nil {
		return nil, fmt.Errorf("ensure bank lesson: %w", err)
	}

	actID, err := s.lessonAuthor.AppendActivity(ctx, lessonID, lessoncontract.ActivitySpec{
		Kind:             q.Kind,
		ContentVersionID: ver.ID,
		Config:           ver.Body,
		Weight:           1,
	})
	if err != nil {
		return nil, fmt.Errorf("append activity to bank course: %w", err)
	}

	q, err = s.repo.UpdateQuestionActivityID(ctx, q.ID, actID)
	if err != nil {
		return nil, fmt.Errorf("update question activity id: %w", err)
	}

	q, err = s.repo.UpdateQuestionStatus(ctx, q.ID, domain.StatusPublished)
	if err != nil {
		return nil, fmt.Errorf("update question status: %w", err)
	}

	s.publishItemEvent(ctx, q, actID)
	return toContractQuestion(q), nil
}

func resolveExamVersion(kind string) string {
	switch kind {
	case contract.KindPhotoDescription, contract.KindQuestionResponse,
		contract.KindMcqGap, contract.KindTextCompletion:
		return "TOEIC"
	default:
		return "VSTEP"
	}
}

func (s *Service) publishItemEvent(ctx context.Context, q *domain.Question, actID uuid.UUID) {
	if s.events == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"question_id": q.ID.String(),
		"activity_id": actID.String(),
		"skill":       q.Skill,
		"cefr_level":  q.CEFRLevel,
		"kind":        q.Kind,
	})
	_ = s.events.Publish(ctx, eventbus.Message{
		ID:      uuid.New(),
		Topic:   contract.EventItemPublished,
		Payload: payload,
	})
}

// RetireQuestion marks a question as retired.
func (s *Service) RetireQuestion(ctx context.Context, id uuid.UUID) (*contract.Question, error) {
	if s.rbac != nil {
		if err := s.rbac.Require(ctx, rbaccontract.PermQuestionBankReview); err != nil {
			return nil, err
		}
	}

	q, err := s.repo.UpdateQuestionStatus(ctx, id, domain.StatusRetired)
	if err != nil {
		if errors.Is(err, domain.ErrQuestionNotFound) {
			return nil, apperr.New(apperr.NotFound, "QUESTION_NOT_FOUND", "Question not found.")
		}
		return nil, fmt.Errorf("retire question %s: %w", id, err)
	}
	return toContractQuestion(q), nil
}

func determineQuestionCount(kind string, body json.RawMessage) int {
	var parsed struct {
		Questions []json.RawMessage `json:"questions"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && len(parsed.Questions) > 0 {
		return len(parsed.Questions)
	}
	if kind == contract.KindTextCompletion {
		return 4
	}
	return 1
}

func determineSkill(kind string) string {
	switch kind {
	case contract.KindPhotoDescription, contract.KindQuestionResponse, "listening_comprehension":
		return "listening"
	case contract.KindMcqGap, contract.KindTextCompletion, "reading_comprehension":
		return skillReading
	case "writing_prompt":
		return "writing"
	case "speaking_task":
		return "speaking"
	default:
		if strings.HasPrefix(kind, "listening") {
			return "listening"
		}
		if strings.HasPrefix(kind, skillReading) {
			return skillReading
		}
		if strings.HasPrefix(kind, "grammar") {
			return "grammar"
		}
		if strings.HasPrefix(kind, "writing") {
			return "writing"
		}
		if strings.HasPrefix(kind, "speaking") {
			return "speaking"
		}
		return skillReading
	}
}

func toContractQuestion(q *domain.Question) *contract.Question {
	if q == nil {
		return nil
	}
	return &contract.Question{
		ID:            q.ID,
		ContentItemID: q.ContentItemID,
		ActivityID:    q.ActivityID,
		ExamPartID:    q.ExamPartID,
		Kind:          q.Kind,
		Skill:         q.Skill,
		CEFRLevel:     q.CEFRLevel,
		Difficulty:    q.Difficulty,
		QuestionCount: q.QuestionCount,
		Fingerprint:   q.Fingerprint,
		Provenance:    q.Provenance,
		Status:        q.Status,
		CreatedAt:     q.CreatedAt,
		UpdatedAt:     q.UpdatedAt,
	}
}

func toDomainQuestion(q *contract.Question) *domain.Question {
	if q == nil {
		return nil
	}
	return &domain.Question{
		ID:            q.ID,
		ContentItemID: q.ContentItemID,
		ActivityID:    q.ActivityID,
		ExamPartID:    q.ExamPartID,
		Kind:          q.Kind,
		Skill:         q.Skill,
		CEFRLevel:     q.CEFRLevel,
		Difficulty:    q.Difficulty,
		QuestionCount: q.QuestionCount,
		Fingerprint:   q.Fingerprint,
		Provenance:    q.Provenance,
		Status:        q.Status,
		CreatedAt:     q.CreatedAt,
		UpdatedAt:     q.UpdatedAt,
	}
}

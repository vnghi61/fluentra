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
	contentAuthor contentcontract.Author
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
	ContentAuthor contentcontract.Author
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
		contentAuthor: cfg.ContentAuthor,
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

	items, total, err := s.repo.ListQuestions(ctx, filter)
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
	})
	if err != nil {
		return nil, fmt.Errorf("learning generator: %w", err)
	}

	createdQuestions := make([]*contract.Question, 0, len(genItems))
	for _, it := range genItems {
		var contentItemID uuid.UUID
		if s.contentReader != nil {
			ver, vErr := s.contentReader.GetVersion(ctx, it.ContentVersionID)
			if vErr == nil && ver != nil {
				contentItemID = ver.ItemID
			}
		}
		if contentItemID == uuid.Nil {
			contentItemID = it.ContentVersionID
		}

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

// PublishQuestion publishes a question and appends it to the bank course.
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

	if q.ActivityID != nil && q.Status == domain.StatusPublished {
		return toContractQuestion(q), nil
	}

	if s.bankCourses == nil || s.lessonAuthor == nil {
		return nil, fmt.Errorf("lesson author is required to publish question to bank course")
	}

	versionID, body := s.resolveContentVersionAndBody(ctx, q)
	examVersion := resolveExamVersion(q.Kind)

	lessonID, err := s.bankCourses.ensureBankLesson(ctx, examVersion, q.Kind)
	if err != nil {
		return nil, fmt.Errorf("ensure bank lesson: %w", err)
	}

	actID, err := s.lessonAuthor.AppendActivity(ctx, lessonID, lessoncontract.ActivitySpec{
		Kind:             q.Kind,
		ContentVersionID: versionID,
		Config:           body,
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

func (s *Service) resolveContentVersionAndBody(
	ctx context.Context, q *domain.Question,
) (uuid.UUID, json.RawMessage) {
	var versionID uuid.UUID
	var body json.RawMessage
	if rawVerID, ok := q.Provenance["content_version_id"].(string); ok && rawVerID != "" {
		versionID, _ = uuid.Parse(rawVerID)
	}
	if versionID == uuid.Nil && s.contentAuthor != nil {
		vID, ensureErr := s.contentAuthor.EnsurePublished(ctx, contentcontract.AuthorSpec{
			Slug:      fmt.Sprintf("bank-%s", q.Fingerprint[:16]),
			Kind:      q.Kind,
			CEFRLevel: q.CEFRLevel,
			Body:      json.RawMessage(`{}`),
			AuthorID:  uuid.Nil,
		})
		if ensureErr == nil {
			versionID = vID
		}
	}
	if s.contentReader != nil && versionID != uuid.Nil {
		ver, vErr := s.contentReader.GetVersion(ctx, versionID)
		if vErr == nil && ver != nil {
			body = ver.Body
		}
	}
	return versionID, body
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

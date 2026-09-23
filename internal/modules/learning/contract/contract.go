package contract

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
)

// Aggregate is the outbox aggregate name every event below is written under.
const Aggregate = "learning"

// Event topics published by the learning module.
const (
	EventActivityCompleted        = "activity.completed"
	EventLessonCompleted          = "lesson.completed"
	EventCourseCompleted          = "course.completed"
	EventPlacementCompleted       = "placement.completed"
	EventLearningSessionCompleted = "learning.session_completed"
)

// ActivityCompleted is published when a learner completes an activity attempt.
type ActivityCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	ActivityID uuid.UUID `json:"activity_id"`
	Score      int       `json:"score"`
	Skill      string    `json:"skill"`
	DurationMs int       `json:"duration_ms"`
	OccurredAt time.Time `json:"occurred_at"`
}

// LessonCompleted is published when all required activities in a lesson are completed.
type LessonCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	LessonID   uuid.UUID `json:"lesson_id"`
	Score      int       `json:"score"`
	SkillFocus string    `json:"skill_focus"`
	OccurredAt time.Time `json:"occurred_at"`
}

// CourseCompleted is published when all required units and lessons in a course are completed.
type CourseCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	CourseID   uuid.UUID `json:"course_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// PlacementCompleted is published when a learner finishes an adaptive placement test.
type PlacementCompleted struct {
	UserID     uuid.UUID       `json:"user_id"`
	Level      string          `json:"level"`
	PerSkill   json.RawMessage `json:"per_skill"`
	OccurredAt time.Time       `json:"occurred_at"`
}

// SessionCompleted is published when a study session is ended.
type SessionCompleted struct {
	UserID     uuid.UUID `json:"user_id"`
	SessionID  uuid.UUID `json:"session_id"`
	Minutes    int       `json:"minutes"`
	Activities int       `json:"activities"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ReviewItem models an item worthy of spaced repetition produced by grading.
// Owned by learning to allow all skill modules to implement ExerciseGrader without depending on srs (Trap 1).
type ReviewItem struct {
	ContentVersionID uuid.UUID `json:"content_version_id"`
	Skill            string    `json:"skill"`
	InitialGrade     string    `json:"initial_grade"`
}

// GradeRequest contains the inputs required for a skill grader to evaluate an attempt.
// Carries IDs rather than full content structs to avoid tight coupling with content representations (P8.3 trap).
type GradeRequest struct {
	AttemptID        uuid.UUID       `json:"attempt_id"`
	ActivityID       uuid.UUID       `json:"activity_id"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	UserID           uuid.UUID       `json:"user_id"`
	Response         json.RawMessage `json:"response"`
}

// GradeResult contains the outcome of grading an exercise.
type GradeResult struct {
	Score    int    `json:"score"`
	MaxScore int    `json:"max_score"`
	Correct  bool   `json:"correct"`
	Feedback string `json:"feedback"`
	// CorrectAnswer is what the learner should have said, revealed after they
	// have answered.
	//
	// It exists because the answer stopped travelling with the question. The
	// authored body carries both, and the renderer used to read the answer
	// straight out of it — which meant every learner held the answer key for a
	// lesson before starting it. The body is redacted now, so the grader, which
	// is the only thing that has to know, hands the answer back at the one
	// moment it is safe to.
	//
	// For a choice-based activity this is the option's id, because that is what
	// the renderer needs to mark the right row. Empty when the activity has no
	// single answer worth showing.
	CorrectAnswer string             `json:"correct_answer,omitempty"`
	Async         bool               `json:"async"`
	ReviewItems   []ReviewItem       `json:"review_items,omitempty"`
	Explanation   *AnswerExplanation `json:"explanation,omitempty"`
	ItemResults   []ItemResult       `json:"item_results,omitempty"`
}

// ItemResult models the grading outcome of a single question within a multi-question activity.
type ItemResult struct {
	ID            string  `json:"id"`
	Correct       bool    `json:"correct"`
	CorrectAnswer *string `json:"correct_answer,omitempty"`
	// Explanation is why the correct answer is correct, shown once the set is graded.
	Explanation *AnswerExplanation `json:"explanation,omitempty"`
}

// AnswerExplanation models an explanation in English and Vietnamese for an exercise answer.
type AnswerExplanation struct {
	Text   string `json:"text"`
	TextVi string `json:"text_vi"`
}

// UnmarshalJSON reads both spellings an explanation is stored under.
//
// The generation prompts write "explanation_en" and "explanation_vi" into an
// item body, while graders and the explanation cache use "text" and "text_vi".
// Decoding a generated body with the second spelling only gave a non-nil but
// empty explanation: the learner saw nothing, and because it was not nil the
// AI fallback never ran either.
func (e *AnswerExplanation) UnmarshalJSON(data []byte) error {
	var raw struct {
		Text          string `json:"text"`
		TextVi        string `json:"text_vi"`
		ExplanationEn string `json:"explanation_en"`
		ExplanationVi string `json:"explanation_vi"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.Text = raw.Text
	if strings.TrimSpace(e.Text) == "" {
		e.Text = raw.ExplanationEn
	}
	e.TextVi = raw.TextVi
	if strings.TrimSpace(e.TextVi) == "" {
		e.TextVi = raw.ExplanationVi
	}
	return nil
}

// Empty reports whether there is nothing to show in either language.
func (e *AnswerExplanation) Empty() bool {
	return e == nil || (strings.TrimSpace(e.Text) == "" && strings.TrimSpace(e.TextVi) == "")
}

// ExerciseGrader is implemented by every skill module to grade domain-specific exercises.
type ExerciseGrader interface {
	Grade(ctx context.Context, req GradeRequest) (GradeResult, error)
}

// MeteredGrader is optionally implemented by an ExerciseGrader that consumes metered,
// billable resources (such as external AI models).
//
// Unauthenticated visitors reaching preview routes are refused before metered grading runs.
type MeteredGrader interface {
	SpendsMoney() bool
}

// ProgressScope represents the aggregation level of learner progress.
type ProgressScope string

// ProgressScope enumeration values.
const (
	ScopeActivity ProgressScope = "activity"
	ScopeLesson   ProgressScope = "lesson"
	ScopeUnit     ProgressScope = "unit"
	ScopeCourse   ProgressScope = "course"
)

// Progress represents a rolled-up progress record.
type Progress struct {
	UserID      uuid.UUID     `json:"user_id"`
	Scope       ProgressScope `json:"scope"`
	ScopeID     uuid.UUID     `json:"scope_id"`
	Status      string        `json:"status"`
	Score       *int          `json:"score,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
}

// ProgressReader provides read access to rolled-up progress for other modules.
//
// One call answers for a whole scope rather than one row. `gamification`,
// `admin` and `analytics` each render a learner's progress across every course
// or lesson at once, so a reader keyed on a single scope id would put the same
// N+1 into three Phase 3 modules that UnlockChecker below was batched to avoid.
// This is the signature AGENT.md §4 documents.
type ProgressReader interface {
	ProgressOf(ctx context.Context, userID uuid.UUID, scope ProgressScope) ([]Progress, error)
}

// UnlockChecker answers whether a learner has met prerequisites for lessons.
// Batched to avoid N+1 queries when evaluating an entire course tree (Trap 3).
type UnlockChecker interface {
	IsUnlocked(ctx context.Context, userID uuid.UUID, lessonIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}

// AsyncGradingCompleter handles asynchronous grading completion and failure.
//
// Both methods update the attempt row only WHERE status = 'grading' and report
// whether they did, preventing races with background sweeps and concurrent workers.
type AsyncGradingCompleter interface {
	CompleteAsyncGrading(ctx context.Context, attemptID uuid.UUID, result GradeResult) (bool, error)
	FailAsyncGrading(ctx context.Context, attemptID uuid.UUID, reason string) (bool, error)
}

// AttemptCounter reports how many of a user's attempts for a grader count toward a
// daily limit: graded ones and ones still being graded, but not failed ones.
type AttemptCounter interface {
	CountAttemptsTowardLimitSince(ctx context.Context, userID uuid.UUID, grader string, since time.Time) (int, error)
}

// AttemptDetail contains attempt data needed by asynchronous graders.
type AttemptDetail struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	ActivityID       uuid.UUID
	ContentVersionID uuid.UUID
	Response         json.RawMessage
	CreatedAt        time.Time
	Status           string
}

// AttemptReader reads an attempt by ID across modules.
type AttemptReader interface {
	GetAttemptForGrading(ctx context.Context, attemptID uuid.UUID) (*AttemptDetail, error)
}

// SittingAnswerRequest contains an answer to be graded and recorded as an attempt from an exam sitting.
type SittingAnswerRequest struct {
	UserID         uuid.UUID       `json:"user_id"`
	ActivityID     uuid.UUID       `json:"activity_id"`
	Response       json.RawMessage `json:"response"`
	IdempotencyKey uuid.UUID       `json:"idempotency_key"`
}

// SittingAnswerResult is the outcome of grading a sitting answer.
type SittingAnswerResult struct {
	AttemptID   uuid.UUID    `json:"attempt_id"`
	Status      string       `json:"status"`
	Score       int          `json:"score"`
	MaxScore    int          `json:"max_score"`
	Correct     bool         `json:"correct"`
	Feedback    string       `json:"feedback"`
	Async       bool         `json:"async"`
	ItemResults []ItemResult `json:"item_results,omitempty"`
}

// SittingAnswerSubmitter submits and grades an exam sitting answer into a learn.attempts row.
type SittingAnswerSubmitter interface {
	SubmitSittingAnswer(ctx context.Context, req SittingAnswerRequest) (*SittingAnswerResult, error)
}

// ItemExposureRecorder records and lists exposed items for a user.
//
// RecordItemExposures takes the caller's transaction: a sitting that fails to
// start must mark nothing as seen, so the exposures commit with the sitting or
// not at all. A nil tx writes outside any transaction.
type ItemExposureRecorder interface {
	RecordItemExposures(ctx context.Context, tx pgx.Tx, userID uuid.UUID, activityIDs []uuid.UUID) error
	ListItemExposures(ctx context.Context, userID uuid.UUID, activityIDs []uuid.UUID) (map[uuid.UUID]time.Time, error)
}

// ExamSectionActivities represents drawn activities for an exam sitting section.
type ExamSectionActivities struct {
	SectionPosition int            `json:"section_position"`
	Skill           string         `json:"skill"`
	Activities      []ExamActivity `json:"activities"`
}

// ExamActivity represents an activity drawn for an exam sitting.
type ExamActivity struct {
	ID               uuid.UUID       `json:"id"`
	Kind             string          `json:"kind"`
	ContentVersionID uuid.UUID       `json:"content_version_id"`
	Config           json.RawMessage `json:"config,omitempty"`
	Weight           int             `json:"weight"`
}

// ExamPoolDrawer draws activities across the 4 skills for an exam sitting.
type ExamPoolDrawer interface {
	DrawExamSitting(ctx context.Context, userID uuid.UUID, level string) ([]ExamSectionActivities, error)
}

// AuthorResolver resolves an author for generated content.
type AuthorResolver interface {
	FirstHolderOf(ctx context.Context, role string) (uuid.UUID, error)
}

// AttemptOutcome is where an attempt's grading stands.
type AttemptOutcome struct {
	AttemptID uuid.UUID
	Status    string
	Score     *int
	MaxScore  int
}

// AttemptOutcomeReader reads where an attempt's grading stands. The exam report
// uses it to settle the items that were graded asynchronously.
type AttemptOutcomeReader interface {
	GetAttemptOutcome(ctx context.Context, attemptID uuid.UUID) (*AttemptOutcome, error)
}

// PlacementListeningPolicy answers how many times a clip may be played in a
// placement test. listening asks it for a play whose context is a placement
// session, so the context is the caller's open session serving that clip, not an
// id the client chose.
type PlacementListeningPolicy interface {
	PlacementListeningPlays(ctx context.Context, userID, sessionID, versionID uuid.UUID) (int, error)
}

// AudioLocator finds the rendered audio for a listening script, if there is any.
// Audio is rendered offline after an item is published, so an item's body may
// carry no object key while the clip exists in the TTS cache.
type AudioLocator interface {
	AudioKey(ctx context.Context, script, voice string) (objectKey string, found bool, err error)
}

// ItemVerifier runs the six checks that stand between a generated item and a
// learner. A community submission is checked by exactly the same machine, for
// exactly the same reason: a wrong answer key is invisible to a reader and
// wrong for everyone who meets it.
type ItemVerifier interface {
	// VerifyItem returns nil when the item passes, or an error naming the
	// check that failed. `blindSolve` is optional because check 4 costs an AI
	// call per item.
	VerifyItem(ctx context.Context, req VerifyItemRequest) error
}

// IndependentVerifier is the second, independent check a machine-authored item
// passes before publication (WO 22 Stage A).
//
// It is deliberately separate from ItemVerifier. That one runs inside the
// generator, answered by the same provider chain that wrote the item — a model
// grading its own work. This one is answered by a model other than the writer's,
// read from the item's `_provenance.model`, so a confirmation means a second
// opinion rather than self-review.
type IndependentVerifier interface {
	// VerifyIndependently returns a confirmed verification when an independent
	// model solves the item and finds its key and explanation sound. Anything it
	// doubts — including having no independent provider to ask — comes back
	// unconfirmed with the reason, never as an error: a doubt is a person's
	// work, not a failure.
	VerifyIndependently(
		ctx context.Context, req IndependentVerifyRequest,
	) (contentcontract.Verification, error)
}

// IndependentVerifyRequest specifies an item for independent verification.
type IndependentVerifyRequest struct {
	Kind      string
	TaskType  string
	CEFRLevel string
	Body      json.RawMessage
	// OfficialSpec is the exam part's published format, empty for Foundation.
	// The verifier checks the item against it when it is set.
	OfficialSpec string
}

// ExamPartConstraints defines structural constraints for an exam part item (Stage E/G).
type ExamPartConstraints struct {
	OptionCount       int  `json:"option_count,omitempty"`
	QuestionsPerGroup int  `json:"questions_per_group,omitempty"`
	MinWords          int  `json:"min_words,omitempty"`
	MaxWords          int  `json:"max_words,omitempty"`
	AudioRequired     bool `json:"audio_required,omitempty"`
}

// VerifyItemRequest specifies an item to verify through ItemVerifier.
type VerifyItemRequest struct {
	Kind            string
	TaskType        string // read_aloud / respond, for speaking_task
	CEFRLevel       string
	Body            json.RawMessage
	Existing        []json.RawMessage // for the duplicate check
	BlindSolve      bool
	CheckCEFR       bool                 // evaluates CEFR calibration through item_level task
	ExamConstraints *ExamPartConstraints // evaluates exam part structural shape
	CheckProvenance bool                 // verifies _provenance presence and completeness
}

const (
	// KindFoundationTopic is the content kind holding a spine topic's body.
	KindFoundationTopic = "foundation_topic"
	// KindFoundationQuiz is a quiz item tagged to a spine node.
	KindFoundationQuiz = "foundation_quiz"
	// KindFoundationReview is a review question tagged to a spine node.
	KindFoundationReview = "foundation_review"
	// KindLessonMaterial is a non-graded course material — a document or video
	// a learner opens and marks as done (WO 20).
	KindLessonMaterial = "lesson_material"
	// KindTypedCompletion is a completion question with a typed answer and a
	// word limit — form, note, sentence and summary completion, which IELTS
	// listening and reading are mostly made of (WO 22 D22-25).
	KindTypedCompletion = "typed_completion"
)

// GenerateRequest specifies parameters for the unified item generator.
type GenerateRequest struct {
	Kind       string
	CEFRLevel  string
	NodeCodes  []string // spine codes the item must exercise; at least one
	Count      int
	Purpose    string     // "practice" | "foundation" | "bank" | "resource"
	OwnerID    *uuid.UUID // set only for Purpose "resource": private to that learner
	SourceText string     // Purpose "resource" only: the extraction to generate from
	SlugPrefix string     // optional deterministic slug prefix for idempotent generation
	// Batch groups the drafts of one generation run so a person reviews the
	// doubts together (WO 22 Stage A). Stored under `_provenance.batch`; empty
	// means the item is not part of a batch.
	Batch string
	// ExamConstraints is the exam part's published format, when the item
	// belongs to one (WO 22 Stage I). The generator puts it on the structure
	// check, so a "TOEIC Part 3" item cannot be composed with the wrong
	// question count or option count.
	ExamConstraints *ExamPartConstraints
}

// GeneratedItem represents a single authored and verified item.
type GeneratedItem struct {
	ContentVersionID uuid.UUID
	Body             json.RawMessage
	PromptVersion    string
	Model            string
	AIRequestID      uuid.UUID
}

// Generator produces verified, spine-tagged educational content items.
type Generator interface {
	Generate(ctx context.Context, req GenerateRequest) ([]GeneratedItem, error)
}

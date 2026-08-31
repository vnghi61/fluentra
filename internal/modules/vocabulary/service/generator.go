package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/generated/vocabulary/sqlc"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/domain"
	"github.com/fluentra/fluentra/internal/modules/vocabulary/repository"
)

// Practice generation: turning the dictionary into things to do.
//
// The curated course has eight lessons and thirty-two activities against two
// hundred words, so most of the vocabulary a learner has been given had nothing
// to practise it with. This builds the rest — six exercises per word plus a
// matching drill per group — into a course of its own.
//
// Why a separate course rather than more activities in the curated one: the
// curated course is authored, ordered and pedagogically sequenced by a person,
// and a job that appended to it would silently rewrite somebody's syllabus. This
// one is entirely the job's, regenerated whole, and nothing hand-written lives
// in it.

// GeneratedCourse identifies the practice course. Constants because the slug is
// the identity that makes re-running the job converge rather than duplicate.
const (
	GeneratedCourseSlug  = "generated-vocabulary-practice"
	GeneratedCourseTitle = "Vocabulary Practice"

	// generatedSensesPerLesson is how many words one generated lesson covers.
	//
	// Four, matching the matching drill's group size, so every lesson opens with
	// one match over exactly the words it then drills. Six exercises per word
	// plus the match is 25 activities — long, but a practice lesson is meant to
	// be worked through rather than finished in one sitting.
	generatedSensesPerLesson = domain.MatchGroupSize

	// generatedLessonsPerUnit keeps a unit browsable.
	generatedLessonsPerUnit = 10

	// maxGeneratedSenses bounds one run. The job is scheduled, so what it does
	// not reach this time it reaches next time — and an unbounded scan is how a
	// background job becomes an outage as the dictionary grows.
	maxGeneratedSenses = 400

	generatedSkillFocus = "vocabulary"
)

// ContentAuthor is the narrow slice of content's authoring surface this uses.
type ContentAuthor interface {
	EnsurePublished(ctx context.Context, spec contentcontract.AuthorSpec) (uuid.UUID, error)
}

// LessonAuthor is the narrow slice of lesson's authoring surface this uses.
//
// Activities live in lesson's tables and rule L2 forbids writing them from here,
// so the generator asks rather than reaches. It is also why nothing in this file
// knows what a `learn.activities` row looks like.
type LessonAuthor interface {
	EnsureCourse(ctx context.Context, spec lessoncontract.CourseSpec) (uuid.UUID, error)
	EnsureUnit(ctx context.Context, spec lessoncontract.UnitSpec) (uuid.UUID, error)
	EnsureLesson(ctx context.Context, spec lessoncontract.LessonSpec) (uuid.UUID, error)
	ReplaceActivities(
		ctx context.Context, lessonID uuid.UUID, activities []lessoncontract.ActivitySpec,
	) error
}

// GeneratorDeps carries what the generator needs beyond the service itself.
type GeneratorDeps struct {
	Content ContentAuthor
	Lessons LessonAuthor
	// AuthorID owns the generated content. The admin account, supplied by the
	// composition root: generated content still has an owner, because
	// `content_items.owner_id` is not nullable and because unattributed content
	// is content nobody can be asked about.
	AuthorID uuid.UUID
}

// Generator builds practice lessons out of the dictionary.
type Generator struct {
	repo    repository.Repository
	content ContentAuthor
	lessons LessonAuthor
	author  uuid.UUID
}

// NewGenerator constructs the generator.
func NewGenerator(repo repository.Repository, deps GeneratorDeps) *Generator {
	return &Generator{
		repo:    repo,
		content: deps.Content,
		lessons: deps.Lessons,
		author:  deps.AuthorID,
	}
}

// GenerateExercises is the scheduled entry point.
//
// Idempotent end to end: content is addressed by slug, lessons and units by
// position, and activities are replaced wholesale. A second run over unchanged
// data writes nothing at all — content.EnsurePublished returns the existing
// version when the body matches, and the upserts leave their rows as they are.
func (g *Generator) GenerateExercises(ctx context.Context) error {
	if g.content == nil || g.lessons == nil || g.author == uuid.Nil {
		// Not an error: cmd/worker builds this module for other reasons too,
		// and a generator with no authoring surface simply has no work to do.
		slog.DebugContext(ctx, "vocabulary generator is not configured; skipping")
		return nil
	}

	rows, err := g.repo.ListSensesForGeneration(ctx, maxGeneratedSenses)
	if err != nil {
		return fmt.Errorf("list senses for generation: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	senses := make([]domain.GenSense, 0, len(rows))
	for _, row := range rows {
		senses = append(senses, toGenSense(row))
	}

	courseID, err := g.lessons.EnsureCourse(ctx, lessoncontract.CourseSpec{
		Slug:  GeneratedCourseSlug,
		Title: GeneratedCourseTitle,
		Description: "Extra practice generated from every word in your dictionary: " +
			"recall, spelling, meaning in context, and sentence order.",
		CEFRFrom:       "A1",
		CEFRTo:         "C2",
		EstimatedHours: len(senses) / 10,
	})
	if err != nil {
		return fmt.Errorf("ensure generated course: %w", err)
	}

	groups := chunk(senses, generatedSensesPerLesson)
	built := 0
	for index, group := range groups {
		if err := g.buildLesson(ctx, courseID, index, group, senses); err != nil {
			// One lesson's failure must not cost the rest of the run: the next
			// scheduled run retries it, and a partial catalogue is better than
			// none.
			slog.WarnContext(ctx, "generated lesson failed",
				"lesson_index", index, "error", err)
			continue
		}
		built++
	}

	slog.InfoContext(ctx, "vocabulary practice generated",
		"senses", len(senses), "lessons", built)
	return nil
}

// buildLesson authors one lesson's content and activities.
func (g *Generator) buildLesson(
	ctx context.Context, courseID uuid.UUID, index int, group, pool []domain.GenSense,
) error {
	unitPosition := index/generatedLessonsPerUnit + 1
	unitID, err := g.lessons.EnsureUnit(ctx, lessoncontract.UnitSpec{
		CourseID: courseID,
		Position: unitPosition,
		Title:    fmt.Sprintf("Practice set %d", unitPosition),
		Description: "Generated drills over the words in your dictionary, " +
			"ordered by how common they are.",
	})
	if err != nil {
		return fmt.Errorf("ensure unit: %w", err)
	}

	lessonID, err := g.lessons.EnsureLesson(ctx, lessoncontract.LessonSpec{
		UnitID:     unitID,
		Position:   index%generatedLessonsPerUnit + 1,
		Title:      lessonTitle(group),
		SkillFocus: generatedSkillFocus,
		// Roughly a minute an exercise. An estimate, and honest about being one.
		EstimatedMinutes: len(group) * 6,
	})
	if err != nil {
		return fmt.Errorf("ensure lesson: %w", err)
	}

	// The matching drill opens the lesson: it introduces the four words the
	// rest of the lesson then practises one at a time.
	exercises := make([]domain.GenExercise, 0, len(group)*6+1)
	if match, ok := domain.GenerateMatch(group); ok {
		exercises = append(exercises, match)
	}
	for _, sense := range group {
		exercises = append(exercises, domain.GenerateForSense(sense, pool)...)
	}

	activities := make([]lessoncontract.ActivitySpec, 0, len(exercises))
	for position, exercise := range exercises {
		versionID, err := g.publishExercise(ctx, exercise, group)
		if err != nil {
			// A single exercise that cannot be authored is dropped rather than
			// failing the lesson: the others are still worth having.
			slog.WarnContext(ctx, "generated exercise not authored",
				"slug", exercise.Slug, "error", err)
			continue
		}
		config, err := exercise.MarshalConfig()
		if err != nil {
			continue
		}
		activities = append(activities, lessoncontract.ActivitySpec{
			Position:         position + 1,
			Kind:             exercise.Kind,
			ContentVersionID: versionID,
			Config:           config,
			Weight:           1,
		})
	}

	if len(activities) == 0 {
		return fmt.Errorf("no activity could be authored for lesson %d", index)
	}
	return g.lessons.ReplaceActivities(ctx, lessonID, activities)
}

// publishExercise stores an exercise's authored side and returns its version.
func (g *Generator) publishExercise(
	ctx context.Context, exercise domain.GenExercise, group []domain.GenSense,
) (uuid.UUID, error) {
	body, err := exercise.MarshalBody()
	if err != nil {
		return uuid.Nil, err
	}
	return g.content.EnsurePublished(ctx, contentcontract.AuthorSpec{
		Slug:      exercise.Slug,
		Kind:      exercise.Kind,
		CEFRLevel: groupCEFR(group),
		Body:      body,
		AuthorID:  g.author,
	})
}

// toGenSense maps a query row into the generator's input.
func toGenSense(row sqlc.ListSensesForGenerationRow) domain.GenSense {
	sense := domain.GenSense{
		Lemma:      row.Lemma,
		POS:        row.Pos,
		CEFRLevel:  row.CefrLevel,
		Definition: row.Definition,
	}
	if row.Ipa != nil {
		sense.IPA = *row.Ipa
	}
	if row.DefinitionVi != nil {
		sense.DefinitionVi = *row.DefinitionVi
	}
	if row.ContentVersionID != nil {
		sense.ContentVersionID = row.ContentVersionID.String()
	}

	// The stored shape is `[{sentence, sentence_vi, audio_url}]`. A row that
	// does not parse yields no examples rather than failing the word: the
	// kinds that need a sentence are skipped and the flashcard still works.
	var stored []struct {
		Sentence   string `json:"sentence"`
		SentenceVi string `json:"sentence_vi"`
		AudioURL   string `json:"audio_url"`
	}
	if len(row.Examples) > 0 {
		_ = json.Unmarshal(row.Examples, &stored)
	}
	for _, example := range stored {
		if example.Sentence == "" {
			continue
		}
		sense.Examples = append(sense.Examples, domain.GenExample{
			Sentence:   example.Sentence,
			SentenceVi: example.SentenceVi,
		})
		if sense.AudioURL == "" {
			sense.AudioURL = example.AudioURL
		}
	}
	return sense
}

// lessonTitle names a lesson after the words in it, so the syllabus is readable
// rather than a list of "Practice set 7, lesson 3".
func lessonTitle(group []domain.GenSense) string {
	switch len(group) {
	case 0:
		return "Practice"
	case 1:
		return group[0].Lemma
	default:
		title := group[0].Lemma
		for _, sense := range group[1 : len(group)-1] {
			title += ", " + sense.Lemma
		}
		return title + " & " + group[len(group)-1].Lemma
	}
}

// groupCEFR is the hardest level in the group, because a set is only as easy as
// its hardest member.
func groupCEFR(group []domain.GenSense) string {
	order := map[string]int{"A1": 1, "A2": 2, "B1": 3, "B2": 4, "C1": 5, "C2": 6}
	best, level := 0, "A1"
	for _, sense := range group {
		if rank, ok := order[sense.CEFRLevel]; ok && rank > best {
			best, level = rank, sense.CEFRLevel
		}
	}
	return level
}

func chunk(senses []domain.GenSense, size int) [][]domain.GenSense {
	if size <= 0 {
		return nil
	}
	groups := make([][]domain.GenSense, 0, (len(senses)+size-1)/size)
	for start := 0; start < len(senses); start += size {
		end := start + size
		if end > len(senses) {
			end = len(senses)
		}
		groups = append(groups, senses[start:end])
	}
	return groups
}

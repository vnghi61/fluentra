//go:build integration

package learning_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/content"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	"github.com/fluentra/fluentra/internal/modules/learning/repository"
	"github.com/fluentra/fluentra/internal/modules/learning/service"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	"github.com/fluentra/fluentra/internal/platform/ai"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// fencedExamModel answers only writing-prompt generation, inside a ```json
// fence, as models do. Every other slot's candidate is refused, which the
// top-up logs and moves past.
type fencedExamModel struct {
	mu    sync.Mutex
	calls int
}

func (m *fencedExamModel) Complete(_ context.Context, req ai.Request) (ai.Response, error) {
	if req.Task != ai.TaskPracticeGenerate || req.Vars["Kind"] != "writing_prompt" {
		return ai.Response{}, errors.New("this model only writes essay prompts")
	}
	m.mu.Lock()
	m.calls++
	n := m.calls
	m.mu.Unlock()
	prompt := fmt.Sprintf("Prompt %d: some people think cities should ban cars from their centres. "+
		"Discuss both views and give your own opinion with reasons.", n)
	body := fmt.Sprintf(`{
		"prompt": "%s",
		"model_answer": "%s",
		"min_words": 150,
		"topic": "Cities %d"
	}`, prompt, modelAnswer(), n)
	return ai.Response{Text: "```json\n" + body + "\n```", Model: "fenced-test"}, nil
}

func modelAnswer() string {
	sentence := "Banning cars from city centres makes streets quieter and safer for people who walk. "
	out := ""
	for i := 0; i < 8; i++ {
		out += sentence
	}
	return out
}

type allowAll struct{}

func (allowAll) Require(context.Context, string) error { return nil }

// TestTopUpExamPool_AppendsAnActivityAgainstTheRealDatabase is the test work order
// 12 §3.7 asks for. Work order 11's pool passed every unit test and stayed empty
// in a running stack: a fake content author accepted a slug and an owner the
// database refused. Here the content and lesson authors are the real ones.
func TestTopUpExamPool_AppendsAnActivityAgainstTheRealDatabase(t *testing.T) {
	if attemptPool == nil {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	adminID := uuid.New()
	if _, err := attemptPool.Exec(ctx,
		`INSERT INTO core.users (id, email, status) VALUES ($1, $2, 'active')`,
		adminID, "exam-pool-"+adminID.String()+"@example.test"); err != nil {
		t.Fatalf("seed owner: %v", err)
	}

	contentModule := content.NewAuthoring(content.Deps{Pool: attemptPool})
	lessonModule := lesson.New(lesson.Deps{Pool: attemptPool, Guard: allowAll{}, Env: "test"})
	model := &fencedExamModel{}

	svc := service.New(service.Deps{
		Pool:          attemptPool,
		Repo:          repositoryAdapter{Repository: repository.New(attemptPool)},
		Lesson:        lessonModule.Reader(),
		LessonAuthor:  lessonModule.Author(),
		Content:       contentModule.Reader(),
		ContentAuthor: contentModule.Author(),
		Graders:       domain.NewGraderRegistry(),
		AI:            model,
		Clock:         clock.Real{},

		GeneratorAuthorID: adminID,
	})

	if err := svc.TopUpExamPool(ctx); err != nil {
		t.Fatalf("TopUpExamPool: %v", err)
	}

	var activities int
	if err := attemptPool.QueryRow(ctx, `
		SELECT count(*)
		FROM learn.activities a
		JOIN learn.lessons l ON l.id = a.lesson_id
		JOIN learn.course_units u ON u.id = l.unit_id
		JOIN learn.courses c ON c.id = u.course_id
		WHERE c.slug = 'pool-exam' AND a.kind = 'writing_prompt'`).Scan(&activities); err != nil {
		t.Fatalf("count pool activities: %v", err)
	}
	if activities == 0 {
		t.Fatalf("the top-up appended no activity (the model answered %d times)", model.calls)
	}

	var owned int
	if err := attemptPool.QueryRow(ctx, `
		SELECT count(*) FROM content.content_items
		WHERE slug LIKE 'pool-exam-%-writing-prompt-%' AND owner_id = $1`, adminID).Scan(&owned); err != nil {
		t.Fatalf("count published items: %v", err)
	}
	if owned != activities {
		t.Fatalf("published %d items owned by the generator, appended %d activities", owned, activities)
	}
}

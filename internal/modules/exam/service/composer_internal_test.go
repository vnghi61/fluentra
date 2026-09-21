package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	questionbankcontract "github.com/fluentra/fluentra/internal/modules/questionbank/contract"
)

func bankItem(questions int, level string) *questionbankcontract.Question {
	act := uuid.New()
	return &questionbankcontract.Question{
		ID: uuid.New(), ActivityID: &act, QuestionCount: questions, CEFRLevel: level,
		Status: questionbankcontract.StatusPublished,
	}
}

// TOEIC Part 7 is seeded with group_size 1 and 54 questions, but its bank items
// are passages holding several questions each. Drawing must stop at 54
// questions, not 54 passages (G.3).
func TestPickByCEFR_AVariableSizePartIsFilledByQuestionsNotItems(t *testing.T) {
	var ordered []*questionbankcontract.Question
	for i := 0; i < 30; i++ {
		ordered = append(ordered, bankItem(2+i%4, "B1")) // 2..5 questions each
	}
	part := &domain.ExamPart{QuestionCount: 54, GroupSize: 1}

	picked, filled := pickByCEFR(fitPart(part, ordered), part.QuestionCount, nil)

	assert.Equal(t, 54, filled)
	assert.Less(t, len(picked), 54, "54 questions come from far fewer than 54 passages")
}

func TestFitPart_AFixedGroupPartTakesOnlyItemsOfThatSize(t *testing.T) {
	part := &domain.ExamPart{QuestionCount: 12, GroupSize: 4}
	fits := bankItem(4, "B1")
	tooSmall := bankItem(3, "B1")
	noActivity := bankItem(4, "B1")
	noActivity.ActivityID = nil

	got := fitPart(part, []*questionbankcontract.Question{fits, tooSmall, noActivity})

	assert.Equal(t, []*questionbankcontract.Question{fits}, got)
}

func TestPickByCEFR_FollowsTheMixThenFillsFromWhatIsLeft(t *testing.T) {
	var ordered []*questionbankcontract.Question
	for i := 0; i < 10; i++ {
		ordered = append(ordered, bankItem(1, "B1"))
	}
	for i := 0; i < 2; i++ {
		ordered = append(ordered, bankItem(1, "C1"))
	}
	mix := map[string]float64{"B1": 0.5, "C1": 0.5}

	picked, filled := pickByCEFR(ordered, 6, mix)

	levels := map[string]int{}
	for _, q := range picked {
		levels[q.CEFRLevel]++
	}
	assert.Equal(t, 6, filled)
	assert.Equal(t, 2, levels["C1"], "every C1 the bank holds is used")
	assert.Equal(t, 4, levels["B1"], "the C1 shortfall is filled rather than refused")
}

func TestCEFRQuotas_SumToTheTarget(t *testing.T) {
	quota := cefrQuotas(35, map[string]float64{"B1": 0.33, "B2": 0.34, "C1": 0.33})

	assert.Equal(t, 35, quota["B1"]+quota["B2"]+quota["C1"])
}

package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/fluentra/fluentra/internal/modules/learning/domain"
)

func planLessons(n int, skill string) []domain.PlanLesson {
	lessons := make([]domain.PlanLesson, n)
	for i := range lessons {
		lessons[i] = domain.PlanLesson{ID: uuid.New(), CourseID: uuid.Nil, Title: "Lesson", Skill: skill, Minutes: 10}
	}
	return lessons
}

func countKinds(items []domain.WeeklyPlanItem) (map[string]int, int, int) {
	kinds := map[string]int{}
	total, largest := 0, 0
	for _, item := range items {
		kinds[item.Kind]++
		total += item.Minutes
		largest = max(largest, item.Minutes)
	}
	return kinds, total, largest
}

func TestWeeklyPlan_ComposesTheWeekByShare(t *testing.T) {
	items := domain.BuildWeeklyPlan(domain.WeeklyPlanInput{
		MinutesGoal:   90,
		Lessons:       planLessons(12, domain.SkillGrammar),
		DueReviews:    100,
		ReviewSeconds: 20,
	})

	kinds, total, largest := countKinds(items)
	assert.Positive(t, kinds[domain.PlanItemLesson], "the next lessons are planned")
	assert.Positive(t, kinds[domain.PlanItemDailyPractice], "daily practice sets are planned")
	assert.Positive(t, kinds[domain.PlanItemReviews], "due reviews are planned")
	assert.Equal(t, 1, kinds[domain.PlanItemWriting], "one writing or speaking task")
	assert.GreaterOrEqual(t, total, 90-largest, "the plan fills the goal")
	assert.Less(t, total, 90+largest, "the plan never exceeds the goal by more than one item")
}

func TestWeeklyPlan_WholeItemsOnly(t *testing.T) {
	items := domain.BuildWeeklyPlan(domain.WeeklyPlanInput{MinutesGoal: 40, DueReviews: 7, ReviewSeconds: 30})
	for _, item := range items {
		assert.Positive(t, item.Minutes)
		if item.Kind == domain.PlanItemReviews {
			assert.Equal(t, domain.ReviewBlockMinutes, item.Minutes)
			assert.LessOrEqual(t, item.Reviews, 7)
		}
	}
}

func TestWeeklyPlan_NoGoalMeansNinetyMinutes(t *testing.T) {
	withDefault := domain.BuildWeeklyPlan(domain.WeeklyPlanInput{Lessons: planLessons(12, domain.SkillReading)})
	withNinety := domain.BuildWeeklyPlan(domain.WeeklyPlanInput{
		MinutesGoal: domain.DefaultWeeklyMinutes, Lessons: planLessons(12, domain.SkillReading),
	})
	_, defaultTotal, _ := countKinds(withDefault)
	_, ninetyTotal, _ := countKinds(withNinety)
	assert.Equal(t, 90, domain.DefaultWeeklyMinutes)
	assert.Equal(t, ninetyTotal, defaultTotal)
}

func TestWeeklyPlan_WeakestSkillGetsAnExtraItem(t *testing.T) {
	lessons := append(planLessons(6, domain.SkillGrammar), planLessons(2, domain.SkillListening)...)
	without := domain.BuildWeeklyPlan(domain.WeeklyPlanInput{MinutesGoal: 90, Lessons: lessons, DueReviews: 50})
	with := domain.BuildWeeklyPlan(domain.WeeklyPlanInput{
		MinutesGoal: 90, Lessons: lessons, DueReviews: 50, WeakestSkill: domain.SkillListening,
	})

	var focus []domain.WeeklyPlanItem
	for _, item := range with {
		if item.Focus {
			focus = append(focus, item)
		}
	}
	if assert.Len(t, focus, 1, "one extra item for the weakest skill") {
		assert.Equal(t, domain.SkillListening, focus[0].Skill)
		assert.Equal(t, domain.PlanItemLesson, focus[0].Kind, "the next listening lesson not already planned")
	}
	assert.Len(t, with, len(without), "the extra item replaces the least important one")
}

func TestWeeklyPlan_ALearnerWithNothingStillGetsAPlan(t *testing.T) {
	items := domain.BuildWeeklyPlan(domain.WeeklyPlanInput{})
	kinds, total, _ := countKinds(items)
	assert.NotEmpty(t, items)
	assert.Positive(t, kinds[domain.PlanItemDailyPractice])
	assert.LessOrEqual(t, total, 90+domain.ProductiveTaskMinutes)
}

func TestWeakestSkill(t *testing.T) {
	mastery := []domain.SkillMastery{
		{Skill: domain.SkillReading, Level: domain.LevelB1},
		{Skill: domain.SkillListening, Level: domain.LevelA2},
		{Skill: domain.SkillWriting, Level: domain.LevelB2},
	}
	assert.Equal(t, domain.SkillListening, domain.WeakestSkill(mastery, ""))
	assert.Equal(t, domain.SkillListening, domain.WeakestSkill(mastery, domain.LevelC1))
	assert.Empty(t, domain.WeakestSkill(nil, domain.LevelB2), "no mastery, no weakest skill")
}

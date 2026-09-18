package domain

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// The weekly plan — work order 13 §3.7.

// Weekly plan item kinds.
const (
	PlanItemLesson        = "lesson"
	PlanItemDailyPractice = "daily_practice"
	PlanItemReviews       = "reviews"
	PlanItemWriting       = "writing"
	PlanItemSpeaking      = "speaking"
)

// Weekly plan sizing.
const (
	// DefaultWeeklyMinutes is the goal used until the learner sets one.
	DefaultWeeklyMinutes = 90
	// DailyPracticeMinutes is what one daily practice set takes: a reading passage,
	// five tense choices and three sentence transformations.
	DailyPracticeMinutes = 10
	// ReviewBlockMinutes is the size of one review item.
	ReviewBlockMinutes = 5
	// ProductiveTaskMinutes is one writing or speaking task.
	ProductiveTaskMinutes = 15
	// DefaultReviewSeconds is the pace used for a learner with no reviews yet.
	DefaultReviewSeconds = 20.0
)

// The composition of a week, by estimated minutes.
const (
	planShareLessons    = 0.40
	planSharePractice   = 0.30
	planShareReviews    = 0.20
	planShareProductive = 0.10
)

// WeeklyPlanItem is one whole thing to do this week.
type WeeklyPlanItem struct {
	Kind     string     `json:"kind"`
	Minutes  int        `json:"minutes"`
	LessonID *uuid.UUID `json:"lesson_id,omitempty"`
	CourseID *uuid.UUID `json:"course_id,omitempty"`
	Title    string     `json:"title,omitempty"`
	Skill    string     `json:"skill,omitempty"`
	// Reviews is how many due reviews a review item covers.
	Reviews int `json:"reviews,omitempty"`
	// Focus marks the item added for the weakest skill.
	Focus bool `json:"focus,omitempty"`
}

// WeeklyPlan is a learner's plan for one week, fixed once built.
type WeeklyPlan struct {
	UserID      uuid.UUID
	WeekStart   time.Time
	MinutesGoal int
	Items       []WeeklyPlanItem
	CreatedAt   time.Time
}

// PlanLesson is an upcoming lesson the plan may include.
type PlanLesson struct {
	ID       uuid.UUID
	CourseID uuid.UUID
	Title    string
	Skill    string
	Minutes  int
}

// WeeklyPlanInput is everything the plan is built from.
type WeeklyPlanInput struct {
	MinutesGoal   int
	Lessons       []PlanLesson
	DueReviews    int
	ReviewSeconds float64
	// WeakestSkill is empty when the learner has no mastery to compare.
	WeakestSkill string
}

// BuildWeeklyPlan composes the week: 40% the next lessons, 30% daily practice
// sets, 20% due reviews at the learner's pace and 10% one writing or speaking
// task. Items are whole, and the plan never exceeds the goal by more than one
// item. The weakest skill gets one extra item in place of the least important.
func BuildWeeklyPlan(in WeeklyPlanInput) []WeeklyPlanItem {
	goal := in.MinutesGoal
	if goal <= 0 {
		goal = DefaultWeeklyMinutes
	}
	budget := func(share float64) float64 { return share * float64(goal) }

	lessons := lessonItems(in.Lessons, budget(planShareLessons))
	practice := repeatItems(WeeklyPlanItem{Kind: PlanItemDailyPractice, Minutes: DailyPracticeMinutes},
		wholeItems(budget(planSharePractice), DailyPracticeMinutes))
	reviews := reviewItems(in.DueReviews, in.ReviewSeconds, budget(planShareReviews))
	productive := []WeeklyPlanItem{productiveItem(in.WeakestSkill)}

	groups := [][]WeeklyPlanItem{lessons, practice, reviews, productive}
	shares := []float64{planShareLessons, planSharePractice, planShareReviews, planShareProductive}
	groups = trimToGoal(groups, shares, goal)

	if focus, ok := focusItem(in.WeakestSkill, in.Lessons, lessons); ok {
		groups = replaceLeastImportant(groups, focus)
	}

	var items []WeeklyPlanItem
	for _, group := range groups {
		items = append(items, group...)
	}
	return items
}

// wholeItems rounds a budget to whole items of a size.
func wholeItems(minutes float64, size int) int {
	return int(math.Round(minutes / float64(size)))
}

func repeatItems(item WeeklyPlanItem, count int) []WeeklyPlanItem {
	items := make([]WeeklyPlanItem, 0, max(count, 0))
	for i := 0; i < count; i++ {
		items = append(items, item)
	}
	return items
}

// lessonItems takes the next lessons in order while their minutes are within the
// share, rounding to the nearer whole lesson at the edge.
func lessonItems(lessons []PlanLesson, share float64) []WeeklyPlanItem {
	var items []WeeklyPlanItem
	total := 0.0
	for _, lesson := range lessons {
		minutes := max(lesson.Minutes, 1)
		if total+float64(minutes)/2 > share {
			break
		}
		id, course := lesson.ID, lesson.CourseID
		items = append(items, WeeklyPlanItem{
			Kind: PlanItemLesson, Minutes: minutes, LessonID: &id, CourseID: &course,
			Title: lesson.Title, Skill: lesson.Skill,
		})
		total += float64(minutes)
	}
	return items
}

// reviewItems covers as many due reviews as fit the share at the learner's
// pace, in whole review blocks.
func reviewItems(due int, secondsEach, share float64) []WeeklyPlanItem {
	if due <= 0 {
		return nil
	}
	if secondsEach <= 0 {
		secondsEach = DefaultReviewSeconds
	}
	dueMinutes := float64(due) * secondsEach / 60
	blocks := wholeItems(math.Min(dueMinutes, share), ReviewBlockMinutes)
	if blocks == 0 && dueMinutes > 0 && share >= ReviewBlockMinutes/2.0 {
		blocks = 1
	}
	perBlock := max(1, int(float64(ReviewBlockMinutes*60)/secondsEach))
	items := make([]WeeklyPlanItem, 0, blocks)
	remaining := due
	for i := 0; i < blocks && remaining > 0; i++ {
		covered := min(perBlock, remaining)
		items = append(items, WeeklyPlanItem{Kind: PlanItemReviews, Minutes: ReviewBlockMinutes, Reviews: covered})
		remaining -= covered
	}
	return items
}

func productiveItem(weakest string) WeeklyPlanItem {
	if weakest == SkillSpeaking {
		return WeeklyPlanItem{Kind: PlanItemSpeaking, Minutes: ProductiveTaskMinutes, Skill: SkillSpeaking}
	}
	return WeeklyPlanItem{Kind: PlanItemWriting, Minutes: ProductiveTaskMinutes, Skill: SkillWriting}
}

// trimToGoal removes items, from the group furthest over its share, while the
// plan would still meet the goal without the item removed. What is left exceeds
// the goal by less than one item. The last group, the one writing or speaking
// task, is never trimmed: it is the plan's only productive practice.
func trimToGoal(groups [][]WeeklyPlanItem, shares []float64, goal int) [][]WeeklyPlanItem {
	for {
		total := totalMinutes(groups)
		worst, worstOver := -1, math.Inf(-1)
		for i, group := range groups[:len(groups)-1] {
			if len(group) == 0 {
				continue
			}
			last := group[len(group)-1]
			if total-last.Minutes < goal {
				continue
			}
			over := float64(groupMinutes(group)) - shares[i]*float64(goal)
			if over > worstOver {
				worst, worstOver = i, over
			}
		}
		if worst < 0 {
			return groups
		}
		groups[worst] = groups[worst][:len(groups[worst])-1]
	}
}

func groupMinutes(group []WeeklyPlanItem) int {
	total := 0
	for _, item := range group {
		total += item.Minutes
	}
	return total
}

func totalMinutes(groups [][]WeeklyPlanItem) int {
	total := 0
	for _, group := range groups {
		total += groupMinutes(group)
	}
	return total
}

// focusItem is the extra item for the weakest skill: a writing or speaking task
// for those skills, otherwise the next lesson on that skill not already planned,
// or a daily practice set when there is none.
func focusItem(weakest string, lessons []PlanLesson, planned []WeeklyPlanItem) (WeeklyPlanItem, bool) {
	switch weakest {
	case "":
		return WeeklyPlanItem{}, false
	case SkillWriting, SkillSpeaking:
		item := productiveItem(weakest)
		item.Focus = true
		return item, true
	}
	inPlan := make(map[uuid.UUID]bool, len(planned))
	for _, item := range planned {
		if item.LessonID != nil {
			inPlan[*item.LessonID] = true
		}
	}
	for _, lesson := range lessons {
		if lesson.Skill == weakest && !inPlan[lesson.ID] {
			id, course := lesson.ID, lesson.CourseID
			return WeeklyPlanItem{
				Kind: PlanItemLesson, Minutes: max(lesson.Minutes, 1), LessonID: &id, CourseID: &course,
				Title: lesson.Title, Skill: weakest, Focus: true,
			}, true
		}
	}
	return WeeklyPlanItem{Kind: PlanItemDailyPractice, Minutes: DailyPracticeMinutes, Skill: weakest, Focus: true}, true
}

// replaceLeastImportant puts the focus item in place of the least important
// item: the last review block, else the last practice set, else the last lesson.
// With nothing to replace, the focus item is simply added.
func replaceLeastImportant(groups [][]WeeklyPlanItem, focus WeeklyPlanItem) [][]WeeklyPlanItem {
	for _, index := range []int{2, 1, 0} {
		if n := len(groups[index]); n > 0 {
			groups[index] = groups[index][:n-1]
			groups[index] = append(groups[index], focus)
			return groups
		}
	}
	groups[0] = append(groups[0], focus)
	return groups
}

// WeakestSkill is the skill with the lowest mastery band, measured as the
// distance below the target level when one is set. Ties go to the first in
// skill order. Empty when there is no mastery at all.
func WeakestSkill(mastery []SkillMastery, targetLevel string) string {
	order := []string{SkillVocabulary, SkillGrammar, SkillReading, SkillListening, SkillWriting, SkillSpeaking}
	bySkill := make(map[string]string, len(mastery))
	for _, m := range mastery {
		bySkill[m.Skill] = m.Level
	}
	target := cefrOrdinal(targetLevel)
	weakest, worst := "", math.MinInt
	for _, skill := range order {
		level, ok := bySkill[skill]
		if !ok {
			continue
		}
		gap := -cefrOrdinal(level)
		if target > 0 {
			gap = target - cefrOrdinal(level)
		}
		if gap > worst {
			weakest, worst = skill, gap
		}
	}
	return weakest
}

func cefrOrdinal(level string) int {
	for i, l := range []string{LevelA1, LevelA2, LevelB1, LevelB2, LevelC1, LevelC2} {
		if l == level {
			return i + 1
		}
	}
	return 0
}

// CEFROrdinal is a level's position from A1 = 1 to C2 = 6, or 0 when it is not
// a level.
func CEFROrdinal(level string) int {
	return cefrOrdinal(level)
}

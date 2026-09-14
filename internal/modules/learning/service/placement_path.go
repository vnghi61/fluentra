package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/fluentra/fluentra/internal/modules/learning/contract"
	"github.com/fluentra/fluentra/internal/modules/learning/domain"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
)

// The starting point and the weekly plan — work order 13 §3.6 and §3.7.

// Where the level a path uses comes from.
const (
	LevelSourcePlacement = "placement"
	LevelSourceDeclared  = "declared"
	LevelSourceDefault   = "default"
)

// planLessonLookahead is how many upcoming lessons a weekly plan chooses from.
const planLessonLookahead = 12

// Graders whose attempts count as a writing or speaking task done.
const (
	graderWritingPrompt = "writing_prompt"
	graderSpeakingTask  = "speaking_task"
)

// PathLessonDTO is the lesson a course starts at.
type PathLessonDTO struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	CEFRLevel *string   `json:"cefr_level"`
}

// PathCourseDTO is a recommended course and where to start in it.
type PathCourseDTO struct {
	ID          uuid.UUID      `json:"id"`
	Slug        string         `json:"slug"`
	Title       string         `json:"title"`
	CEFRFrom    string         `json:"cefr_from"`
	CEFRTo      string         `json:"cefr_to"`
	StartLesson *PathLessonDTO `json:"start_lesson"`
}

// StartingPathDTO is GET /me/path.
type StartingPathDTO struct {
	Level       string          `json:"level"`
	LevelSource string          `json:"level_source"`
	Courses     []PathCourseDTO `json:"courses"`
}

// WeeklyPlanItemDTO is a plan item and whether it is done.
type WeeklyPlanItemDTO struct {
	domain.WeeklyPlanItem
	Done bool `json:"done"`
}

// WeeklyPlanProgressDTO is the week's progress, read when the plan is read.
type WeeklyPlanProgressDTO struct {
	Minutes    int `json:"minutes"`
	ItemsDone  int `json:"items_done"`
	ItemsTotal int `json:"items_total"`
}

// WeeklyPlanDTO is GET /me/weekly-plan.
type WeeklyPlanDTO struct {
	WeekStart   string                `json:"week_start"`
	MinutesGoal int                   `json:"minutes_goal"`
	Items       []WeeklyPlanItemDTO   `json:"items"`
	Progress    WeeklyPlanProgressDTO `json:"progress"`
}

// --------------------------------------------------------------------------
// The starting point
// --------------------------------------------------------------------------

// GetStartingPath recommends courses for the learner's level and the lesson to
// start at in each.
func (s *Service) GetStartingPath(ctx context.Context, userID uuid.UUID) (*StartingPathDTO, error) {
	level, source, err := s.pathLevel(ctx, userID)
	if err != nil {
		return nil, err
	}
	courses, err := s.pathCourses(ctx, level)
	if err != nil {
		return nil, err
	}
	path := &StartingPathDTO{Level: level, LevelSource: source, Courses: []PathCourseDTO{}}
	for _, course := range courses {
		lessons, err := s.courseLessonsInOrder(ctx, course.ID)
		if err != nil {
			return nil, err
		}
		dto := PathCourseDTO{
			ID: course.ID, Slug: course.Slug, Title: course.Title, CEFRFrom: course.CEFRFrom, CEFRTo: course.CEFRTo,
		}
		if start := startLesson(lessons, level); start != nil {
			dto.StartLesson = &PathLessonDTO{ID: start.ID, Title: start.Title, CEFRLevel: start.CEFRLevel}
		}
		path.Courses = append(path.Courses, dto)
	}
	return path, nil
}

// pathLevel is the current placement result; failing that the declared level;
// failing that A2.
func (s *Service) pathLevel(ctx context.Context, userID uuid.UUID) (level, source string, err error) {
	result, err := s.repo.GetCurrentPlacementResult(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("read placement result: %w", err)
	}
	if result != nil {
		return result.Level, LevelSourcePlacement, nil
	}
	if s.user != nil {
		profile, found, err := s.user.GetLearningProfile(ctx, userID)
		if err != nil {
			return "", "", fmt.Errorf("read learning profile: %w", err)
		}
		if found && profile.DeclaredLevel != nil && domain.CEFROrdinal(*profile.DeclaredLevel) > 0 {
			return *profile.DeclaredLevel, LevelSourceDeclared, nil
		}
	}
	return domain.LevelA2, LevelSourceDefault, nil
}

// pathCourses are the curriculum courses whose range contains the level, and the
// nearest below it when none does.
func (s *Service) pathCourses(ctx context.Context, level string) ([]*lessoncontract.Course, error) {
	if s.courses == nil {
		return nil, fmt.Errorf("course catalogue is not configured")
	}
	containing, err := s.courses.ListCurriculumCourses(ctx, &level)
	if err != nil {
		return nil, fmt.Errorf("list courses for %s: %w", level, err)
	}
	if len(containing) > 0 {
		return containing, nil
	}
	all, err := s.courses.ListCurriculumCourses(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list curriculum courses: %w", err)
	}
	target := domain.CEFROrdinal(level)
	var nearest []*lessoncontract.Course
	nearestTo := 0
	for _, course := range all {
		to := domain.CEFROrdinal(course.CEFRTo)
		switch {
		case to >= target || to == 0 || to < nearestTo:
		case to > nearestTo:
			nearest, nearestTo = []*lessoncontract.Course{course}, to
		default:
			nearest = append(nearest, course)
		}
	}
	return nearest, nil
}

// startLesson is the first lesson, in course order, whose level is at or above
// the learner's. In a course entirely below the learner — the nearest course
// below their level — it is the first lesson of the course's highest level.
func startLesson(lessons []*lessoncontract.Lesson, level string) *lessoncontract.Lesson {
	target := domain.CEFROrdinal(level)
	var top *lessoncontract.Lesson
	topLevel := 0
	for _, lesson := range lessons {
		if lesson == nil || lesson.CEFRLevel == nil {
			continue
		}
		ordinal := domain.CEFROrdinal(strings.ToUpper(*lesson.CEFRLevel))
		if ordinal >= target {
			return lesson
		}
		if ordinal > topLevel {
			top, topLevel = lesson, ordinal
		}
	}
	return top
}

// placedOrdinal is the learner's placed level as an ordinal, or 0 with no
// placement. A declared level is a claim, not a measurement, and opens nothing.
func (s *Service) placedOrdinal(ctx context.Context, userID uuid.UUID) (int, error) {
	if userID == uuid.Nil {
		return 0, nil
	}
	result, err := s.repo.GetCurrentPlacementResult(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("read placement result: %w", err)
	}
	if result == nil {
		return 0, nil
	}
	return domain.CEFROrdinal(result.Level), nil
}

// openedByPlacement reports whether a prerequisite is met by placement: its
// lesson has a level, and that level is below the placed one. Nothing is
// written; the prerequisite lesson stays not done.
func openedByPlacement(req lessoncontract.PrerequisiteItem, placed int) bool {
	if placed == 0 || req.RequiresLessonLevel == nil {
		return false
	}
	required := domain.CEFROrdinal(strings.ToUpper(*req.RequiresLessonLevel))
	return required > 0 && required < placed
}

// placementStartLesson is where a learner who has not begun a course starts:
// the first lesson at or above their placed level. Nil without a placement.
func (s *Service) placementStartLesson(
	ctx context.Context, userID uuid.UUID, lessons []*lessoncontract.Lesson, completed map[uuid.UUID]bool,
) (*lessoncontract.Lesson, error) {
	for _, lesson := range lessons {
		if lesson != nil && completed[lesson.ID] {
			return nil, nil
		}
	}
	result, err := s.repo.GetCurrentPlacementResult(ctx, userID)
	if err != nil || result == nil {
		return nil, err
	}
	return startLesson(lessons, result.Level), nil
}

// defaultPracticeLevel is the daily set's level when the request names none and
// the learner has chosen no practice level: the placed level held to A2–B2.
// learning reads the result it owns and never writes the preference.
func (s *Service) defaultPracticeLevel(ctx context.Context, userID uuid.UUID) string {
	result, err := s.repo.GetCurrentPlacementResult(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "could not read placement for the daily set level", "error", err)
		return defaultPracticeLevel
	}
	if result == nil {
		return defaultPracticeLevel
	}
	switch placed := domain.CEFROrdinal(result.Level); {
	case placed <= domain.CEFROrdinal(domain.LevelA2):
		return domain.LevelA2
	case placed >= domain.CEFROrdinal(domain.LevelB2):
		return domain.LevelB2
	default:
		return result.Level
	}
}

// --------------------------------------------------------------------------
// The weekly plan
// --------------------------------------------------------------------------

// GetWeeklyPlan returns this week's plan, building it on the first request of
// the week. The plan is fixed for the week; progress is read now.
func (s *Service) GetWeeklyPlan(ctx context.Context, userID uuid.UUID) (*WeeklyPlanDTO, error) {
	weekStart, from, to := learnerWeek(s.clock.Now())
	plan, err := s.repo.GetWeeklyPlan(ctx, userID, weekStart)
	if err != nil {
		return nil, fmt.Errorf("read weekly plan: %w", err)
	}
	if plan == nil {
		if plan, err = s.createWeeklyPlan(ctx, userID, weekStart); err != nil {
			return nil, err
		}
	}
	return s.weeklyPlanDTO(ctx, userID, plan, from, to)
}

// learnerWeek is the week containing now, Monday to Monday in Asia/Ho_Chi_Minh:
// its start as a date, and its bounds as instants.
func learnerWeek(now time.Time) (weekStart, from, to time.Time) {
	loc, err := time.LoadLocation(hoChiMinhTimeZone)
	if err != nil {
		loc = time.FixedZone(hoChiMinhTimeZone, 7*3600)
	}
	local := now.In(loc)
	offset := (int(local.Weekday()) + 6) % 7
	monday := time.Date(local.Year(), local.Month(), local.Day()-offset, 0, 0, 0, 0, loc)
	weekStart = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
	return weekStart, monday, monday.AddDate(0, 0, 7)
}

func (s *Service) createWeeklyPlan(
	ctx context.Context, userID uuid.UUID, weekStart time.Time,
) (*domain.WeeklyPlan, error) {
	input, err := s.weeklyPlanInput(ctx, userID)
	if err != nil {
		return nil, err
	}
	plan := &domain.WeeklyPlan{
		UserID:      userID,
		WeekStart:   weekStart,
		MinutesGoal: input.MinutesGoal,
		Items:       domain.BuildWeeklyPlan(input),
	}
	stored, err := s.repo.CreateWeeklyPlan(ctx, plan)
	if err != nil {
		return nil, fmt.Errorf("store weekly plan: %w", err)
	}
	if stored != nil {
		return stored, nil
	}
	// Another request built this week's plan first; that one is the plan.
	winner, err := s.repo.GetWeeklyPlan(ctx, userID, weekStart)
	if err != nil || winner == nil {
		return nil, fmt.Errorf("read weekly plan after a concurrent build: %w", err)
	}
	return winner, nil
}

func (s *Service) weeklyPlanInput(ctx context.Context, userID uuid.UUID) (domain.WeeklyPlanInput, error) {
	input := domain.WeeklyPlanInput{MinutesGoal: domain.DefaultWeeklyMinutes, ReviewSeconds: domain.DefaultReviewSeconds}
	target := ""
	if s.user != nil {
		profile, found, err := s.user.GetLearningProfile(ctx, userID)
		if err != nil {
			return input, fmt.Errorf("read learning profile: %w", err)
		}
		if found && profile.WeeklyMinutesGoal != nil {
			input.MinutesGoal = *profile.WeeklyMinutesGoal
		}
		if found && profile.TargetLevel != nil {
			target = *profile.TargetLevel
		}
	}
	lessons, err := s.upcomingLessons(ctx, userID)
	if err != nil {
		return input, err
	}
	input.Lessons = lessons
	input.DueReviews = s.dueReviews(ctx, userID)
	if s.srsPace != nil {
		if pace, paceErr := s.srsPace.AverageReviewSeconds(ctx, userID); paceErr == nil && pace > 0 {
			input.ReviewSeconds = pace
		}
	}
	mastery, err := s.repo.ListSkillMasteryByUser(ctx, userID)
	if err != nil {
		return input, fmt.Errorf("read skill mastery: %w", err)
	}
	input.WeakestSkill = domain.WeakestSkill(mastery, target)
	return input, nil
}

func (s *Service) dueReviews(ctx context.Context, userID uuid.UUID) int {
	if s.srsDue == nil {
		return 0
	}
	due, err := s.srsDue.DueCount(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "could not count due reviews for the weekly plan", "error", err)
		return 0
	}
	return due
}

// upcomingLessons are the next lessons the learner has not completed: in the
// course they are enrolled in, or else the path's first course, from the start
// lesson when they have not begun it.
func (s *Service) upcomingLessons(ctx context.Context, userID uuid.UUID) ([]domain.PlanLesson, error) {
	courseID, lessons, err := s.planCourse(ctx, userID)
	if err != nil || len(lessons) == 0 {
		return nil, err
	}
	completed, err := s.completedScopeIDs(ctx, userID, string(contract.ScopeLesson))
	if err != nil {
		return nil, err
	}
	start, err := s.placementStartLesson(ctx, userID, lessons, completed)
	if err != nil {
		return nil, err
	}
	begun := start == nil
	var upcoming []domain.PlanLesson
	for _, lesson := range lessons {
		if lesson == nil {
			continue
		}
		if !begun {
			if lesson.ID != start.ID {
				continue
			}
			begun = true
		}
		if completed[lesson.ID] {
			continue
		}
		upcoming = append(upcoming, domain.PlanLesson{
			ID: lesson.ID, CourseID: courseID, Title: lesson.Title,
			Skill: lesson.SkillFocus, Minutes: lesson.EstimatedMinutes,
		})
		if len(upcoming) == planLessonLookahead {
			break
		}
	}
	return upcoming, nil
}

func (s *Service) planCourse(ctx context.Context, userID uuid.UUID) (uuid.UUID, []*lessoncontract.Lesson, error) {
	active, _, err := s.activeEnrollment(ctx, userID)
	if err != nil {
		return uuid.Nil, nil, err
	}
	if active != nil {
		lessons, listErr := s.courseLessonsInOrder(ctx, active.CourseID)
		return active.CourseID, lessons, listErr
	}
	if s.courses == nil {
		return uuid.Nil, nil, nil
	}
	path, err := s.GetStartingPath(ctx, userID)
	if err != nil || len(path.Courses) == 0 {
		return uuid.Nil, nil, err
	}
	lessons, err := s.courseLessonsInOrder(ctx, path.Courses[0].ID)
	return path.Courses[0].ID, lessons, err
}

// weeklyProgress is what the learner has done this week, read at request time.
type weeklyProgress struct {
	minutes   int
	lessons   map[uuid.UUID]bool
	practiced int
	writing   int
	speaking  int
	dueNow    int
}

func (s *Service) weeklyPlanDTO(
	ctx context.Context, userID uuid.UUID, plan *domain.WeeklyPlan, from, to time.Time,
) (*WeeklyPlanDTO, error) {
	progress, err := s.readWeeklyProgress(ctx, userID, plan, from, to)
	if err != nil {
		return nil, err
	}
	dto := &WeeklyPlanDTO{
		WeekStart:   plan.WeekStart.Format(time.DateOnly),
		MinutesGoal: plan.MinutesGoal,
		Items:       make([]WeeklyPlanItemDTO, 0, len(plan.Items)),
		Progress:    WeeklyPlanProgressDTO{Minutes: progress.minutes, ItemsTotal: len(plan.Items)},
	}
	for _, item := range plan.Items {
		done := progress.claim(item)
		if done {
			dto.Progress.ItemsDone++
		}
		dto.Items = append(dto.Items, WeeklyPlanItemDTO{WeeklyPlanItem: item, Done: done})
	}
	return dto, nil
}

func (s *Service) readWeeklyProgress(
	ctx context.Context, userID uuid.UUID, plan *domain.WeeklyPlan, from, to time.Time,
) (*weeklyProgress, error) {
	var err error
	progress := &weeklyProgress{lessons: map[uuid.UUID]bool{}}
	if progress.minutes, err = s.repo.SumLearningMinutesBetween(ctx, userID, from, to); err != nil {
		return nil, fmt.Errorf("sum weekly minutes: %w", err)
	}
	var lessonIDs []uuid.UUID
	for _, item := range plan.Items {
		if item.LessonID != nil {
			lessonIDs = append(lessonIDs, *item.LessonID)
		}
	}
	if len(lessonIDs) > 0 {
		rows, listErr := s.repo.ListProgressByUserScopeAndIDs(ctx, userID, string(contract.ScopeLesson), lessonIDs)
		if listErr != nil {
			return nil, fmt.Errorf("read planned lesson progress: %w", listErr)
		}
		for _, row := range rows {
			progress.lessons[row.ScopeID] = row.Status == domain.ProgressCompleted
		}
	}
	weekEnd := plan.WeekStart.AddDate(0, 0, 7)
	progress.practiced, err = s.repo.CountPracticedDailySetsBetween(ctx, userID, plan.WeekStart, weekEnd, from)
	if err != nil {
		return nil, fmt.Errorf("count practiced sets: %w", err)
	}
	graders := []string{graderWritingPrompt}
	if progress.writing, err = s.repo.CountAttemptsByGradersBetween(ctx, userID, graders, from, to); err != nil {
		return nil, fmt.Errorf("count writing tasks: %w", err)
	}
	graders = []string{graderSpeakingTask}
	if progress.speaking, err = s.repo.CountAttemptsByGradersBetween(ctx, userID, graders, from, to); err != nil {
		return nil, fmt.Errorf("count speaking tasks: %w", err)
	}
	progress.dueNow = s.dueReviews(ctx, userID)
	return progress, nil
}

// claim reports whether an item is done, spending one unit of counted work on
// it so that two practice items are not both marked done by one practiced set.
func (p *weeklyProgress) claim(item domain.WeeklyPlanItem) bool {
	switch item.Kind {
	case domain.PlanItemLesson:
		return item.LessonID != nil && p.lessons[*item.LessonID]
	case domain.PlanItemDailyPractice:
		return spend(&p.practiced)
	case domain.PlanItemReviews:
		return p.dueNow == 0
	case domain.PlanItemWriting:
		return spend(&p.writing)
	case domain.PlanItemSpeaking:
		return spend(&p.speaking)
	default:
		return false
	}
}

func spend(count *int) bool {
	if *count <= 0 {
		return false
	}
	*count--
	return true
}

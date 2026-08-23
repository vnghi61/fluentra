package domain

// ActivityEstimate represents the weight/duration parameters of an activity.
type ActivityEstimate struct {
	Weight int
}

// CalculateLessonDuration recalculates the estimated duration in minutes for a lesson
// based on its activities (BR-LESSON-06). Each activity contribution is proportional to its weight,
// with a minimum baseline per activity.
// The int32 is the column's width, and it is safe because the caller rejects a
// list longer than MaxActivitiesPerLesson and a weight above MaxActivityWeight
// before reaching here: the largest total this can return is 100 * 100 * 2.
func CalculateLessonDuration(activities []ActivityInput) int32 {
	if len(activities) == 0 {
		return 0
	}

	var totalMinutes int32
	for _, act := range activities {
		weight := act.Weight
		if weight <= 0 {
			weight = 1
		}
		if weight > MaxActivityWeight {
			weight = MaxActivityWeight
		}
		// Baseline estimate: 2 minutes per weight unit
		totalMinutes += int32(weight) * 2
	}

	return totalMinutes
}

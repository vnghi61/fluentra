package domain

// ActivityEstimate represents the weight/duration parameters of an activity.
type ActivityEstimate struct {
	Weight int
}

// CalculateLessonDuration recalculates the estimated duration in minutes for a lesson
// based on its activities (BR-LESSON-06). Each activity contribution is proportional to its weight,
// with a minimum baseline per activity.
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
		// Baseline estimate: 2 minutes per weight unit
		totalMinutes += int32(weight) * 2
	}

	return totalMinutes
}

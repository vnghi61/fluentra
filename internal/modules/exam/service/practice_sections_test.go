package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/service"
)

// A practice sitting could only be all four sections for as long as a learner
// chose. Practising one skill — reading, say — meant sitting listening first and
// spending its items. A practice sitting now holds the sections chosen, and only
// their items are marked seen.
func TestStartSitting_PracticeHoldsOnlyTheChosenSections(t *testing.T) {
	f := newFixture(t)

	att, err := f.svc.StartSitting(context.Background(), uuid.New(), f.examID, service.StartAttemptRequest{
		Mode: domain.ModePractice, ChosenDurationMinutes: 30, Sections: []int{4, 2},
	})
	require.NoError(t, err)

	require.Len(t, att.SectionActivities, 2)
	assert.Equal(t, 2, att.SectionActivities[0].SectionPosition)
	assert.Equal(t, 4, att.SectionActivities[1].SectionPosition)
	assert.Equal(t, 2, att.CurrentSection, "the sitting opens on the first section chosen")
	assert.ElementsMatch(t, []uuid.UUID{f.activity(2).ID, f.activity(4).ID}, f.exposures.seen)
}

func TestStartSitting_RefusesSectionsThatDoNotExistAndMarksNothingSeen(t *testing.T) {
	for _, sections := range [][]int{{0}, {5}, {2, 2}} {
		f := newFixture(t)
		_, err := f.svc.StartSitting(context.Background(), uuid.New(), f.examID, service.StartAttemptRequest{
			Mode: domain.ModePractice, ChosenDurationMinutes: 30, Sections: sections,
		})
		require.ErrorIs(t, err, domain.ErrInvalidSections, "sections %v", sections)
		assert.Empty(t, f.exposures.seen)
	}
}

func TestStartSitting_ExamModeSitsEverySectionWhateverIsAsked(t *testing.T) {
	f := newFixture(t)

	att, err := f.svc.StartSitting(context.Background(), uuid.New(), f.examID, service.StartAttemptRequest{
		Mode: domain.ModeExam, Sections: []int{1},
	})
	require.NoError(t, err)
	assert.Len(t, att.SectionActivities, 4)
}

// A report of a two-section practice sitting is complete when both are scored:
// the sections the learner did not choose are not missing.
func TestReport_APracticeSittingOfChosenSectionsIsNotPartial(t *testing.T) {
	f := newFixture(t)
	userID := uuid.New()
	att, err := f.svc.StartSitting(context.Background(), userID, f.examID, service.StartAttemptRequest{
		Mode: domain.ModePractice, ChosenDurationMinutes: 30, Sections: []int{2},
	})
	require.NoError(t, err)
	_, err = f.svc.AutosaveAnswers(context.Background(), userID, att.ID, service.SaveAnswersRequest{
		Answers: answer(f.activity(2).ID, `{"answers":{"q1":"A"}}`),
	})
	require.NoError(t, err)
	_, err = f.svc.SubmitExam(context.Background(), userID, att.ID, domain.SubmittedByLearner)
	require.NoError(t, err)

	report, err := f.svc.GetScoreReport(context.Background(), userID, att.ID)
	require.NoError(t, err)

	require.Len(t, report.PerSection, 1)
	assert.Equal(t, domain.ReportStatusReady, report.Status)
}

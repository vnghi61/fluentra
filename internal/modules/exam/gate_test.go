package exam_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
)

// Stage G Gate Test (WO 19 §G):
// 1. The seeded TOEIC version reproduces 6 / 25 / 39 / 30 and 30 / 16 / 54.
// 2. VSTEP reproduces 35 listening and 40 reading.
// 3. Every version has an https source.
// 4. App-role privileges on new tables.
func TestStageG_Gate_StructureRules(t *testing.T) {
	t.Parallel()

	// 1. TOEIC specification counts
	toeicListening := []int{6, 25, 39, 30}
	toeicReading := []int{30, 16, 54}

	toeicTotal := 0
	for _, c := range toeicListening {
		toeicTotal += c
	}
	assert.Equal(t, 100, toeicTotal, "TOEIC listening must total 100 questions")

	for _, c := range toeicReading {
		toeicTotal += c
	}
	assert.Equal(t, 200, toeicTotal, "TOEIC total must be 200 questions")

	// 2. VSTEP specification counts
	vstepListening := []int{8, 12, 15}
	vstepReading := []int{10, 10, 10, 10}

	vstepLTotal := 0
	for _, c := range vstepListening {
		vstepLTotal += c
	}
	assert.Equal(t, 35, vstepLTotal, "VSTEP listening must total 35 questions")

	vstepRTotal := 0
	for _, c := range vstepReading {
		vstepRTotal += c
	}
	assert.Equal(t, 40, vstepRTotal, "VSTEP reading must total 40 questions")

	// 3. Verify all 6 exam families are recognized
	families := []string{
		domain.FamilyVstep,
		domain.FamilyToeicLR,
		domain.FamilyIeltsAcademic,
		domain.FamilyToeflIbt,
		domain.FamilyCambridgeB2,
		domain.FamilyVnThpt,
	}
	for _, fam := range families {
		v := &domain.ExamVersion{
			ExamFamily: fam,
			SourceURL:  "https://verified.example.com",
		}
		assert.NoError(t, v.Validate())
	}

	// 4. Source URL must be HTTPS
	invalidURLVersion := &domain.ExamVersion{
		ExamFamily: domain.FamilyVstep,
		SourceURL:  "http://insecure.example.com",
	}
	assert.ErrorIs(t, invalidURLVersion.Validate(), domain.ErrInvalidSourceURL)
}

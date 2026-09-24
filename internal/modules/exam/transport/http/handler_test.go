package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fluentra/fluentra/internal/modules/exam/domain"
	"github.com/fluentra/fluentra/internal/modules/exam/service"
	examhttp "github.com/fluentra/fluentra/internal/modules/exam/transport/http"
	"github.com/fluentra/fluentra/internal/shared/apperr"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

type fakeExamService struct {
	listExamsFn    func(ctx context.Context) ([]service.ExamDTO, error)
	startSittingFn func(
		ctx context.Context, userID, examID uuid.UUID, req service.StartAttemptRequest,
	) (*service.ExamAttemptDTO, error)
	getAttemptFn func(ctx context.Context, userID, attemptID uuid.UUID) (*service.ExamAttemptDTO, error)
	autosaveFn   func(
		ctx context.Context, userID, attemptID uuid.UUID, req service.SaveAnswersRequest,
	) (*service.SaveAnswersResult, error)
	completeSectionFn func(
		ctx context.Context, userID, attemptID uuid.UUID, sectionNum int,
	) (*service.CompleteSectionResult, error)
	submitExamFn func(
		ctx context.Context, userID, attemptID uuid.UUID, submittedBy string,
	) (*service.SubmitExamResult, error)
	getReportFn    func(ctx context.Context, userID, attemptID uuid.UUID) (*service.ScoreReportDTO, error)
	listAttemptsFn func(
		ctx context.Context, userID uuid.UUID, limit, offset int32,
	) ([]service.ExamAttemptDTO, int64, error)
	listFixedTestsFn func(
		ctx context.Context, userID, versionID uuid.UUID,
	) (*service.FixedTestListResponseDTO, error)
}

func (f *fakeExamService) ListFixedTests(
	ctx context.Context, userID, versionID uuid.UUID,
) (*service.FixedTestListResponseDTO, error) {
	if f.listFixedTestsFn != nil {
		return f.listFixedTestsFn(ctx, userID, versionID)
	}
	return nil, nil
}

func (f *fakeExamService) ListExams(ctx context.Context) ([]service.ExamDTO, error) {
	if f.listExamsFn != nil {
		return f.listExamsFn(ctx)
	}
	return nil, nil
}

func (f *fakeExamService) StartSitting(
	ctx context.Context, userID, examID uuid.UUID, req service.StartAttemptRequest,
) (*service.ExamAttemptDTO, error) {
	if f.startSittingFn != nil {
		return f.startSittingFn(ctx, userID, examID, req)
	}
	return nil, apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func (f *fakeExamService) GetExamAttempt(
	ctx context.Context, userID, attemptID uuid.UUID,
) (*service.ExamAttemptDTO, error) {
	if f.getAttemptFn != nil {
		return f.getAttemptFn(ctx, userID, attemptID)
	}
	return nil, apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func (f *fakeExamService) AutosaveAnswers(
	ctx context.Context, userID, attemptID uuid.UUID, req service.SaveAnswersRequest,
) (*service.SaveAnswersResult, error) {
	if f.autosaveFn != nil {
		return f.autosaveFn(ctx, userID, attemptID, req)
	}
	return nil, apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func (f *fakeExamService) CompleteSection(
	ctx context.Context, userID, attemptID uuid.UUID, sectionNum int,
) (*service.CompleteSectionResult, error) {
	if f.completeSectionFn != nil {
		return f.completeSectionFn(ctx, userID, attemptID, sectionNum)
	}
	return nil, apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func (f *fakeExamService) SubmitExam(
	ctx context.Context, userID, attemptID uuid.UUID, submittedBy string,
) (*service.SubmitExamResult, error) {
	if f.submitExamFn != nil {
		return f.submitExamFn(ctx, userID, attemptID, submittedBy)
	}
	return nil, apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func (f *fakeExamService) GetScoreReport(
	ctx context.Context, userID, attemptID uuid.UUID,
) (*service.ScoreReportDTO, error) {
	if f.getReportFn != nil {
		return f.getReportFn(ctx, userID, attemptID)
	}
	return nil, apperr.New(apperr.Internal, "NOT_IMPLEMENTED", "not implemented")
}

func (f *fakeExamService) ListUserAttempts(
	ctx context.Context, userID uuid.UUID, limit, offset int32,
) ([]service.ExamAttemptDTO, int64, error) {
	if f.listAttemptsFn != nil {
		return f.listAttemptsFn(ctx, userID, limit, offset)
	}
	return nil, 0, nil
}

func (f *fakeExamService) SittingsToday(_ context.Context, _ uuid.UUID) (service.SittingsToday, error) {
	return service.SittingsToday{Used: 1, Limit: 5}, nil
}

func (f *fakeExamService) ListCurrentExamVersions(_ context.Context) ([]service.ExamVersionDTO, error) {
	return nil, nil
}

func (f *fakeExamService) ComposeMockTest(
	_ context.Context, _ *uuid.UUID, _ service.ComposeMockTestRequest,
) (*domain.MockTest, error) {
	return nil, nil
}

func (f *fakeExamService) StartMockTestAttempt(
	_ context.Context, _, _ uuid.UUID,
) (*service.ExamAttemptDTO, error) {
	return nil, nil
}

func (f *fakeExamService) GetExamVersionCoverage(
	_ context.Context, _ uuid.UUID,
) (*service.ExamCoverageReportDTO, error) {
	return nil, nil
}

func withActor(r *http.Request, userID uuid.UUID) *http.Request {
	actor := httpx.Actor{
		UserID: userID,
		Role:   "user",
	}
	return r.WithContext(httpx.WithActor(r.Context(), actor))
}

func setupExamRouter(svc examhttp.ExamService) chi.Router {
	r := chi.NewRouter()
	h := examhttp.NewHandler(svc)
	h.Routes(r)
	return r
}

func TestListExams(t *testing.T) {
	svc := &fakeExamService{
		listExamsFn: func(_ context.Context) ([]service.ExamDTO, error) {
			return []service.ExamDTO{
				{
					ID:           uuid.New(),
					Slug:         "b1-full",
					TitleEn:      "B1 Full Mock",
					Level:        "B1",
					TotalMinutes: 75,
				},
			}, nil
		},
	}

	router := setupExamRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/exams", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var exams []service.ExamDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &exams))
	assert.Len(t, exams, 1)
	assert.Equal(t, "b1-full", exams[0].Slug)
}

func TestListFixedTests(t *testing.T) {
	versionID := uuid.New()
	svc := &fakeExamService{
		listFixedTestsFn: func(
			_ context.Context, _, vID uuid.UUID,
		) (*service.FixedTestListResponseDTO, error) {
			assert.Equal(t, versionID, vID)
			return &service.FixedTestListResponseDTO{
				Items: []service.FixedTestSummaryDTO{
					{ID: uuid.New(), Number: 1, Title: "Đề 1", QuestionCount: 200, Minutes: 120},
				},
			}, nil
		},
	}

	router := setupExamRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/exam-versions/"+versionID.String()+"/tests", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body service.FixedTestListResponseDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, 1, body.Items[0].Number)
	assert.Equal(t, "Đề 1", body.Items[0].Title)
}

func TestListFixedTests_InvalidID(t *testing.T) {
	router := setupExamRouter(&fakeExamService{})
	req := httptest.NewRequest(http.MethodGet, "/exam-versions/not-a-uuid/tests", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStartSitting_RequiresAuth(t *testing.T) {
	svc := &fakeExamService{}
	router := setupExamRouter(svc)
	examID := uuid.New()

	req := httptest.NewRequest(http.MethodPost, "/exams/"+examID.String()+"/attempts", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestStartSitting_Success(t *testing.T) {
	userID := uuid.New()
	examID := uuid.New()
	attemptID := uuid.New()

	svc := &fakeExamService{
		startSittingFn: func(
			_ context.Context, uID, eID uuid.UUID, _ service.StartAttemptRequest,
		) (*service.ExamAttemptDTO, error) {
			assert.Equal(t, userID, uID)
			assert.Equal(t, examID, eID)
			return &service.ExamAttemptDTO{
				ID:                    attemptID,
				ExamID:                examID,
				Mode:                  "exam",
				ChosenDurationMinutes: 75,
				CurrentSection:        1,
				Status:                "in_progress",
				RemainingSeconds:      4500,
			}, nil
		},
	}

	router := setupExamRouter(svc)
	body := `{"mode":"exam"}`
	req := httptest.NewRequest(http.MethodPost, "/exams/"+examID.String()+"/attempts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var created service.ExamAttemptDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, attemptID, created.ID)
	assert.Equal(t, 4500, created.RemainingSeconds)
}

func TestAutosaveAnswers(t *testing.T) {
	userID := uuid.New()
	attemptID := uuid.New()

	svc := &fakeExamService{
		autosaveFn: func(
			_ context.Context, uID, aID uuid.UUID, req service.SaveAnswersRequest,
		) (*service.SaveAnswersResult, error) {
			assert.Equal(t, userID, uID)
			assert.Equal(t, attemptID, aID)
			assert.Equal(t, 1, req.SectionNumber)
			return &service.SaveAnswersResult{
				Saved:            true,
				RemainingSeconds: 4200,
			}, nil
		},
	}

	router := setupExamRouter(svc)
	body := `{"section_number":1,"answers":{"q1":"val"}}`
	req := httptest.NewRequest(
		http.MethodPut, "/exam-attempts/"+attemptID.String()+"/answers", bytes.NewBufferString(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var res service.SaveAnswersResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	assert.True(t, res.Saved)
	assert.Equal(t, 4200, res.RemainingSeconds)
}

func TestCompleteSection(t *testing.T) {
	userID := uuid.New()
	attemptID := uuid.New()

	svc := &fakeExamService{
		completeSectionFn: func(_ context.Context, uID, aID uuid.UUID, secNum int) (*service.CompleteSectionResult, error) {
			assert.Equal(t, userID, uID)
			assert.Equal(t, attemptID, aID)
			assert.Equal(t, 1, secNum)
			return &service.CompleteSectionResult{
				CurrentSection:   2,
				RemainingSeconds: 3000,
			}, nil
		},
	}

	router := setupExamRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/exam-attempts/"+attemptID.String()+"/sections/1/complete", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var res service.CompleteSectionResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	assert.Equal(t, 2, res.CurrentSection)
}

func TestSubmitExam(t *testing.T) {
	userID := uuid.New()
	attemptID := uuid.New()

	svc := &fakeExamService{
		submitExamFn: func(_ context.Context, uID, aID uuid.UUID, _ string) (*service.SubmitExamResult, error) {
			assert.Equal(t, userID, uID)
			assert.Equal(t, attemptID, aID)
			return &service.SubmitExamResult{
				AttemptID:    attemptID,
				Status:       "completed",
				ReportStatus: "ready",
			}, nil
		},
	}

	router := setupExamRouter(svc)
	req := httptest.NewRequest(http.MethodPost, "/exam-attempts/"+attemptID.String()+"/submit", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	var res service.SubmitExamResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	assert.Equal(t, "completed", res.Status)
}

func TestGetScoreReport(t *testing.T) {
	userID := uuid.New()
	attemptID := uuid.New()

	svc := &fakeExamService{
		getReportFn: func(_ context.Context, uID, aID uuid.UUID) (*service.ScoreReportDTO, error) {
			assert.Equal(t, userID, uID)
			assert.Equal(t, attemptID, aID)
			return &service.ScoreReportDTO{
				AttemptID:    attemptID,
				Status:       "completed",
				OverallScore: 82.0,
				OverallBand:  "B2",
				Disclaimer:   service.OfficialDisclaimer,
			}, nil
		},
	}

	router := setupExamRouter(svc)
	req := httptest.NewRequest(http.MethodGet, "/exam-attempts/"+attemptID.String()+"/report", nil)
	req = withActor(req, userID)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var rep service.ScoreReportDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rep))
	assert.Equal(t, 82.0, rep.OverallScore)
	assert.Equal(t, "B2", rep.OverallBand)
	assert.Equal(t, service.OfficialDisclaimer, rep.Disclaimer)
}

//go:build contract

package http_test

import (
	"net/http"
	"testing"

	admindomain "github.com/fluentra/fluentra/internal/modules/admin/domain"
	"github.com/fluentra/fluentra/internal/shared/httpx"
)

func TestContract_BannersMatchTheSpec(t *testing.T) {
	spec := loadSpec(t)
	admin := &httpx.Actor{UserID: testAdminID, Role: roleAdmin}
	learner := &httpx.Actor{UserID: testTargetID, Role: roleLearner}

	full := sampleBanner()
	bare := admindomain.Banner{
		ID: testBannerID, Title: "Maintenance tonight", Tone: admindomain.BannerToneWarning,
		CreatedBy: testAdminID, CreatedAt: full.CreatedAt, UpdatedAt: full.UpdatedAt,
	}

	svc := &fakeAdminService{}
	svc.banner.list = []admindomain.Banner{full, bare}
	svc.banner.result = bare
	srv := newServer(svc, newFakeGuard(permFlags))

	cases := []struct {
		name, method, path, specPath, body string
		status                             int
		actor                              *httpx.Actor
	}{
		{"admin list", http.MethodGet, pathBanners, "/admin/banners", "", http.StatusOK, admin},
		{"create", http.MethodPost, pathBanners, "/admin/banners", validBannerBody, http.StatusCreated, admin},
		{
			"update", http.MethodPut, pathBanners + "/" + testBannerID.String(), "/admin/banners/{id}",
			validBannerBody, http.StatusOK, admin,
		},
		{"learner active list", http.MethodGet, pathActive, "/banners/active", "", http.StatusOK, learner},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(srv, tc.method, tc.path, tc.body, tc.actor)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.status, rec.Body)
			}
			assertMatchesSchema(t, responseSchema(t, spec, tc.specPath, tc.method, tc.status), rec.Body.Bytes())
		})
	}
}

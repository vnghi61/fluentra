package domain_test

import (
	"testing"

	"github.com/fluentra/fluentra/internal/modules/content/domain"
)

func TestHasProvenance_TellsMachineBodiesFromHumanOnes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "a generated body",
			body: `{"prompt":"x","_provenance":{"prompt_version":"item_generate.v1","model":"m"}}`,
			want: true,
		},
		{
			name: "a human draft",
			body: `{"prompt":"x","correct_option_id":"A"}`,
			want: false,
		},
		{
			name: "an empty provenance object",
			body: `{"prompt":"x","_provenance":{}}`,
			want: false,
		},
		{
			name: "not an object",
			body: `[]`,
			want: false,
		},
		{
			name: "empty",
			body: ``,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.HasProvenance([]byte(tc.body)); got != tc.want {
				t.Errorf("HasProvenance(%s) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

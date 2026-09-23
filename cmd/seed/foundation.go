package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/cmd/internal/foundationfixture"
	"github.com/fluentra/fluentra/internal/modules/content"
	contentcontract "github.com/fluentra/fluentra/internal/modules/content/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// defaultFoundationFixtureDir is where `cmd/foundation -export` writes the
// frozen content this loader reads (WO 22 Stage G).
const defaultFoundationFixtureDir = "db/fixtures/foundation"

// seedFoundationFixtures loads the frozen Foundation content the export wrote
// (WO 22 Stage G). No model call: the content and its approval are already in
// the file, so `make seed` is fast, offline and the same on every machine.
//
// Idempotent on the slug and body: a node whose published content already
// matches the fixture writes nothing, so a second seed does not churn versions.
func seedFoundationFixtures(
	ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, dir string, out io.Writer,
) error {
	files, err := foundationfixture.ReadAll(dir)
	if err != nil {
		return fmt.Errorf("read foundation fixtures: %w", err)
	}
	if len(files) == 0 {
		_, _ = fmt.Fprintf(out,
			"No Foundation fixtures in %s; the curriculum will have no topic content "+
				"until `cmd/foundation -all` and `-export` have run.\n", dir)
		return nil
	}

	// The content module, not raw SQL: the fixture is loaded through the same
	// draft -> verified -> published state machine the API uses, so the rows are
	// a state the product could also have produced.
	contentMod := content.NewAuthoring(content.Deps{Pool: pool, Clock: clock.Real{}})
	author := contentMod.Author()
	recorder := contentMod.VerificationRecorder()

	published := 0
	drafted := 0
	for _, file := range files {
		for _, item := range orderedFixtureItems(file.Items) {
			body, found, err := publishedBodyBySlug(ctx, pool, item.Slug)
			if err != nil {
				return fmt.Errorf("check published %s: %w", item.Slug, err)
			}
			if found && jsonBodiesEqual(body, item.Body) {
				continue
			}

			spec := contentcontract.AuthorSpec{
				Slug:      item.Slug,
				Kind:      item.Kind,
				CEFRLevel: item.CEFRLevel,
				Body:      item.Body,
				AuthorID:  adminID,
				Tags:      []contentcontract.TagRef{{Namespace: file.Node.Namespace, Code: file.Node.Code}},
			}
			versionID, err := author.EnsureDraft(ctx, spec)
			if err != nil {
				return fmt.Errorf("author %s: %w", item.Slug, err)
			}

			verification := contentcontract.Verification{
				Confirmed: item.Verification.Confirmed,
				Model:     item.Verification.Model,
				Reason:    item.Verification.Reason,
				CheckedAt: item.Verification.CheckedAt,
			}
			if !verification.Confirmed {
				// A doubted item ships as a draft for a person, exactly as it
				// would have on the machine that generated it.
				if err := recorder.RecordVerification(ctx, versionID, verification); err != nil {
					return fmt.Errorf("record doubt for %s: %w", item.Slug, err)
				}
				drafted++
				continue
			}
			if err := author.ApproveVerified(ctx, versionID, verification); err != nil {
				return fmt.Errorf("publish %s: %w", item.Slug, err)
			}
			published++
		}
	}

	_, _ = fmt.Fprintf(out, "  ✓ Foundation fixtures: %d published, %d left as drafts, from %d node file(s)\n",
		published, drafted, len(files))
	return nil
}

// orderedFixtureItems sorts a node's items so a topic publishes after its
// exercises and quiz: the topic's own publication gate requires them.
func orderedFixtureItems(items []foundationfixture.Item) []foundationfixture.Item {
	ordered := append([]foundationfixture.Item(nil), items...)
	// Stable and tiny: topics last, quiz/review before them, exercises first.
	rank := func(item foundationfixture.Item) int {
		switch {
		case item.Kind == "foundation_topic":
			return 3
		case strings.HasPrefix(item.Kind, "foundation_"):
			return 2
		default:
			return 1
		}
	}
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && rank(ordered[j]) < rank(ordered[j-1]); j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	return ordered
}

// publishedBodyBySlug reads the body of the item's currently published version.
func publishedBodyBySlug(ctx context.Context, pool *pgxpool.Pool, slug string) ([]byte, bool, error) {
	const query = `
		SELECT v.body
		FROM content.content_items i
		JOIN content.content_versions v ON v.id = i.current_version_id
		WHERE i.slug = $1 AND i.status = 'published' AND v.status = 'published'`

	var body []byte
	err := pool.QueryRow(ctx, query, slug).Scan(&body)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return body, true, nil
}

// jsonBodiesEqual compares two bodies as decoded JSON, since Postgres stores
// jsonb with its own key order and whitespace.
func jsonBodiesEqual(a, b []byte) bool {
	var decodedA, decodedB any
	if json.Unmarshal(a, &decodedA) != nil || json.Unmarshal(b, &decodedB) != nil {
		return false
	}
	encodedA, errA := json.Marshal(decodedA)
	encodedB, errB := json.Marshal(decodedB)
	if errA != nil || errB != nil {
		return false
	}
	return bytes.Equal(encodedA, encodedB)
}

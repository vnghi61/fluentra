package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/cmd/internal/foundationfixture"
	learningcontract "github.com/fluentra/fluentra/internal/modules/learning/contract"
)

// exportFoundationFixtures writes every node's published Foundation content to
// one file per node, ready for `cmd/seed -foundation` to load offline.
//
// A node with no published content is named in the output rather than skipped
// silently: a course that lost a lesson is worse than a seed that says so.
func exportFoundationFixtures(
	ctx context.Context, pool *pgxpool.Pool, nodes []spineNodeRow, dir string, out io.Writer,
) error {
	var empty []string
	for _, node := range nodes {
		items, err := publishedItemsForNode(ctx, pool, node.Code, node.Namespace)
		if err != nil {
			return fmt.Errorf("read published content for %s: %w", node.Code, err)
		}
		if len(items) == 0 {
			empty = append(empty, node.Code)
		}

		file := foundationfixture.File{
			Format:      foundationfixture.Format,
			GeneratedAt: time.Now().UTC(),
			Node: foundationfixture.Node{
				Namespace: node.Namespace,
				Code:      node.Code,
				Label:     node.Label,
				CEFRLevel: node.CEFRLevel,
			},
			Items: items,
		}
		if err := foundationfixture.Write(dir, file); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "  ✓ %-32s %d item(s)\n", node.Code, len(items))
	}

	if len(empty) > 0 {
		_, _ = fmt.Fprintf(out, "\n%d node(s) have no published content:\n  %s\n",
			len(empty), strings.Join(empty, "\n  "))
	}
	return nil
}

// publishedItemsForNode reads the node's published versions and lifts the
// verifier's marking out of each body's provenance.
func publishedItemsForNode(
	ctx context.Context, pool *pgxpool.Pool, code, namespace string,
) ([]foundationfixture.Item, error) {
	const query = `
		SELECT i.slug, v.kind, v.cefr_level, v.body
		FROM content.content_items i
		JOIN content.content_versions v ON v.id = i.current_version_id
		JOIN content.content_tags ct ON ct.item_id = i.id
		JOIN content.taxonomies t ON t.id = ct.taxonomy_id
		WHERE t.code = $1 AND t.namespace = $2
		  AND i.status = 'published' AND v.status = 'published'
		ORDER BY v.kind, i.slug`

	rows, err := pool.Query(ctx, query, code, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []foundationfixture.Item
	for rows.Next() {
		var item foundationfixture.Item
		var body []byte
		if err := rows.Scan(&item.Slug, &item.Kind, &item.CEFRLevel, &body); err != nil {
			return nil, err
		}
		item.Body = body
		item.Verification = verificationFromBody(body)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		return fixtureKindOrder(items[i].Kind) < fixtureKindOrder(items[j].Kind)
	})
	return items, nil
}

// verificationFromBody lifts `_provenance.verification` out of a body.
func verificationFromBody(body []byte) foundationfixture.Verify {
	var decoded struct {
		Provenance struct {
			Verification struct {
				Model     string `json:"model"`
				Verdict   string `json:"verdict"`
				CheckedAt string `json:"checked_at"`
				Reason    string `json:"reason"`
			} `json:"verification"`
		} `json:"_provenance"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return foundationfixture.Verify{}
	}
	verify := foundationfixture.Verify{
		Confirmed: decoded.Provenance.Verification.Verdict == "confirmed",
		Model:     decoded.Provenance.Verification.Model,
		Reason:    decoded.Provenance.Verification.Reason,
	}
	if checkedAt, err := time.Parse(time.RFC3339, decoded.Provenance.Verification.CheckedAt); err == nil {
		verify.CheckedAt = checkedAt
	}
	return verify
}

// fixtureKindOrder publishes a topic after its exercises and quiz, because a
// topic does not publish until its node carries them (BR-FOUNDATION-05).
func fixtureKindOrder(kind string) int {
	switch kind {
	case learningcontract.KindFoundationTopic:
		return 3
	case learningcontract.KindFoundationQuiz, learningcontract.KindFoundationReview:
		return 2
	default:
		return 1
	}
}

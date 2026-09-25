// Command examgen freezes an exam's published questions into a fixture and, in
// time, generates them (WO 22 Stage N).
//
// Generate once, verify once, freeze into `db/fixtures/exams/`, and let
// `make seed` load it offline. Generating on every machine would make the seed
// depend on a model that is slow, paid and sometimes down, and two machines
// would seed different tests.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/cmd/internal/examfixture"
	"github.com/fluentra/fluentra/internal/shared/config"
)

// defaultFixtureDir is where `cmd/examgen -export` writes and `cmd/seed -exams`
// reads.
const defaultFixtureDir = "db/fixtures/exams"

type examCLIConfig struct {
	App struct {
		Environment string `koanf:"environment"`
	} `koanf:"app"`
	// "db", not "database": the key is db.dsn, and a struct tag that did not
	// match it left the DSN empty whatever DB_DSN said.
	Database struct {
		DSN string `koanf:"dsn"`
	} `koanf:"db"`
	AI aiConfig `koanf:"ai"`
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "examgen error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("examgen", flag.ContinueOnError)
	examFlag := flags.String("exam", "", "Exam version code to work on, e.g. TOEIC_LR_2026")
	testsFlag := flags.Int("tests", 5, "How many disjoint tests to generate toward")
	exportFlag := flags.Bool("export", false, "Export published questions to the fixture and exit")
	fixturesFlag := flags.String("fixtures", defaultFixtureDir,
		"Directory an -export writes to and `cmd/seed -exams` reads")
	dryRunFlag := flags.Bool("dry-run", false, "Print what would happen without writing")
	mockFlag := flags.Bool("mock", false,
		"Generate with the offline mock provider, writing placeholder questions (a throwaway database only)")

	if err := flags.Parse(args); err != nil {
		return err
	}

	examCode := strings.TrimSpace(*examFlag)
	if examCode == "" {
		return errors.New("must specify -exam CODE")
	}

	cfg, err := loadConfig(ctx)
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.Database.DSN)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if *exportFlag {
		return exportExam(ctx, pool, examCode, *fixturesFlag, *dryRunFlag, out)
	}
	if len(cfg.AI.providers()) == 0 && !*mockFlag {
		return errors.New(
			"no AI provider is configured, so this would fill the bank with mock questions: " +
				"set AI_PROVIDER_1_NAME and AI_PROVIDER_1_API_KEY (and a second provider of a " +
				"different model with AI_AUTO_PUBLISH=true, so the verifier can publish), or pass -mock")
	}
	return generateTests(ctx, cfg, pool, examCode, *testsFlag, *dryRunFlag, out)
}

func exportExam(
	ctx context.Context, pool *pgxpool.Pool, examCode, dir string, dryRun bool, out io.Writer,
) error {
	items, versionCode, err := publishedExamItems(ctx, pool, examCode)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("exam %s has no published questions to export", examCode)
	}

	file := &examfixture.ExamFile{
		Format:      examfixture.ExamFormat,
		Exam:        strings.ToLower(strings.SplitN(examCode, "_", 2)[0]),
		ExamVersion: versionCode,
		GeneratedAt: time.Now().UTC(),
		Items:       items,
	}
	if dryRun {
		_, _ = fmt.Fprintf(out, "Would export %d question(s) for %s to %s\n", len(items), examCode, dir)
		return nil
	}
	path, err := examfixture.WriteExamFile(dir, file)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "  ✓ %s: %d question(s) -> %s\n", examCode, len(items), path)
	return nil
}

// publishedExamItems reads the exam's published questions and the verifier's
// marking out of each body's provenance.
func publishedExamItems(
	ctx context.Context, pool *pgxpool.Pool, examCode string,
) ([]examfixture.ExamItem, string, error) {
	const query = `
		SELECT i.slug, v.kind, v.cefr_level, q.skill, q.exam_part_id, q.activity_id, v.body, ev.code
		FROM assess.questions q
		JOIN content.content_items i ON i.id = q.content_item_id
		JOIN content.content_versions v ON v.id = i.current_version_id
		JOIN assess.exam_parts p ON p.id = q.exam_part_id
		JOIN assess.exam_versions ev ON ev.id = p.version_id
		WHERE ev.code = $1 AND q.status = 'published' AND v.status = 'published'
		ORDER BY p.section, p.part_number, i.slug`

	rows, err := pool.Query(ctx, query, examCode)
	if err != nil {
		return nil, "", fmt.Errorf("query published questions: %w", err)
	}
	defer rows.Close()

	var items []examfixture.ExamItem
	versionCode := examCode
	for rows.Next() {
		var item examfixture.ExamItem
		var partID, activityID *string
		var body []byte
		if err := rows.Scan(&item.Slug, &item.Kind, &item.CEFRLevel, &item.Skill,
			&partID, &activityID, &body, &versionCode); err != nil {
			return nil, "", fmt.Errorf("scan question: %w", err)
		}
		if partID != nil {
			item.ExamPartID = *partID
		}
		if activityID != nil {
			item.Group = *activityID
		}
		item.ExamVersion = versionCode
		item.Body = body
		item.Verification = verificationFromBody(body)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	return items, versionCode, nil
}

// verificationFromBody lifts the verifier's marking out of a body's provenance.
func verificationFromBody(body []byte) examfixture.Verification {
	var parsed struct {
		Provenance struct {
			Verification struct {
				Model     string `json:"model"`
				Verdict   string `json:"verdict"`
				CheckedAt string `json:"checked_at"`
			} `json:"verification"`
		} `json:"_provenance"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return examfixture.Verification{ApprovedBy: examfixture.ApprovedByPerson}
	}
	v := parsed.Provenance.Verification
	checkedAt, _ := time.Parse(time.RFC3339, v.CheckedAt)
	confirmed := strings.EqualFold(v.Verdict, "confirmed")
	// The export holds published questions only: one the verifier did not
	// confirm was published by a person.
	approvedBy := examfixture.ApprovedByPerson
	if confirmed {
		approvedBy = examfixture.ApprovedByVerifier
	}
	return examfixture.Verification{
		ApprovedBy: approvedBy,
		Confirmed:  confirmed,
		Model:      v.Model,
		CheckedAt:  checkedAt,
	}
}

// loadConfig reads this command's configuration. Every key it may read is
// declared, because config.Load drops any key a command does not.
func loadConfig(ctx context.Context) (examCLIConfig, error) {
	var cfg examCLIConfig
	defaults := map[string]any{"app.environment": "local"}
	for key, value := range aiDefaults() {
		defaults[key] = value
	}
	if err := config.Load(ctx, config.Options{
		Defaults: defaults,
		Required: []config.RequiredKey{
			{Name: "db.dsn", DocSection: "docs/deployment/configuration.md#database"},
		},
	}, &cfg); err != nil {
		return cfg, fmt.Errorf("load examgen configuration: %w", err)
	}
	return cfg, nil
}

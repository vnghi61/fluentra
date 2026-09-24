package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fluentra/fluentra/internal/modules/content"
	"github.com/fluentra/fluentra/internal/modules/lesson"
	lessoncontract "github.com/fluentra/fluentra/internal/modules/lesson/contract"
	"github.com/fluentra/fluentra/internal/shared/clock"
)

// The thirteen Foundation course slugs, as content.foundation_course_nodes
// spells them. Constants so a typo is a compile error, not a course that
// silently gets no lessons.
const (
	courseGrammarFoundations         = "grammar-foundations"
	courseEnglishTenses              = "english-tenses"
	courseSentenceStructure          = "sentence-structure"
	courseClauses                    = "clauses"
	courseSentencePatterns           = "sentence-patterns"
	courseVocabularyFoundations      = "vocabulary-foundations"
	courseWordFormation              = "word-formation"
	coursePhrasalVerbsAndExpressions = "phrasal-verbs-and-expressions"
	coursePronunciationFoundations   = "pronunciation-foundations"
	courseReadingFoundations         = "reading-foundations"
	courseListeningFoundations       = "listening-foundations"
	courseSpeakingFoundations        = "speaking-foundations"
	courseWritingFoundations         = "writing-foundations"
	namespaceSkill                   = "skill"
	namespacePattern                 = "pattern"
	namespacePronunciation           = "pronunciation"
)

// foundationCourseMeta is the card each Foundation course carries. The schema
// holds one title and description, so these are English; the Vietnamese cards
// come from the catalogue's own labels (WO 22 D22-12).
type foundationCourseMeta struct {
	Title       string
	Description string
	CEFRFrom    string
	CEFRTo      string
	Hours       int
}

// foundationCourseMetaBySlug is every course the map in
// content.foundation_course_nodes groups nodes into. A slug in the map with no
// metadata here is refused rather than seeded with a placeholder title.
var foundationCourseMetaBySlug = map[string]foundationCourseMeta{
	courseGrammarFoundations: {
		"Grammar Foundations", "Parts of speech, nouns, articles, verbs, clauses of agreement.",
		"A1", "B1", 20,
	},
	courseEnglishTenses: {
		"English Tenses", "The twelve tenses, one lesson each, from present simple to future perfect continuous.",
		"A1", "B1", 16,
	},
	courseSentenceStructure: {
		"Sentence Structure", "Subjects, verbs and objects, questions and negatives.",
		"A1", "A2", 8,
	},
	courseClauses: {
		"Clauses", "Relative, noun and adverbial clauses and how they join.",
		"A2", "B1", 10,
	},
	courseSentencePatterns: {
		"Sentence Patterns", "Everyday patterns: introducing, asking, requesting, agreeing, explaining.",
		"A1", "A2", 14,
	},
	courseVocabularyFoundations: {
		"Vocabulary Foundations", "High-frequency words, collocations, synonyms and topic vocabulary.",
		"A1", "B1", 14,
	},
	courseWordFormation: {
		"Word Formation", "Prefixes, suffixes, families, conversion and compounds.",
		"A2", "B1", 8,
	},
	coursePhrasalVerbsAndExpressions: {
		"Phrasal Verbs & Expressions", "Phrasal verbs, idioms and fixed spoken expressions.",
		"A2", "B1", 10,
	},
	coursePronunciationFoundations: {
		"Pronunciation Foundations", "Sounds, word and sentence stress, linking, reductions and intonation.",
		"A1", "B1", 12,
	},
	courseReadingFoundations: {
		"Reading Foundations", "Sentences, paragraphs, words in context and main idea and detail.",
		"A1", "B1", 12,
	},
	courseListeningFoundations: {
		"Listening Foundations", "Words, phrases, sentences and conversations at natural speed.",
		"A1", "B1", 12,
	},
	courseSpeakingFoundations: {
		"Speaking Foundations", "Words, phrases, sentences and speaking at length.",
		"A1", "B1", 12,
	},
	courseWritingFoundations: {
		"Writing Foundations", "Sentences, paragraphs and linking words.",
		"A1", "B1", 10,
	},
}

// The five Phase 2 course slugs, archived so the catalogue shows thirteen.
const (
	phase2EverydayEnglish   = "everyday-english-a2-b1"
	phase2ReadingPractice   = "reading-practice"
	phase2WritingPractice   = "writing-practice"
	phase2SpeakingPractice  = "speaking-practice"
	phase2ListeningPractice = "listening-practice"
)

// phase2CourseSlugs are the five courses this seed built before the thirteen
// Foundation courses replaced them (WO 22 D22-14). They are archived, never
// deleted: a learner's attempts point at their lessons.
var phase2CourseSlugs = []string{
	phase2EverydayEnglish,
	phase2ReadingPractice,
	phase2WritingPractice,
	phase2SpeakingPractice,
	phase2ListeningPractice,
}

// foundationCourseNode is one row of the course map.
type foundationCourseNode struct {
	courseSlug string
	nodeID     uuid.UUID
	namespace  string
	code       string
	label      string
	cefr       string
	position   int
}

// seedFoundationCourses builds the thirteen Foundation courses from
// content.foundation_course_nodes, one lesson per node, in prerequisite order,
// through lesson's Author — never by writing learn.* myself (WO 22 Stage H).
func seedFoundationCourses(
	ctx context.Context, pool *pgxpool.Pool, adminID uuid.UUID, out io.Writer,
) error {
	nodes, err := foundationCourseNodes(ctx, pool)
	if err != nil {
		return fmt.Errorf("read the course map: %w", err)
	}
	if len(nodes) == 0 {
		_, _ = fmt.Fprintln(out, "No Foundation course map found; run the migrations first.")
		return nil
	}

	contentMod := content.NewAuthoring(content.Deps{Pool: pool, Clock: clock.Real{}})
	lessonMod := lesson.New(lesson.Deps{
		Pool:    pool,
		Clock:   clock.Real{},
		Guard:   offlineGuard{},
		Content: contentMod.Reader(),
	})
	author := lessonMod.Author()

	order, byCourse := groupByCourse(nodes)
	for _, slug := range order {
		meta, ok := foundationCourseMetaBySlug[slug]
		if !ok {
			return fmt.Errorf("course %q is in the map but has no card metadata", slug)
		}
		courseNodes := byCourse[slug]
		owner := adminID
		courseID, err := author.EnsureCourse(ctx, lessoncontract.CourseSpec{
			Slug:           slug,
			Title:          meta.Title,
			Description:    meta.Description,
			CEFRFrom:       meta.CEFRFrom,
			CEFRTo:         meta.CEFRTo,
			EstimatedHours: meta.Hours,
			Origin:         "foundation",
			Visibility:     "public",
			OwnerID:        &owner,
		})
		if err != nil {
			return fmt.Errorf("course %s: %w", slug, err)
		}
		unitID, err := author.EnsureUnit(ctx, lessoncontract.UnitSpec{
			CourseID: courseID, Position: 1, Title: meta.Title,
		})
		if err != nil {
			return fmt.Errorf("unit for %s: %w", slug, err)
		}

		withActivities := 0
		for _, node := range courseNodes {
			cefr := node.cefr
			lessonID, err := author.EnsureLesson(ctx, lessoncontract.LessonSpec{
				UnitID:           unitID,
				Position:         node.position,
				Title:            node.label,
				SkillFocus:       skillFocusFor(node),
				EstimatedMinutes: 15,
				CEFRLevel:        &cefr,
			})
			if err != nil {
				return fmt.Errorf("lesson %s: %w", node.code, err)
			}
			activities, err := publishedNodeActivities(ctx, pool, node.nodeID)
			if err != nil {
				return fmt.Errorf("activities for %s: %w", node.code, err)
			}
			if len(activities) == 0 {
				continue
			}
			if err := author.SyncActivities(ctx, lessonID, activities); err != nil {
				return fmt.Errorf("activities for %s: %w", node.code, err)
			}
			withActivities++
		}
		_, _ = fmt.Fprintf(out, "  ✓ Course: %s (%d lessons, %d with content)\n",
			meta.Title, len(courseNodes), withActivities)
	}

	if err := archivePhase2Courses(ctx, pool); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out,
		"  ✓ %d Foundation courses in the catalogue; the five Phase 2 courses are archived\n", len(order))
	return nil
}

// groupByCourse returns the course slugs in first-seen order and the nodes of
// each, already ordered by the map's position.
func groupByCourse(nodes []foundationCourseNode) ([]string, map[string][]foundationCourseNode) {
	var order []string
	byCourse := make(map[string][]foundationCourseNode)
	for _, node := range nodes {
		if _, seen := byCourse[node.courseSlug]; !seen {
			order = append(order, node.courseSlug)
		}
		byCourse[node.courseSlug] = append(byCourse[node.courseSlug], node)
	}
	return order, byCourse
}

// skillFocusFor names the skill a lesson works on, for the lesson's badge.
func skillFocusFor(node foundationCourseNode) string {
	if node.namespace != namespaceSkill {
		return node.namespace
	}
	// A skill node's code is SKILL_STEP ("LISTENING_WORDS"), so the skill is the
	// part before the first underscore.
	if i := strings.IndexByte(node.code, '_'); i > 0 {
		return strings.ToLower(node.code[:i])
	}
	return strings.ToLower(node.code)
}

// foundationCourseNodes reads the course map with each node's card details.
func foundationCourseNodes(ctx context.Context, pool *pgxpool.Pool) ([]foundationCourseNode, error) {
	const query = `
		SELECT m.course_slug, t.id, t.namespace, t.code, t.label, t.cefr_level, m.position
		FROM content.foundation_course_nodes m
		JOIN content.taxonomies t ON t.id = m.node_id
		ORDER BY m.course_slug, m.position`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []foundationCourseNode
	for rows.Next() {
		var node foundationCourseNode
		if err := rows.Scan(&node.courseSlug, &node.nodeID, &node.namespace,
			&node.code, &node.label, &node.cefr, &node.position); err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

// publishedNodeActivities reads a node's published content as lesson activities:
// the topic first (ungraded), then the exercises, then the quiz.
func publishedNodeActivities(
	ctx context.Context, pool *pgxpool.Pool, nodeID uuid.UUID,
) ([]lessoncontract.ActivitySpec, error) {
	const query = `
		SELECT v.id, v.kind, v.body
		FROM content.content_items i
		JOIN content.content_versions v ON v.id = i.current_version_id
		JOIN content.content_tags ct ON ct.item_id = i.id
		WHERE ct.taxonomy_id = $1 AND i.status = 'published' AND v.status = 'published'
		ORDER BY CASE v.kind
			WHEN 'foundation_topic' THEN 0
			WHEN 'foundation_quiz' THEN 2
			WHEN 'foundation_review' THEN 3
			ELSE 1 END, i.slug`

	rows, err := pool.Query(ctx, query, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var activities []lessoncontract.ActivitySpec
	position := 0
	for rows.Next() {
		var versionID uuid.UUID
		var kind string
		var body []byte
		if err := rows.Scan(&versionID, &kind, &body); err != nil {
			return nil, err
		}
		position++
		weight := 1
		if kind == "foundation_topic" {
			weight = 0
		}
		activities = append(activities, lessoncontract.ActivitySpec{
			Position:         position,
			Kind:             kind,
			ContentVersionID: versionID,
			Config:           json.RawMessage(body),
			Weight:           weight,
		})
	}
	return activities, rows.Err()
}

// archivePhase2Courses takes the five Phase 2 courses out of the catalogue
// without deleting them: a learner's attempts point at their lessons.
func archivePhase2Courses(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		UPDATE learn.courses SET status = 'archived', updated_at = now()
		WHERE slug = ANY($1)`, phase2CourseSlugs)
	if err != nil {
		return fmt.Errorf("archive the Phase 2 courses: %w", err)
	}
	return nil
}

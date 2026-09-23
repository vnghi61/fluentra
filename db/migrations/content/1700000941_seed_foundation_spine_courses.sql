-- +goose Up
-- +goose StatementBegin

-- WO 22 Stage E: the spine the thirteen foundation courses need. Twenty-three
-- new nodes — word formation, expressions and one progression per skill — and
-- the course map every node belongs to exactly one course in.

-- 1. Word formation (vocabulary, 5).
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position) VALUES
    ('vocabulary', 'PREFIXES', 'Prefixes', 'Word beginnings that change a word''s meaning: un-, re-, dis-, pre-.', 'A2', 9),
    ('vocabulary', 'SUFFIXES', 'Suffixes', 'Word endings that change a word''s class: -ness, -ful, -ly, -ment.', 'A2', 10),
    ('vocabulary', 'WORD_FAMILIES', 'Word Families', 'The noun, verb, adjective and adverb forms of one root.', 'B1', 11),
    ('vocabulary', 'CONVERSION', 'Conversion', 'Words that change class without changing form: a book, to book.', 'B1', 12),
    ('vocabulary', 'COMPOUND_WORDS', 'Compound Words', 'Two words joined into one meaning: bedroom, well-known.', 'B1', 13)
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position;

-- 2. Expressions (vocabulary, 3).
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position) VALUES
    ('vocabulary', 'IDIOMS', 'Idioms', 'Fixed phrases whose meaning is not the sum of their words.', 'B1', 14),
    ('vocabulary', 'FIXED_EXPRESSIONS', 'Fixed Expressions', 'Set phrases used as single units: as soon as, in order to.', 'B1', 15),
    ('vocabulary', 'SPOKEN_EXPRESSIONS', 'Spoken Expressions', 'Everyday spoken formulas: never mind, by the way, you know.', 'A2', 16)
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position;

-- 3. Skill progressions (skill, 15): four steps for listening and speaking,
-- four for reading, three for writing, each chained from its skill root.
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position, parent_id) VALUES
    ('skill', 'LISTENING_WORDS', 'Listening: Words', 'Hearing and recognising single words in connected speech.', 'A1', 5,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'LISTENING')),
    ('skill', 'LISTENING_PHRASES', 'Listening: Phrases', 'Recognising common phrases and chunks as they are spoken.', 'A2', 6,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'LISTENING')),
    ('skill', 'LISTENING_SENTENCES', 'Listening: Sentences', 'Following whole sentences and their meaning at natural speed.', 'A2', 7,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'LISTENING')),
    ('skill', 'LISTENING_CONVERSATIONS', 'Listening: Conversations', 'Following a conversation between two or more speakers.', 'B1', 8,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'LISTENING')),
    ('skill', 'SPEAKING_WORDS', 'Speaking: Words', 'Saying single words clearly with correct sounds and stress.', 'A1', 9,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'SPEAKING')),
    ('skill', 'SPEAKING_PHRASES', 'Speaking: Phrases', 'Producing common phrases fluently and with natural rhythm.', 'A2', 10,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'SPEAKING')),
    ('skill', 'SPEAKING_SENTENCES', 'Speaking: Sentences', 'Building and saying complete sentences without long pauses.', 'A2', 11,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'SPEAKING')),
    ('skill', 'SPEAKING_PARAGRAPHS', 'Speaking: Paragraphs', 'Speaking at length, linking ideas across several sentences.', 'B1', 12,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'SPEAKING')),
    ('skill', 'READING_SENTENCES', 'Reading: Sentences', 'Unpacking the structure of a single written sentence.', 'A1', 13,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'READING')),
    ('skill', 'READING_PARAGRAPHS', 'Reading: Paragraphs', 'Finding the topic sentence and the shape of a paragraph.', 'A2', 14,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'READING')),
    ('skill', 'READING_VOCABULARY_IN_CONTEXT', 'Reading: Vocabulary in Context', 'Working out an unknown word''s meaning from its sentence.', 'B1', 15,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'READING')),
    ('skill', 'READING_MAIN_IDEA_AND_DETAIL', 'Reading: Main Idea and Detail', 'Separating the main idea from supporting detail.', 'B1', 16,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'READING')),
    ('skill', 'WRITING_SENTENCES', 'Writing: Sentences', 'Writing grammatical, correctly punctuated sentences.', 'A1', 17,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'WRITING')),
    ('skill', 'WRITING_PARAGRAPHS', 'Writing: Paragraphs', 'Organising sentences into a clear, focused paragraph.', 'A2', 18,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'WRITING')),
    ('skill', 'WRITING_LINKING_WORDS', 'Writing: Linking Words', 'Connecting ideas with however, therefore, although, moreover.', 'B1', 19,
        (SELECT id FROM content.taxonomies WHERE namespace = 'skill' AND code = 'WRITING'))
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position,
    parent_id = EXCLUDED.parent_id;

-- 4. Prerequisites for the new nodes.
INSERT INTO content.taxonomy_prerequisites (node_id, requires_node_id)
SELECT n.id, r.id
FROM content.taxonomies n
JOIN content.taxonomies r ON r.namespace = n.namespace
WHERE (n.namespace = 'vocabulary' AND (
    (n.code = 'PREFIXES' AND r.code = 'HIGH_FREQUENCY_WORDS') OR
    (n.code = 'SUFFIXES' AND r.code = 'PREFIXES') OR
    (n.code = 'WORD_FAMILIES' AND r.code = 'SUFFIXES') OR
    (n.code = 'CONVERSION' AND r.code = 'WORD_FAMILIES') OR
    (n.code = 'COMPOUND_WORDS' AND r.code = 'SUFFIXES') OR
    (n.code = 'IDIOMS' AND r.code = 'COLLOCATIONS') OR
    (n.code = 'FIXED_EXPRESSIONS' AND r.code = 'COLLOCATIONS') OR
    (n.code = 'SPOKEN_EXPRESSIONS' AND r.code = 'IDIOMS')
))
OR (n.namespace = 'skill' AND (
    (n.code = 'LISTENING_WORDS' AND r.code = 'LISTENING') OR
    (n.code = 'LISTENING_PHRASES' AND r.code = 'LISTENING_WORDS') OR
    (n.code = 'LISTENING_SENTENCES' AND r.code = 'LISTENING_PHRASES') OR
    (n.code = 'LISTENING_CONVERSATIONS' AND r.code = 'LISTENING_SENTENCES') OR
    (n.code = 'SPEAKING_WORDS' AND r.code = 'SPEAKING') OR
    (n.code = 'SPEAKING_PHRASES' AND r.code = 'SPEAKING_WORDS') OR
    (n.code = 'SPEAKING_SENTENCES' AND r.code = 'SPEAKING_PHRASES') OR
    (n.code = 'SPEAKING_PARAGRAPHS' AND r.code = 'SPEAKING_SENTENCES') OR
    (n.code = 'READING_SENTENCES' AND r.code = 'READING') OR
    (n.code = 'READING_PARAGRAPHS' AND r.code = 'READING_SENTENCES') OR
    (n.code = 'READING_VOCABULARY_IN_CONTEXT' AND r.code = 'READING_PARAGRAPHS') OR
    (n.code = 'READING_MAIN_IDEA_AND_DETAIL' AND r.code = 'READING_PARAGRAPHS') OR
    (n.code = 'WRITING_SENTENCES' AND r.code = 'WRITING') OR
    (n.code = 'WRITING_PARAGRAPHS' AND r.code = 'WRITING_SENTENCES') OR
    (n.code = 'WRITING_LINKING_WORDS' AND r.code = 'WRITING_PARAGRAPHS')
))
ON CONFLICT (node_id, requires_node_id) DO NOTHING;

-- 5. The course map. One table the seed reads: every spine node belongs to
-- exactly one course, and a node in two courses is refused by the unique index.
CREATE TABLE IF NOT EXISTS content.foundation_course_nodes (
    course_slug text NOT NULL,
    node_id     uuid NOT NULL REFERENCES content.taxonomies (id) ON DELETE CASCADE,
    position    int  NOT NULL DEFAULT 0,
    PRIMARY KEY (course_slug, node_id),
    CONSTRAINT uq_foundation_course_nodes_node UNIQUE (node_id)
);

-- The map, so that Stage H's seed and the gate test read the same source.
INSERT INTO content.foundation_course_nodes (course_slug, node_id, position)
SELECT m.course_slug, n.id, m.position
FROM (VALUES
    -- Grammar Foundations
    ('grammar-foundations', 'PARTS_OF_SPEECH', 1),
    ('grammar-foundations', 'NOUNS', 2),
    ('grammar-foundations', 'ARTICLES', 3),
    ('grammar-foundations', 'PRONOUNS', 4),
    ('grammar-foundations', 'ADJECTIVES', 5),
    ('grammar-foundations', 'ADVERBS', 6),
    ('grammar-foundations', 'PREPOSITIONS', 7),
    ('grammar-foundations', 'CONJUNCTIONS', 8),
    ('grammar-foundations', 'MODAL_VERBS', 9),
    ('grammar-foundations', 'COMPARATIVES_AND_SUPERLATIVES', 10),
    ('grammar-foundations', 'PASSIVE_VOICE', 11),
    ('grammar-foundations', 'CONDITIONALS', 12),
    ('grammar-foundations', 'REPORTED_SPEECH', 13),
    ('grammar-foundations', 'GERUNDS_AND_INFINITIVES', 14),
    ('grammar-foundations', 'PARTICIPLES', 15),
    ('grammar-foundations', 'COMMON_GRAMMAR_MISTAKES', 16),
    -- English Tenses
    ('english-tenses', 'TENSES', 1),
    ('english-tenses', 'PRESENT_SIMPLE', 2),
    ('english-tenses', 'PRESENT_CONTINUOUS', 3),
    ('english-tenses', 'PRESENT_PERFECT', 4),
    ('english-tenses', 'PRESENT_PERFECT_CONTINUOUS', 5),
    ('english-tenses', 'PAST_SIMPLE', 6),
    ('english-tenses', 'PAST_CONTINUOUS', 7),
    ('english-tenses', 'PAST_PERFECT', 8),
    ('english-tenses', 'PAST_PERFECT_CONTINUOUS', 9),
    ('english-tenses', 'FUTURE_SIMPLE', 10),
    ('english-tenses', 'FUTURE_CONTINUOUS', 11),
    ('english-tenses', 'FUTURE_PERFECT', 12),
    ('english-tenses', 'FUTURE_PERFECT_CONTINUOUS', 13),
    -- Sentence Structure
    ('sentence-structure', 'SENTENCE_STRUCTURE', 1),
    ('sentence-structure', 'SUBJECT_VERB_OBJECT', 2),
    ('sentence-structure', 'QUESTIONS_AND_NEGATIVES', 3),
    -- Clauses
    ('clauses', 'CLAUSES', 1),
    ('clauses', 'RELATIVE_CLAUSES', 2),
    ('clauses', 'NOUN_CLAUSES', 3),
    ('clauses', 'ADVERBIAL_CLAUSES', 4),
    -- Sentence Patterns
    ('sentence-patterns', 'INTRODUCING_YOURSELF', 1),
    ('sentence-patterns', 'ASKING_QUESTIONS', 2),
    ('sentence-patterns', 'ANSWERING_QUESTIONS', 3),
    ('sentence-patterns', 'DAILY_CONVERSATIONS', 4),
    ('sentence-patterns', 'REQUESTING', 5),
    ('sentence-patterns', 'OFFERING', 6),
    ('sentence-patterns', 'SUGGESTING', 7),
    ('sentence-patterns', 'AGREEING_AND_DISAGREEING', 8),
    ('sentence-patterns', 'GIVING_OPINIONS', 9),
    ('sentence-patterns', 'DESCRIBING', 10),
    ('sentence-patterns', 'COMPARING', 11),
    ('sentence-patterns', 'EXPLAINING', 12),
    ('sentence-patterns', 'ASKING_FOR_CLARIFICATION', 13),
    -- Vocabulary Foundations
    ('vocabulary-foundations', 'ESSENTIAL_EVERYDAY', 1),
    ('vocabulary-foundations', 'COMMON_VERBS', 2),
    ('vocabulary-foundations', 'HIGH_FREQUENCY_WORDS', 3),
    ('vocabulary-foundations', 'TOPIC_VOCABULARY', 4),
    ('vocabulary-foundations', 'COLLOCATIONS', 5),
    ('vocabulary-foundations', 'SYNONYMS_AND_ANTONYMS', 6),
    ('vocabulary-foundations', 'ACADEMIC_VOCABULARY', 7),
    ('vocabulary-foundations', 'WORKPLACE_VOCABULARY', 8),
    -- Word Formation
    ('word-formation', 'PREFIXES', 1),
    ('word-formation', 'SUFFIXES', 2),
    ('word-formation', 'WORD_FAMILIES', 3),
    ('word-formation', 'CONVERSION', 4),
    ('word-formation', 'COMPOUND_WORDS', 5),
    -- Phrasal Verbs & Expressions
    ('phrasal-verbs-and-expressions', 'PHRASAL_VERBS', 1),
    ('phrasal-verbs-and-expressions', 'IDIOMS', 2),
    ('phrasal-verbs-and-expressions', 'FIXED_EXPRESSIONS', 3),
    ('phrasal-verbs-and-expressions', 'SPOKEN_EXPRESSIONS', 4),
    -- Pronunciation Foundations
    ('pronunciation-foundations', 'IPA_BASICS', 1),
    ('pronunciation-foundations', 'ENGLISH_SOUNDS', 2),
    ('pronunciation-foundations', 'WORD_STRESS', 3),
    ('pronunciation-foundations', 'SENTENCE_STRESS', 4),
    ('pronunciation-foundations', 'LINKING', 5),
    ('pronunciation-foundations', 'REDUCTIONS', 6),
    ('pronunciation-foundations', 'INTONATION', 7),
    ('pronunciation-foundations', 'COMMON_PRONUNCIATION_MISTAKES', 8),
    -- Skill Foundations
    ('listening-foundations', 'LISTENING', 1),
    ('listening-foundations', 'LISTENING_WORDS', 2),
    ('listening-foundations', 'LISTENING_PHRASES', 3),
    ('listening-foundations', 'LISTENING_SENTENCES', 4),
    ('listening-foundations', 'LISTENING_CONVERSATIONS', 5),
    ('speaking-foundations', 'SPEAKING', 1),
    ('speaking-foundations', 'SPEAKING_WORDS', 2),
    ('speaking-foundations', 'SPEAKING_PHRASES', 3),
    ('speaking-foundations', 'SPEAKING_SENTENCES', 4),
    ('speaking-foundations', 'SPEAKING_PARAGRAPHS', 5),
    ('reading-foundations', 'READING', 1),
    ('reading-foundations', 'READING_SENTENCES', 2),
    ('reading-foundations', 'READING_PARAGRAPHS', 3),
    ('reading-foundations', 'READING_VOCABULARY_IN_CONTEXT', 4),
    ('reading-foundations', 'READING_MAIN_IDEA_AND_DETAIL', 5),
    ('writing-foundations', 'WRITING', 1),
    ('writing-foundations', 'WRITING_SENTENCES', 2),
    ('writing-foundations', 'WRITING_PARAGRAPHS', 3),
    ('writing-foundations', 'WRITING_LINKING_WORDS', 4)
) AS m(course_slug, code, position)
JOIN content.taxonomies n ON n.code = m.code
WHERE n.namespace IN ('grammar', 'vocabulary', 'pattern', 'pronunciation', 'skill')
ON CONFLICT (course_slug, node_id) DO UPDATE SET position = EXCLUDED.position;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS content.foundation_course_nodes;

DELETE FROM content.taxonomy_prerequisites
WHERE node_id IN (
    SELECT id FROM content.taxonomies
    WHERE code IN (
        'PREFIXES', 'SUFFIXES', 'WORD_FAMILIES', 'CONVERSION', 'COMPOUND_WORDS',
        'IDIOMS', 'FIXED_EXPRESSIONS', 'SPOKEN_EXPRESSIONS',
        'LISTENING_WORDS', 'LISTENING_PHRASES', 'LISTENING_SENTENCES', 'LISTENING_CONVERSATIONS',
        'SPEAKING_WORDS', 'SPEAKING_PHRASES', 'SPEAKING_SENTENCES', 'SPEAKING_PARAGRAPHS',
        'READING_SENTENCES', 'READING_PARAGRAPHS', 'READING_VOCABULARY_IN_CONTEXT',
        'READING_MAIN_IDEA_AND_DETAIL',
        'WRITING_SENTENCES', 'WRITING_PARAGRAPHS', 'WRITING_LINKING_WORDS'
    )
);

DELETE FROM content.taxonomies
WHERE code IN (
    'PREFIXES', 'SUFFIXES', 'WORD_FAMILIES', 'CONVERSION', 'COMPOUND_WORDS',
    'IDIOMS', 'FIXED_EXPRESSIONS', 'SPOKEN_EXPRESSIONS',
    'LISTENING_WORDS', 'LISTENING_PHRASES', 'LISTENING_SENTENCES', 'LISTENING_CONVERSATIONS',
    'SPEAKING_WORDS', 'SPEAKING_PHRASES', 'SPEAKING_SENTENCES', 'SPEAKING_PARAGRAPHS',
    'READING_SENTENCES', 'READING_PARAGRAPHS', 'READING_VOCABULARY_IN_CONTEXT',
    'READING_MAIN_IDEA_AND_DETAIL',
    'WRITING_SENTENCES', 'WRITING_PARAGRAPHS', 'WRITING_LINKING_WORDS'
);

-- +goose StatementEnd

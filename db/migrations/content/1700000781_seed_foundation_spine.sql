-- +goose Up
-- +goose StatementBegin

-- 1. Grammar Roots (containment parents)
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position) VALUES
    ('grammar', 'SENTENCE_STRUCTURE', 'Sentence Structure', 'Basic English sentence patterns and elements.', 'A1', 1),
    ('grammar', 'PARTS_OF_SPEECH', 'Parts of Speech', 'The fundamental word classes in English grammar.', 'A1', 2),
    ('grammar', 'TENSES', 'English Tenses', 'Overview of the English verb tense system.', 'A1', 3),
    ('grammar', 'CLAUSES', 'Clauses', 'Dependent and independent clauses.', 'B1', 4)
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position;

-- 2. Grammar Topics
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position, parent_id) VALUES
    ('grammar', 'SUBJECT_VERB_OBJECT', 'Subject-Verb-Object', 'Basic SVO word order.', 'A1', 11,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'SENTENCE_STRUCTURE')),
    ('grammar', 'QUESTIONS_AND_NEGATIVES', 'Questions and Negatives', 'Forming yes/no questions, wh-questions, and negative statements.', 'A1', 12,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'SENTENCE_STRUCTURE')),
    ('grammar', 'NOUNS', 'Nouns', 'Countable, uncountable, and collective nouns.', 'A1', 21,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'PARTS_OF_SPEECH')),
    ('grammar', 'ARTICLES', 'Articles', 'Definite and indefinite articles: a, an, the.', 'A1', 22,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'PARTS_OF_SPEECH')),
    ('grammar', 'PRONOUNS', 'Pronouns', 'Personal, possessive, demonstrative, and relative pronouns.', 'A1', 23,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'PARTS_OF_SPEECH')),
    ('grammar', 'ADJECTIVES', 'Adjectives', 'Descriptive words and adjective order.', 'A1', 24,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'PARTS_OF_SPEECH')),
    ('grammar', 'ADVERBS', 'Adverbs', 'Manner, place, time, frequency, and degree adverbs.', 'A2', 25,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'PARTS_OF_SPEECH')),
    ('grammar', 'PREPOSITIONS', 'Prepositions', 'Prepositions of time, place, and direction.', 'A2', 26,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'PARTS_OF_SPEECH')),
    ('grammar', 'CONJUNCTIONS', 'Conjunctions', 'Coordinating, subordinating, and correlative conjunctions.', 'A2', 27,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'PARTS_OF_SPEECH')),
    ('grammar', 'MODAL_VERBS', 'Modal Verbs', 'Can, could, may, might, must, should, would.', 'A2', 28,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'PARTS_OF_SPEECH')),
    ('grammar', 'COMPARATIVES_AND_SUPERLATIVES', 'Comparatives and Superlatives', 'Comparing qualities using comparative and superlative forms.', 'A2', 31, NULL),
    ('grammar', 'PASSIVE_VOICE', 'Passive Voice', 'Focusing on the action or object rather than the agent.', 'B1', 32, NULL),
    ('grammar', 'CONDITIONALS', 'Conditionals', 'Zero, first, second, third, and mixed conditionals.', 'B1', 33, NULL),
    ('grammar', 'REPORTED_SPEECH', 'Reported Speech', 'Indirect speech, tense backshifting, and reporting verbs.', 'B1', 34, NULL),
    ('grammar', 'RELATIVE_CLAUSES', 'Relative Clauses', 'Defining and non-defining relative clauses.', 'B1', 41,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'CLAUSES')),
    ('grammar', 'NOUN_CLAUSES', 'Noun Clauses', 'Clauses acting as subjects, objects, or complements.', 'B2', 42,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'CLAUSES')),
    ('grammar', 'ADVERBIAL_CLAUSES', 'Adverbial Clauses', 'Clauses of time, cause, condition, and concession.', 'B2', 43,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'CLAUSES')),
    ('grammar', 'GERUNDS_AND_INFINITIVES', 'Gerunds and Infinitives', 'Verb forms functioning as nouns and complements.', 'B1', 51, NULL),
    ('grammar', 'PARTICIPLES', 'Participles', 'Present and past participles, participle clauses.', 'B2', 52, NULL),
    ('grammar', 'PHRASAL_VERBS', 'Phrasal Verbs', 'Multi-word verbs with idiomatic meanings.', 'B1', 53, NULL),
    ('grammar', 'COMMON_GRAMMAR_MISTAKES', 'Common Grammar Mistakes', 'Frequent errors in English grammar and how to avoid them.', 'A2', 54, NULL)
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position,
    parent_id = EXCLUDED.parent_id;

-- 3. Grammar Tenses (under TENSES)
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position, parent_id) VALUES
    ('grammar', 'PRESENT_SIMPLE', 'Present Simple', 'Habits, routines, and permanent facts.', 'A1', 101,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'PRESENT_CONTINUOUS', 'Present Continuous', 'Actions happening now and temporary situations.', 'A1', 102,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'PRESENT_PERFECT', 'Present Perfect', 'Life experiences, unfinished time, and recent events with present relevance.', 'B1', 107,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'PRESENT_PERFECT_CONTINUOUS', 'Present Perfect Continuous', 'Duration of activities continuing up to the present.', 'B1', 108,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'PAST_SIMPLE', 'Past Simple', 'Completed actions at specific times in the past.', 'A2', 103,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'PAST_CONTINUOUS', 'Past Continuous', 'Actions in progress at a specific past moment.', 'A2', 104,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'PAST_PERFECT', 'Past Perfect', 'An action completed before another past event.', 'B1', 105,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'PAST_PERFECT_CONTINUOUS', 'Past Perfect Continuous', 'Ongoing past activity before another past event.', 'B2', 106,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'FUTURE_SIMPLE', 'Future Simple', 'Predictions, promises, and spontaneous decisions with will.', 'A2', 109,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'FUTURE_CONTINUOUS', 'Future Continuous', 'Actions that will be in progress at a specific future time.', 'B1', 110,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'FUTURE_PERFECT', 'Future Perfect', 'Actions that will be finished before a future point in time.', 'B2', 111,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES')),
    ('grammar', 'FUTURE_PERFECT_CONTINUOUS', 'Future Perfect Continuous', 'Ongoing duration of actions leading up to a future point.', 'C1', 112,
        (SELECT id FROM content.taxonomies WHERE namespace = 'grammar' AND code = 'TENSES'))
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position,
    parent_id = EXCLUDED.parent_id;

-- 4. Vocabulary (8 topics)
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position) VALUES
    ('vocabulary', 'ESSENTIAL_EVERYDAY', 'Essential Everyday Vocabulary', 'Basic survival words for numbers, greetings, family, and home.', 'A1', 1),
    ('vocabulary', 'COMMON_VERBS', 'Common Verbs', 'Most frequent action and state verbs in everyday English.', 'A1', 2),
    ('vocabulary', 'HIGH_FREQUENCY_WORDS', 'High-Frequency Words', 'Top foundational English words for general communication.', 'A2', 3),
    ('vocabulary', 'TOPIC_VOCABULARY', 'Topic Vocabulary', 'Thematic vocabulary covering travel, food, work, and health.', 'A2', 4),
    ('vocabulary', 'COLLOCATIONS', 'Collocations', 'Natural word partnerships and fixed expressions.', 'B1', 5),
    ('vocabulary', 'SYNONYMS_AND_ANTONYMS', 'Synonyms and Antonyms', 'Expanding lexical range through word similarities and opposites.', 'B1', 6),
    ('vocabulary', 'ACADEMIC_VOCABULARY', 'Academic Vocabulary', 'Core academic words for study and formal writing.', 'B2', 7),
    ('vocabulary', 'WORKPLACE_VOCABULARY', 'Workplace Vocabulary', 'Professional language for business meetings, emails, and presentations.', 'B2', 8)
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position;

-- 5. Pattern (13 topics)
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position) VALUES
    ('pattern', 'INTRODUCING_YOURSELF', 'Introducing Yourself', 'Basic conversational patterns to introduce oneself and others.', 'A1', 1),
    ('pattern', 'ASKING_QUESTIONS', 'Asking Questions', 'Patterns for inquiring and seeking information politely.', 'A1', 2),
    ('pattern', 'ANSWERING_QUESTIONS', 'Answering Questions', 'Forming direct and indirect answers clearly.', 'A1', 3),
    ('pattern', 'DAILY_CONVERSATIONS', 'Daily Conversations', 'Small talk, routine exchanges, and situational dialogues.', 'A2', 4),
    ('pattern', 'REQUESTING', 'Requesting', 'Polite formulas to ask for assistance or items.', 'A2', 5),
    ('pattern', 'OFFERING', 'Offering', 'Expressions to offer help, hospitality, and suggestions.', 'A2', 6),
    ('pattern', 'SUGGESTING', 'Suggesting', 'How to propose ideas and activities.', 'A2', 7),
    ('pattern', 'AGREEING_AND_DISAGREEING', 'Agreeing and Disagreeing', 'Expressing agreement, diplomatic disagreement, and partial consensus.', 'B1', 8),
    ('pattern', 'GIVING_OPINIONS', 'Giving Opinions', 'Stating viewpoints, impressions, and beliefs.', 'B1', 9),
    ('pattern', 'DESCRIBING', 'Describing', 'Patterns for describing people, places, events, and feelings.', 'B1', 10),
    ('pattern', 'COMPARING', 'Comparing', 'Patterns for drawing parallels and contrasting alternatives.', 'B1', 11),
    ('pattern', 'EXPLAINING', 'Explaining', 'Clarifying causes, reasons, and step-by-step processes.', 'B1', 12),
    ('pattern', 'ASKING_FOR_CLARIFICATION', 'Asking for Clarification', 'Checking understanding and requesting repetition or explanation.', 'B1', 13)
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position;

-- 6. Pronunciation (8 topics)
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position) VALUES
    ('pronunciation', 'IPA_BASICS', 'IPA Basics', 'Introduction to the International Phonetic Alphabet symbols.', 'A1', 1),
    ('pronunciation', 'ENGLISH_SOUNDS', 'English Sounds', 'Vowels, consonants, and diphthongs of the English sound inventory.', 'A1', 2),
    ('pronunciation', 'WORD_STRESS', 'Word Stress', 'Primary and secondary syllable stress rules.', 'A2', 3),
    ('pronunciation', 'SENTENCE_STRESS', 'Sentence Stress', 'Content words vs. function words and rhythm in connected speech.', 'B1', 4),
    ('pronunciation', 'LINKING', 'Linking', 'Consonant-to-vowel and vowel-to-vowel connecting in speech.', 'B1', 5),
    ('pronunciation', 'REDUCTIONS', 'Reductions', 'Weak forms, contractions, and schwa usage.', 'B1', 6),
    ('pronunciation', 'INTONATION', 'Intonation', 'Pitch contours: rising, falling, and fall-rise patterns.', 'B2', 7),
    ('pronunciation', 'COMMON_PRONUNCIATION_MISTAKES', 'Common Pronunciation Mistakes', 'Addressing common pronunciation errors and silent letters.', 'A2', 8)
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position;

-- 7. Skill (4 topics)
INSERT INTO content.taxonomies (namespace, code, label, description, cefr_level, position) VALUES
    ('skill', 'LISTENING', 'Listening Comprehension', 'Strategies for gist, detail, and inference in spoken English.', 'A1', 1),
    ('skill', 'READING', 'Reading Comprehension', 'Skimming, scanning, and in-depth reading comprehension.', 'A1', 2),
    ('skill', 'SPEAKING', 'Speaking Skills', 'Fluency, pronunciation clarity, and interactive speech.', 'A1', 3),
    ('skill', 'WRITING', 'Writing Skills', 'Sentence construction, paragraph cohesion, and formal/informal writing.', 'A1', 4)
ON CONFLICT (namespace, code) DO UPDATE SET
    label = EXCLUDED.label,
    description = EXCLUDED.description,
    cefr_level = EXCLUDED.cefr_level,
    position = EXCLUDED.position;

-- 8. Prerequisite DAG Edges
INSERT INTO content.taxonomy_prerequisites (node_id, requires_node_id)
SELECT n.id, r.id
FROM content.taxonomies n
JOIN content.taxonomies r ON r.namespace = n.namespace
WHERE (n.namespace = 'grammar' AND (
    (n.code = 'PRESENT_SIMPLE' AND r.code = 'SENTENCE_STRUCTURE') OR
    (n.code = 'PRESENT_CONTINUOUS' AND r.code = 'PRESENT_SIMPLE') OR
    (n.code = 'PAST_SIMPLE' AND r.code = 'PRESENT_CONTINUOUS') OR
    (n.code = 'PRESENT_PERFECT' AND r.code = 'PAST_SIMPLE') OR
    (n.code = 'FUTURE_SIMPLE' AND r.code = 'PRESENT_PERFECT') OR
    (n.code = 'PAST_CONTINUOUS' AND r.code = 'PAST_SIMPLE') OR
    (n.code = 'PAST_PERFECT' AND r.code = 'PAST_SIMPLE') OR
    (n.code = 'PRESENT_PERFECT_CONTINUOUS' AND r.code = 'PRESENT_PERFECT') OR
    (n.code = 'RELATIVE_CLAUSES' AND r.code = 'SENTENCE_STRUCTURE') OR
    (n.code = 'NOUN_CLAUSES' AND r.code = 'RELATIVE_CLAUSES') OR
    (n.code = 'ADVERBIAL_CLAUSES' AND r.code = 'NOUN_CLAUSES') OR
    (n.code = 'NOUNS' AND r.code = 'PARTS_OF_SPEECH') OR
    (n.code = 'ARTICLES' AND r.code = 'NOUNS') OR
    (n.code = 'PRONOUNS' AND r.code = 'NOUNS') OR
    (n.code = 'ADJECTIVES' AND r.code = 'NOUNS') OR
    (n.code = 'ADVERBS' AND r.code = 'ADJECTIVES') OR
    (n.code = 'PREPOSITIONS' AND r.code = 'PARTS_OF_SPEECH') OR
    (n.code = 'CONJUNCTIONS' AND r.code = 'PARTS_OF_SPEECH') OR
    (n.code = 'MODAL_VERBS' AND r.code = 'PARTS_OF_SPEECH') OR
    (n.code = 'COMPARATIVES_AND_SUPERLATIVES' AND r.code = 'ADJECTIVES') OR
    (n.code = 'PARTICIPLES' AND r.code = 'PAST_SIMPLE') OR
    (n.code = 'PASSIVE_VOICE' AND r.code = 'PARTICIPLES') OR
    (n.code = 'REPORTED_SPEECH' AND r.code = 'PAST_SIMPLE') OR
    (n.code = 'CONDITIONALS' AND r.code IN ('PAST_SIMPLE', 'MODAL_VERBS')) OR
    (n.code = 'GERUNDS_AND_INFINITIVES' AND r.code = 'PARTS_OF_SPEECH')
))
OR (n.namespace = 'pronunciation' AND (
    (n.code = 'ENGLISH_SOUNDS' AND r.code = 'IPA_BASICS') OR
    (n.code = 'WORD_STRESS' AND r.code = 'ENGLISH_SOUNDS') OR
    (n.code = 'SENTENCE_STRESS' AND r.code = 'WORD_STRESS') OR
    (n.code = 'LINKING' AND r.code = 'WORD_STRESS') OR
    (n.code = 'REDUCTIONS' AND r.code = 'SENTENCE_STRESS') OR
    (n.code = 'INTONATION' AND r.code = 'SENTENCE_STRESS')
))
OR (n.namespace = 'vocabulary' AND (
    (n.code = 'COMMON_VERBS' AND r.code = 'ESSENTIAL_EVERYDAY') OR
    (n.code = 'HIGH_FREQUENCY_WORDS' AND r.code = 'ESSENTIAL_EVERYDAY') OR
    (n.code = 'COLLOCATIONS' AND r.code = 'HIGH_FREQUENCY_WORDS') OR
    (n.code = 'ACADEMIC_VOCABULARY' AND r.code = 'HIGH_FREQUENCY_WORDS') OR
    (n.code = 'WORKPLACE_VOCABULARY' AND r.code = 'HIGH_FREQUENCY_WORDS')
))
OR (n.namespace = 'pattern' AND (
    (n.code = 'ANSWERING_QUESTIONS' AND r.code = 'ASKING_QUESTIONS') OR
    (n.code = 'DAILY_CONVERSATIONS' AND r.code = 'INTRODUCING_YOURSELF')
))
ON CONFLICT (node_id, requires_node_id) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM content.taxonomy_prerequisites
WHERE node_id IN (
    SELECT id FROM content.taxonomies
    WHERE namespace IN ('grammar', 'vocabulary', 'pattern', 'pronunciation', 'skill')
);

DELETE FROM content.taxonomies
WHERE namespace IN ('grammar', 'vocabulary', 'pattern', 'pronunciation', 'skill');

-- +goose StatementEnd

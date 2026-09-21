-- +goose Up
-- +goose StatementBegin

INSERT INTO content.taxonomies (namespace, code, label) VALUES
    ('course_topic', 'business-english', 'Business English'),
    ('course_topic', 'academic-ielts', 'Academic & IELTS Prep'),
    ('course_topic', 'toeic-workplace', 'TOEIC & Workplace Communication'),
    ('course_topic', 'daily-conversation', 'Daily Conversation & Small Talk'),
    ('course_topic', 'travel-hospitality', 'Travel & Hospitality'),
    ('course_topic', 'tech-software', 'Technology & IT'),
    ('course_topic', 'finance-banking', 'Finance & Banking'),
    ('course_topic', 'medical-healthcare', 'Medical & Healthcare'),
    ('course_topic', 'legal-english', 'Legal English'),
    ('course_topic', 'job-interviews', 'Job Interviews & Career'),
    ('course_topic', 'news-current-affairs', 'News & Current Affairs'),
    ('course_topic', 'arts-culture', 'Arts, Culture & Entertainment'),
    ('course_topic', 'science-environment', 'Science & Environment'),
    ('course_topic', 'digital-life', 'Digital Life & Social Media'),
    ('course_topic', 'food-dining', 'Food & Dining'),
    ('course_topic', 'sports-fitness', 'Sports & Fitness'),
    ('course_topic', 'education-teaching', 'Education & Teaching'),
    ('course_topic', 'cross-cultural', 'Cross-Cultural Communication'),
    ('course_topic', 'academic-writing', 'Academic Writing & Research'),
    ('course_topic', 'public-speaking', 'Public Speaking & Presentations')
ON CONFLICT (namespace, code) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DELETE FROM content.taxonomies WHERE namespace = 'course_topic';

-- +goose StatementEnd

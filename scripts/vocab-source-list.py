#!/usr/bin/env python3
"""Build the lemmatised frequency list cmd/vocabgen reads, and clean the fixture.

WO 22 Stage C step 1 asks for the list to be lemmatised, with proper nouns,
abbreviations, profanity and non-words dropped, before any model call. The Go
build tool reads a finished list (`rank<TAB>lemma`); this script produces it
from wordfreq's English list, offline, and applies the same rules to a fixture
that was written before they existed.

    python -m venv .venv && . .venv/bin/activate
    pip install wordfreq==3.1.1 lemminflect==0.2.3 eng-to-ipa==0.0.2
    python scripts/vocab-source-list.py build   # writes the source list
    python scripts/vocab-source-list.py clean   # cleans db/fixtures/vocabulary
    go run ./cmd/vocabgen -canonicalise         # rewrites the files in the Go format

`clean` keeps every word the rules accept, gives it its frequency rank and, when
it has none, an IPA transcription from the CMU Pronouncing Dictionary (through
eng-to-ipa). It drops inflected forms ("went", "years": the headword is the
lemma, D22-5), proper nouns, foreign words, abbreviations and non-standard
spellings, and records each dropped word with its reason in skipped.json, so
cmd/vocabgen never asks the model about it again. The lemmas the fixture then
lacks are the ones `cmd/vocabgen -source ...` generates.

Licences, recorded in every fixture header: wordfreq's code is Apache-2.0 and
its word-list data CC BY-SA 4.0; the CMU Pronouncing Dictionary is BSD-style.
"""

import datetime
import json
import os
import re
import sys

import eng_to_ipa
import wordfreq
from lemminflect import getAllLemmas, getAllLemmasOOV

FIXTURE_DIR = "db/fixtures/vocabulary"
SOURCE_FILE = os.path.join(FIXTURE_DIR, "source", "wordfreq-lemmas.tsv")
SKIPPED_FILE = os.path.join(FIXTURE_DIR, "skipped.json")
LEVELS = ["a1", "a2", "b1", "b2", "c1"]

# How far down wordfreq's list the candidates go. 10,000 headwords survive the
# rules well before this; the rest is headroom for what the model skips.
CANDIDATES = 40000
# How many lemmas the source list holds, in rank order.
SOURCE_LEMMAS = 16000

SOURCE = "wordfreq 3.1.1 (top_n_list, English), lemmatised with lemminflect 0.2.3"
LICENCE = (
    "Word list: wordfreq data, CC BY-SA 4.0 "
    "(https://creativecommons.org/licenses/by-sa/4.0/), by Robyn Speer; "
    "wordfreq code Apache-2.0. IPA: CMU Pronouncing Dictionary (BSD-style) via "
    "eng-to-ipa, replaced by the Free Dictionary API's (Wiktionary) where step 3 finds one; "
    "each recording is credited on its word. Definitions and examples are original model-written text."
)

PROFANE_STEMS = ("fuck", "shit", "bitch", "cunt", "whore", "slut", "porn", "wank")
PROFANE = {
    "arse", "ass", "asshole", "bastard", "bollocks", "bugger", "cock", "crap",
    "damn", "dick", "goddamn", "hell", "piss", "prick", "pussy", "twat",
    "boobs", "tits", "horny", "cum", "wtf", "lmao", "omg",
}

# Spellings a course does not teach: contractions without their apostrophe,
# chat forms and letters.
NON_STANDARD = {
    "dont", "doesnt", "didnt", "isnt", "wasnt", "arent", "werent", "cant",
    "wont", "wouldnt", "couldnt", "shouldnt", "havent", "hasnt", "hadnt",
    "thats", "whats", "theres", "heres", "lets", "im", "ive", "youre", "youve",
    "theyre", "theyve", "weve", "hes", "shes", "ill", "id", "gonna", "wanna",
    "gotta", "kinda", "sorta", "outta", "lol", "haha", "hmm", "huh", "uh",
    "um", "eh", "ugh", "nah", "yo", "ya", "tho", "idk", "tbh", "btw", "aw",
    "goin", "hahaha", "dem", "lo",
}

# In the model's own definition, these mark a headword that is not an English
# word a learner should drill: a name, a place, a foreign word, an abbreviation.
DROP_DEFINITION = [
    (re.compile(r"\b(first|family|given|last|boy'?s|girl'?s|man'?s|woman'?s|common|male|female|person'?s|surname)\b[^.;]*\bname\b"), "proper_noun"),
    (re.compile(r"\bname (of|for) (a|an|the) (man|woman|boy|girl|person|city|town|place|company|country)\b"), "proper_noun"),
    (re.compile(r"\b(capital|largest) city\b|\ba (big |large |major )?city in\b|\ba (country|state|province|region|continent|island) (in|of|on)\b"), "proper_noun"),
    (re.compile(r"\bpresident of\b|\bthe bible\b|\breligious teacher\b"), "proper_noun"),
    (re.compile(r"\b(website|social media site|video site|company that|brand of|search engine)\b"), "proper_noun"),
    (re.compile(r"\b(vietnamese|french|spanish|italian|german|latin|japanese|korean|chinese|arabic|portuguese|dutch|hindi)\b[^.;]*\bword\b|\bword from (french|spanish|latin|italian|german)\b"), "foreign_word"),
    (re.compile(r"\b(short for|short form|shortened form|abbreviation|abbreviated|roman (number|numeral)|initials)\b"), "abbreviation"),
    (re.compile(r"\b(informal|chat|text) (spelling|way of writing)\b|\bsound of laughter\b"), "non_standard"),
    (re.compile(r"\bending added to\b|\bsuffix\b|\bprefix\b"), "not_a_word"),
]

# Named things, in the model's definition of a word the lexicon does not know:
# places, brands, organisations, people. A nationality or language adjective
# ("relating to France, its people or its language") is kept; the country is not.
#
# A named thing's definition starts by saying what it is ("a large country in",
# "the capital city of", "a famous American singer"); anchoring on the start
# keeps "county: a large area of a country" and "with: in the company of".
NAMED_THING_START = re.compile(
    r"^(the |a |an )?((large|small|big|famous|popular|major|cold|warm|rich|very|old|long|"
    r"american|english|british|us|international|online|mythical|swiss|german|french|greek|"
    r"roman|wise|religious|ancient|island|european|asian|african|technology|professional|"
    r"fictional|free|computer|second|third|largest|biggest|former|independent|green|port|"
    r"tablet|sports|news)[ ,-]+)*"
    r"(country|continent|state|province|island|islands|city|town|capital|district|river|"
    r"region|company|game machine|video game|app|service|store|shop chain|sports team|"
    r"football team|basketball team|baseball team|league|agency|"
    r"organi[sz]ation|university|pop group|group of|football player|singer|writer|rapper|leader|"
    r"emperor|king|president|psychiatrist|television channel|program for|operating system|"
    r"brand|hero|sailor|film industry|holy book|part of)\b"
)
# Named things the patterns miss, found reading what they kept (Stage C's
# person-read sample): brands, people, places and the obscure common-noun
# senses the model gave to surnames ("graham: a type of flour").
CURATED_PROPER = {
    "santa", "pokemon", "lego", "kent", "ipad", "ferrari", "messi", "starbucks", "sussex",
    "kong", "brooklyn", "nasa", "holmes", "yorkshire", "hampshire", "westminster", "biden",
    "devon", "norfolk", "southampton", "ussr", "vatican", "somerset", "reuters", "forbes",
    "obamacare", "isis", "africa", "asia", "arabia", "broadway", "maya", "venus", "caesar",
    "apollo", "carter", "graham", "nelson", "cooper", "jerry", "warner", "spencer", "bailey",
    "palmer", "gilbert", "freeman", "dana", "silva", "timothy", "charleston", "webster",
    "finn", "rand", "kirk", "belle", "yang", "jung", "rico", "grande", "dong", "gaza",
    "essex", "wheeler", "stan",
}

# Wherever it appears, this says the headword is a name.
NAMED_ANYWHERE = re.compile(
    r"\b(surname|first name|family name|(boy|girl|man|woman)'?s name|the name of|proper name|"
    r"part of the (\w+ )?name|(place|city|country) names?|an? (common |english |korean |popular )*name)\b"
)
NATIONALITY = re.compile(r"^(relating to|coming from|belonging to|from or relating to)\b")

# Closed-class parts of speech: the lexicon holds only open-class words, so
# these are kept on the model's word when they are not a named thing.
CLOSED_CLASS = {"determiner", "pronoun", "preposition", "conjunction", "number", "interjection"}
# Short words the lexicon does not know that are still worth drilling.
SHORT_KEEP = {
    "ok", "oh", "hi", "bye", "hey", "wow", "yes", "no", "per", "app", "lab", "pub", "cop",
    "one", "two", "six", "ten",
}

# The model's definition of an inflected form names it as one.
INFLECTION_DEFINITION = re.compile(
    r"\b(past (form|tense|participle)|plural (form )?of|-ing form|present participle|"
    r"third[- ]person|form of '?\w+'? used with)\b"
)

UPOS_FOR_POS = {"noun": "NOUN", "verb": "VERB", "adjective": "ADJ", "adverb": "ADV"}


def is_candidate(word):
    if not re.fullmatch(r"[a-z]{2,}", word):
        return False
    if not re.search(r"[aeiouy]", word):
        return False
    if word in PROFANE or word in NON_STANDARD:
        return False
    return not any(stem in word for stem in PROFANE_STEMS)


def lemma_of(word, pos=None):
    """The headword a form folds into, or the word itself when it is one."""
    lemmas = getAllLemmas(word)
    if not lemmas:
        return word
    own = any(word in forms for forms in lemmas.values())
    if own:
        return word
    wanted = UPOS_FOR_POS.get(pos or "")
    if wanted and wanted in lemmas:
        return lemmas[wanted][0]
    # With no part of speech to go on, the commonest base: "does" is "do",
    # not "doe".
    bases = {form for forms in lemmas.values() for form in forms}
    return max(sorted(bases), key=lambda base: wordfreq.word_frequency(base, "en"))


def source_ranks():
    """Every candidate lemma with the rank of its commonest form, in order."""
    ranks = {}
    for index, word in enumerate(wordfreq.top_n_list("en", CANDIDATES)):
        if not is_candidate(word):
            continue
        lemma = lemma_of(word)
        if is_candidate(lemma) and lemma not in ranks:
            ranks[lemma] = index + 1
    return ranks


def build():
    ranks = source_ranks()
    rows = sorted(ranks.items(), key=lambda item: item[1])[:SOURCE_LEMMAS]
    os.makedirs(os.path.dirname(SOURCE_FILE), exist_ok=True)
    with open(SOURCE_FILE, "w", encoding="utf-8") as out:
        out.write(f"# {SOURCE}\n")
        out.write(f"# {LICENCE}\n")
        out.write(f"# built {datetime.date.today().isoformat()} by scripts/vocab-source-list.py\n")
        out.write("# rank<TAB>lemma: the rank is wordfreq's rank of the lemma's commonest form\n")
        for lemma, rank in rows:
            out.write(f"{rank}\t{lemma}\n")
    print(f"wrote {len(rows)} lemmas to {SOURCE_FILE}")


def drop_reason(word, ranks):
    lemma = word["lemma"]
    definition = word["definition"].lower()
    if not is_candidate(lemma):
        return "non_standard"
    if word.get("pos") in ("abbreviation", "prefix", "suffix"):
        return "abbreviation"
    base = lemma_of(lemma, word.get("pos"))
    if base != lemma:
        return f"inflection_of:{base}"
    if INFLECTION_DEFINITION.search(definition):
        # "left" is a lemma, but a definition of "past form of leave" teaches
        # the inflection; the word is regenerated with its own meaning.
        return "inflection_meaning"
    if not getAllLemmas(lemma):
        for pattern, reason in DROP_DEFINITION:
            if pattern.search(definition):
                return reason
        # A plural or past form the lexicon does not know ("websites",
        # "americans") folds into its lemma like any other inflection.
        bases = {form for upos in ("NOUN", "VERB") for form in getAllLemmasOOV(lemma, upos).get(upos, ())}
        if lemma.endswith("ies"):
            bases.add(lemma[:-1])  # "calories" -> "calorie", which the rules miss
        for form in sorted(bases):
            if form != lemma and form in ranks:
                return f"inflection_of:{form}"
        pos = word.get("pos", "")
        if pos in CLOSED_CLASS:
            return None if lemma in ranks else "not_in_source"
        if len(lemma) <= 3 and lemma not in SHORT_KEEP:
            return "non_standard"
        if lemma in CURATED_PROPER:
            return "proper_noun"
        if NATIONALITY.search(definition):
            return None if lemma in ranks else "not_in_source"
        if NAMED_THING_START.search(definition) or NAMED_ANYWHERE.search(definition):
            return "proper_noun"
    if lemma not in ranks:
        return "not_in_source"
    return None


def ipa_of(lemma):
    ipa = eng_to_ipa.convert(lemma)
    if not ipa or "*" in ipa or " " in ipa:
        return ""
    return f"/{ipa}/"


def clean():
    ranks = source_ranks()
    kept_by_level = {}
    skipped = {}
    if os.path.exists(SKIPPED_FILE):
        for row in json.load(open(SKIPPED_FILE, encoding="utf-8")):
            skipped[row["lemma"]] = row["reason"]
    for level in LEVELS:
        path = os.path.join(FIXTURE_DIR, f"words-{level}.json")
        data = json.load(open(path, encoding="utf-8"))
        kept = []
        for word in data["words"]:
            reason = drop_reason(word, ranks)
            if reason:
                # An inflection whose meaning was the inflection's is not
                # skipped for good: its lemma is still wanted.
                if reason != "inflection_meaning" and reason != "not_in_source":
                    skipped[word["lemma"]] = reason
                continue
            word["rank"] = ranks[word["lemma"]]
            if not word.get("ipa"):
                ipa = ipa_of(word["lemma"])
                if ipa:
                    word["ipa"] = ipa
            kept.append(word)
        kept.sort(key=lambda w: w["rank"])
        data["words"] = kept
        data["source"] = SOURCE
        data["licence"] = LICENCE
        data["checked_at"] = datetime.date.today().isoformat()
        kept_by_level[level] = data
    for level, data in kept_by_level.items():
        path = os.path.join(FIXTURE_DIR, f"words-{level}.json")
        with open(path, "w", encoding="utf-8") as out:
            json.dump(data, out, ensure_ascii=False, indent=2)
            out.write("\n")
    rows = [{"lemma": lemma, "reason": reason} for lemma, reason in sorted(skipped.items())]
    with open(SKIPPED_FILE, "w", encoding="utf-8") as out:
        json.dump(rows, out, ensure_ascii=False, indent=2)
        out.write("\n")
    total = sum(len(d["words"]) for d in kept_by_level.values())
    with_ipa = sum(1 for d in kept_by_level.values() for w in d["words"] if w.get("ipa"))
    print(f"kept {total} words ({with_ipa} with IPA); {len(rows)} recorded in {SKIPPED_FILE}")


if __name__ == "__main__":
    if len(sys.argv) != 2 or sys.argv[1] not in ("build", "clean"):
        sys.exit("usage: vocab-source-list.py build|clean")
    build() if sys.argv[1] == "build" else clean()

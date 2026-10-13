/**
 * The word list a learner pastes or imports, read with the same rules as the
 * server's parser (internal/modules/vocabulary/domain/upload.go).
 */

/** Words one upload takes: domain.MaxUploadEntries on the server. */
export const MAX_UPLOAD_WORDS = 300;

/** Largest file the import reads; a 300-word list is a few kilobytes. */
export const MAX_IMPORT_BYTES = 1024 * 1024;

const SEPARATORS = [
  "\t",
  " - ",
  " – ",
  " — ",
  " = ",
  ": ",
  " : ",
  "=",
  ";",
  "|",
];

/** The term a line names, or "" for a line the server would skip. */
function termOf(rawLine: string): string {
  let line = rawLine.trim().replace(/^[-*•\t ]+/, "");
  line = line.replace(/^\d+[.):]\s*/, "");
  if (line === "") return "";

  let term = line;
  for (const separator of SEPARATORS) {
    const index = line.indexOf(separator);
    if (index > 0) {
      term = line.slice(0, index);
      break;
    }
  }
  term = term.trim().replace(/[.,;:]+$/, "");
  return term !== "" && /\p{L}/u.test(term) ? term.toLowerCase() : "";
}

/** Counts the distinct words the server's parser will find. */
export function countWords(text: string): number {
  const seen = new Set<string>();
  for (const line of text.split("\n")) {
    const term = termOf(line);
    if (term) seen.add(term);
  }
  return seen.size;
}

/**
 * Splits the text after the line that brings it to `limit` distinct words.
 *
 * The server keeps the first MAX_UPLOAD_WORDS entries of a paste and drops the
 * rest without a word, so an imported 2,000-card deck would lose 1,700 cards
 * nobody was told about. The form sends `head` and leaves `rest` in the box.
 */
export function splitAtLimit(
  text: string,
  limit: number = MAX_UPLOAD_WORDS,
): { head: string; rest: string } {
  const lines = text.split("\n");
  const seen = new Set<string>();
  for (let i = 0; i < lines.length; i++) {
    const term = termOf(lines[i] ?? "");
    if (!term || seen.has(term)) continue;
    if (seen.size === limit) {
      return {
        head: lines.slice(0, i).join("\n"),
        rest: lines.slice(i).join("\n"),
      };
    }
    seen.add(term);
  }
  return { head: text, rest: "" };
}

/** What importWordFile made of a file. */
export interface ImportedList {
  /** One "term<TAB>meaning" line per row, ready for the box. */
  text: string;
  /** Rows that became a line. */
  rows: number;
}

/**
 * Turns a CSV, TSV or Anki "Notes in Plain Text" export into the box's format.
 *
 * - Anki writes `#separator:tab`, `#html:true` and similar header lines, and
 *   HTML inside its fields: the headers pick the separator and are dropped, and
 *   the markup is stripped to text.
 * - Otherwise the separator is whichever of tab, semicolon and comma the first
 *   row uses, in that order; quoted fields ("a, b") follow RFC 4180.
 * - The first column is the word and the second, when present, its meaning.
 *   A header row such as "word,meaning" is the learner's to delete: it is a
 *   word to the parser, and guessing would drop a real one.
 */
export function importWordFile(content: string): ImportedList {
  const normalised = content.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n");
  const lines = normalised.split("\n");

  let separator: string | undefined;
  const body: string[] = [];
  for (const line of lines) {
    const header = /^#separator:(.+)$/i.exec(line.trim());
    if (header) {
      separator = ankiSeparator(header[1] ?? "");
      continue;
    }
    if (line.startsWith("#")) continue;
    body.push(line);
  }

  const firstRow = body.find((line) => line.trim() !== "") ?? "";
  separator ??= ["\t", ";", ","].find((s) => firstRow.includes(s)) ?? "\t";

  const out: string[] = [];
  for (const record of parseDelimited(body.join("\n"), separator)) {
    const term = cleanField(record[0] ?? "");
    if (term === "") continue;
    const meaning = cleanField(record[1] ?? "");
    out.push(meaning ? `${term}\t${meaning}` : term);
  }
  return { text: out.join("\n"), rows: out.length };
}

function ankiSeparator(name: string): string {
  switch (name.trim().toLowerCase()) {
    case "comma":
    case ",":
      return ",";
    case "semicolon":
    case ";":
      return ";";
    case "pipe":
    case "|":
      return "|";
    case "space":
    case " ":
      return " ";
    default:
      return "\t";
  }
}

/** RFC 4180 records: quoted fields may hold the separator, quotes and newlines. */
function parseDelimited(text: string, separator: string): string[][] {
  const records: string[][] = [];
  let record: string[] = [];
  let field = "";
  let quoted = false;

  for (let i = 0; i < text.length; i++) {
    const ch = text[i];
    if (quoted) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i++;
        } else {
          quoted = false;
        }
      } else {
        field += ch;
      }
      continue;
    }
    if (ch === '"' && field === "") {
      quoted = true;
    } else if (ch === separator) {
      record.push(field);
      field = "";
    } else if (ch === "\n") {
      record.push(field);
      records.push(record);
      record = [];
      field = "";
    } else {
      field += ch;
    }
  }
  record.push(field);
  records.push(record);
  return records;
}

/**
 * One field as plain text on one line: Anki's markup and sound tags removed,
 * entities decoded, and whitespace (including a quoted newline) collapsed, so
 * a field can never start a second line in the box.
 *
 * Plain string work, no HTML parser: the file is the learner's own and the
 * result only ever goes into a textarea's value, but handing file text to
 * DOMParser is what CodeQL reads as "DOM text reinterpreted as HTML", and
 * nothing here needs a document.
 */
function cleanField(raw: string): string {
  const withoutSound = raw
    .replace(/\[sound:[^\]]*\]/gi, " ")
    .replace(/<br\s*\/?>/gi, " ");
  return decodeEntities(stripTags(withoutSound)).replace(/\s+/g, " ").trim();
}

/**
 * Drops every tag and keeps the text around it, as a document's textContent
 * would (`<b>r</b>ảnh` is one word; a line break was made a space before).
 * As in HTML, `<` opens a tag only before a letter, `/` or `!`; a bare
 * `a < b` is text.
 */
function stripTags(text: string): string {
  let out = "";
  let inTag = false;
  for (let i = 0; i < text.length; i++) {
    const ch = text.charAt(i);
    if (inTag) {
      if (ch === ">") inTag = false;
    } else if (ch === "<" && /[a-z/!]/i.test(text.charAt(i + 1))) {
      inTag = true;
    } else {
      out += ch;
    }
  }
  return out;
}

const NAMED_ENTITIES: Record<string, string> = {
  amp: "&",
  lt: "<",
  gt: ">",
  quot: '"',
  apos: "'",
  nbsp: " ",
};

/** The entities Anki writes: the named few and every numeric reference. */
function decodeEntities(text: string): string {
  return text.replace(
    /&(#x[0-9a-f]+|#\d+|[a-z]+);/gi,
    (entity, body: string) => {
      if (body.startsWith("#")) {
        const code =
          body[1] === "x" || body[1] === "X"
            ? parseInt(body.slice(2), 16)
            : parseInt(body.slice(1), 10);
        return code > 0 && code <= 0x10ffff
          ? String.fromCodePoint(code)
          : entity;
      }
      return NAMED_ENTITIES[body.toLowerCase()] ?? entity;
    },
  );
}

export interface DiffToken {
  type: "match" | "omission" | "addition" | "substitution";
  text: string;
  expected?: string | undefined;
  received?: string | undefined;
}

/** Lowercases a token for comparison. Tokenizing has already stripped the rest. */
export function normalizeWord(word: string): string {
  return word.toLowerCase();
}

/**
 * Splits text the way the server does.
 *
 * This mirrors `domain.TokenizeWords` in
 * `internal/modules/speaking/domain/speaking.go` deliberately, character class
 * for character class: letters and digits accumulate, whitespace and
 * punctuation end a token, and anything else is dropped without ending one.
 *
 * It matters because the percentage beside this diff was computed by that Go
 * function. Splitting on whitespace instead counted "don't" as one token where
 * the server counted two, so a learner could read a diff with one error marked
 * against a score that was docked for two. The explanation has to be built from
 * the same tokens as the number it explains.
 */
export function tokenizeWords(text: string): string[] {
  const tokens: string[] = [];
  let current = "";
  for (const char of text) {
    if (/[\p{L}\p{N}]/u.test(char)) {
      current += char;
    } else if (/[\s\p{Z}\p{P}]/u.test(char)) {
      if (current) {
        tokens.push(current);
        current = "";
      }
    }
    // Anything else — a symbol such as + or $ — is skipped without ending the
    // token, which is what the Go loop does by falling through both branches.
  }
  if (current) tokens.push(current);
  return tokens;
}

/**
 * Computes word-level alignment diff between reference text and transcribed text
 * using Levenshtein distance backtracing.
 */
export function computeWordDiff(
  referenceText: string,
  transcriptText: string,
): DiffToken[] {
  const refWords = tokenizeWords(referenceText);
  const transWords = tokenizeWords(transcriptText);

  if (refWords.length === 0) {
    return transWords.map((t) => ({ type: "addition", text: t }));
  }
  if (transWords.length === 0) {
    return refWords.map((r) => ({ type: "omission", text: r }));
  }

  const m = refWords.length;
  const n = transWords.length;

  const dp: number[][] = [];
  for (let i = 0; i <= m; i++) {
    const row: number[] = new Array<number>(n + 1).fill(0);
    row[0] = i;
    dp.push(row);
  }
  const firstRow = dp[0];
  if (firstRow) {
    for (let j = 0; j <= n; j++) {
      firstRow[j] = j;
    }
  }

  for (let i = 1; i <= m; i++) {
    const currentRow = dp[i];
    const prevRow = dp[i - 1];
    if (!currentRow || !prevRow) continue;
    const rNorm = normalizeWord(refWords[i - 1] ?? "");
    for (let j = 1; j <= n; j++) {
      const tNorm = normalizeWord(transWords[j - 1] ?? "");
      const cost = rNorm === tNorm ? 0 : 1;
      currentRow[j] = Math.min(
        (prevRow[j] ?? 0) + 1, // omission / deletion
        (currentRow[j - 1] ?? 0) + 1, // addition / insertion
        (prevRow[j - 1] ?? 0) + cost, // match or substitution
      );
    }
  }

  const tokens: DiffToken[] = [];
  let i = m;
  let j = n;

  while (i > 0 || j > 0) {
    if (i > 0 && j > 0) {
      const rNorm = normalizeWord(refWords[i - 1] ?? "");
      const tNorm = normalizeWord(transWords[j - 1] ?? "");
      const cost = rNorm === tNorm ? 0 : 1;

      if (dp[i]?.[j] === (dp[i - 1]?.[j - 1] ?? 0) + cost) {
        if (cost === 0) {
          tokens.unshift({
            type: "match",
            text: refWords[i - 1] ?? "",
          });
        } else {
          tokens.unshift({
            type: "substitution",
            text: transWords[j - 1] ?? "",
            expected: refWords[i - 1] ?? "",
            received: transWords[j - 1] ?? "",
          });
        }
        i--;
        j--;
        continue;
      }
    }

    if (i > 0 && dp[i]?.[j] === (dp[i - 1]?.[j] ?? 0) + 1) {
      tokens.unshift({
        type: "omission",
        text: refWords[i - 1] ?? "",
      });
      i--;
    } else if (j > 0 && dp[i]?.[j] === (dp[i]?.[j - 1] ?? 0) + 1) {
      tokens.unshift({
        type: "addition",
        text: transWords[j - 1] ?? "",
      });
      j--;
    } else {
      if (i > 0) i--;
      if (j > 0) j--;
    }
  }

  return tokens;
}

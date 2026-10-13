import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import { ERROR_MESSAGES, getErrorMessage } from "@/lib/errors/catalogue";

/**
 * The catalogue is a copy of a contract that lives in Go, and a copy drifts.
 *
 * It drifted once already: it carried CODE_INVALID, CHALLENGE_BURNED and
 * CHALLENGE_EXPIRED, none of which the server has ever emitted, so every OTP
 * refusal fell through to the bare RFC 9457 title and the burned state was
 * unreachable in the UI. Nothing failed — not the compiler, not a test — because
 * a lookup miss is a fallback, not an error.
 *
 * So this reads the Go source and fails on the mismatch itself.
 */

// The modules, and the shared HTTP layer: the rate limiter answers RATE_LIMITED
// for every endpoint from internal/shared/httpx, not from a module.
const SOURCE_ROOTS = [
  join(__dirname, "../../../internal/modules"),
  join(__dirname, "../../../internal/shared"),
];

let cachedServerErrorCodes: Set<string> | undefined;

/**
 * Codes the API can put in `problem.code`, read from the domain error files.
 *
 * Memoised: the Go tree does not change while the suite runs, and this used to
 * walk it seven times — once in one test and once per code in the other's loop.
 * Standalone that cost under a second; sharing a machine with the rest of the
 * suite it took six, and the 5s default timeout failed two tests that had found
 * nothing wrong.
 */
function serverErrorCodes(): Set<string> {
  if (cachedServerErrorCodes !== undefined) return cachedServerErrorCodes;

  const codes = new Set<string>();

  const walk = (directory: string): void => {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) {
        walk(path);
        continue;
      }
      // Not just errors.go: INVALID_CREDENTIALS is constructed in
      // service/login.go, the OAuth codes in domain/oauth.go, and
      // LAST_ADMIN_PROTECTED in rbac/domain/domain.go. Scoping the scan to one
      // filename would have made this test pass by looking in the wrong place.
      if (!entry.name.endsWith(".go") || entry.name.endsWith("_test.go")) {
        continue;
      }

      const source = readFileSync(path, "utf8");
      // apperr.New(apperr.Kind, "CODE", "message") — the second argument is the
      // wire code. Matching on the constructor rather than on every uppercase
      // string keeps SQL keywords and env var names out of the set.
      for (const match of source.matchAll(
        /apperr\.\w+\s*,\s*"([A-Z][A-Z0-9_]{3,})"/g,
      )) {
        codes.add(match[1]!);
      }
      for (const match of source.matchAll(/Code:\s*"([A-Z][A-Z0-9_]{3,})"/g)) {
        codes.add(match[1]!);
      }
    }
  };

  for (const root of SOURCE_ROOTS) walk(root);
  cachedServerErrorCodes = codes;
  return codes;
}

describe("the error catalogue against the server's own codes", () => {
  it("names no code the server cannot send", () => {
    const server = serverErrorCodes();
    expect(server.size).toBeGreaterThan(20);

    const invented = Object.keys(ERROR_MESSAGES).filter(
      (code) => !server.has(code),
    );

    expect(
      invented,
      `these codes are in the catalogue but no Go errors.go emits them, so they can never match: ${invented.join(", ")}`,
    ).toEqual([]);
  });

  it("covers every OTP refusal a learner can actually hit", () => {
    // The specific family that broke. A learner meets all of these in the
    // registration flow, and each must read as a sentence rather than as
    // "Rate limited".
    for (const code of [
      "OTP_INVALID",
      "OTP_EXPIRED",
      "OTP_ATTEMPTS_EXCEEDED",
      "OTP_ALREADY_USED",
      "OTP_RESEND_TOO_SOON",
      "OTP_ISSUE_LIMIT_REACHED",
    ]) {
      expect(serverErrorCodes(), `${code} is not a server code`).toContain(
        code,
      );
      expect(
        ERROR_MESSAGES[code],
        `${code} has no catalogue entry`,
      ).toBeTruthy();
    }
  });

  it("says how long to wait when the rate limiter refuses", () => {
    // What a learner who signs in too often used to read: "Rate limited".
    const refused = {
      title: "Rate limited",
      status: 429,
      code: "RATE_LIMITED",
      meta: { retry_after_seconds: 241 },
    };
    expect(getErrorMessage(refused)).toBe(
      "You have tried too many times. Please wait 5 minute(s) and try again.",
    );

    // The translator gets the timed key and the minutes to pluralise.
    const calls: unknown[] = [];
    const t = (key: string, options: { count?: number | undefined }) => {
      calls.push([key, options.count]);
      return `${key}:${String(options.count)}`;
    };
    expect(getErrorMessage(refused, t)).toBe("errors.RATE_LIMITED_timed:5");
    expect(calls).toEqual([["errors.RATE_LIMITED_timed", 5]]);
  });

  it("treats a bare 429 as the rate limiter, not as its title", () => {
    expect(getErrorMessage({ title: "Too Many Requests", status: 429 })).toBe(
      ERROR_MESSAGES["RATE_LIMITED"],
    );
  });

  it("falls back to the title for a code it does not know", () => {
    expect(
      getErrorMessage({
        title: "Rate limited",
        status: 429,
        code: "NOT_A_CODE",
      }),
    ).toBe("Rate limited");
  });
});

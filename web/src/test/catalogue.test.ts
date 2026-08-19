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

const MODULE_ROOT = join(__dirname, "../../../internal/modules");

/** Codes the API can put in `problem.code`, read from the domain error files. */
function serverErrorCodes(): Set<string> {
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

  walk(MODULE_ROOT);
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
      expect(serverErrorCodes(), `${code} is not a server code`).toContain(code);
      expect(ERROR_MESSAGES[code], `${code} has no catalogue entry`).toBeTruthy();
    }
  });

  it("falls back to the title for a code it does not know", () => {
    expect(
      getErrorMessage({ title: "Rate limited", status: 429, code: "NOT_A_CODE" }),
    ).toBe("Rate limited");
  });
});

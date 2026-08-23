import { expect, test } from "@playwright/test";

import { extractOtpCode } from "./mailpit";

// Pure logic, no page: extractOtpCode is what every OTP journey depends on, and
// it read the wrong number out of a real email once. These are the four bodies
// the Go templates actually render, plus the one that broke.

const CODE = "482915";

test.describe("extractOtpCode", () => {
  test("reads the code out of every template that carries one", () => {
    const bodies = {
      "en verify (text)": `Hello Learner abcdef,\n\nYour verification code is: ${CODE}\n`,
      "en verify (html)": `<p>Hello Learner abcdef,</p><p>Your verification code is: <strong>${CODE}</strong></p>`,
      "en reset (text)": `Hello Learner abcdef,\n\nYour password reset code is: ${CODE}\n`,
      "en reset (html)": `<p>Your password reset code is: <strong>${CODE}</strong></p>`,
      "vi verify (text)": `Xin chào Learner abcdef,\n\nMã xác minh của bạn là: ${CODE}\n`,
      "vi reset (html)": `<p>Mã đặt lại mật khẩu của bạn là: <strong>${CODE}</strong></p>`,
    };

    for (const [name, body] of Object.entries(bodies)) {
      expect(extractOtpCode(body), name).toBe(CODE);
    }
  });

  // The regression. A learner whose random suffix came out all digits was
  // greeted by a name that looked exactly like a code, above the real one.
  test("ignores a six-digit display name printed above the code", () => {
    const body = `Hello Learner 157304,\n\nYour verification code is: ${CODE}\n\nIf you did not request this, please ignore this email.`;

    expect(extractOtpCode(body)).toBe(CODE);
  });

  test("ignores a six-digit recipient address in the html part", () => {
    const body = `<p>To: learner-j7-1787502065307-157304@example.com</p><p>Your verification code is: <strong>${CODE}</strong></p>`;

    expect(extractOtpCode(body)).toBe(CODE);
  });

  test("refuses a body with no labelled code rather than guessing", () => {
    expect(() => extractOtpCode("Hello Learner 157304, welcome aboard.")).toThrow(
      /labelled 6-digit OTP code/,
    );
  });
});

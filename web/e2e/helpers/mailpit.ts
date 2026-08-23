export interface MailpitMessageSummary {
  ID: string;
  Subject: string;
  From: {
    Address: string;
    Name: string;
  };
  To: {
    Address: string;
    Name: string;
  }[];
  Created: string;
}

export interface MailpitMessageDetail {
  ID: string;
  Subject: string;
  Text: string;
  HTML: string;
}

export interface MailpitListResponse {
  total: number;
  unread: number;
  count: number;
  messages: MailpitMessageSummary[];
}

const MAILPIT_HTTP_URL = process.env.MAILPIT_URL || "http://127.0.0.1:8025";

/**
 * Empties the whole Mailpit inbox.
 *
 * Only safe in a serial test. Under `fullyParallel` it deletes other workers'
 * messages — prefer `waitForEmail` with a recipient (and a subject) instead.
 */
export async function clearMailbox(): Promise<void> {
  try {
    await fetch(`${MAILPIT_HTTP_URL}/api/v1/messages`, {
      method: "DELETE",
    });
  } catch (err) {
    console.warn("Failed to clear Mailpit mailbox:", err);
  }
}

/**
 * Waits for a message addressed to `toEmail`, optionally matching a subject.
 *
 * Matching on the recipient — and never clearing the whole mailbox — is what
 * makes this safe under `fullyParallel`. Mailpit has one inbox for the entire
 * run, so a `DELETE /api/v1/messages` in one worker throws away the code
 * another worker is waiting for, and the failure surfaces as an unrelated
 * timeout in whichever test lost the race.
 */
export async function waitForEmail(
  toEmail: string,
  timeoutMs = 15000,
  intervalMs = 500,
  subjectPattern?: RegExp,
  after?: Date,
): Promise<MailpitMessageDetail> {
  const startTime = Date.now();

  while (Date.now() - startTime < timeoutMs) {
    try {
      const res = await fetch(`${MAILPIT_HTTP_URL}/api/v1/messages`);
      if (res.ok) {
        const data = (await res.json()) as MailpitListResponse;
        const matchingSummary = data.messages?.find(
          (msg) =>
            msg.To.some(
              (to) => to.Address.toLowerCase() === toEmail.toLowerCase(),
            ) &&
            (subjectPattern === undefined || subjectPattern.test(msg.Subject)) &&
            // `after` is what makes a second code readable. The inbox is shared
            // and never cleared, so the first message to this address is still
            // sitting in it; without a lower bound the wait returns instantly
            // with the stale code, the journey types a code the server has
            // already replaced, and the failure surfaces somewhere else
            // entirely — as a burned challenge with a disabled input.
            (after === undefined || new Date(msg.Created) > after),
        );

        if (matchingSummary) {
          const detailRes = await fetch(
            `${MAILPIT_HTTP_URL}/api/v1/message/${matchingSummary.ID}`,
          );
          if (detailRes.ok) {
            return (await detailRes.json()) as MailpitMessageDetail;
          }
        }
      }
    } catch {
      // Mailpit might be starting or temporary network blip
    }

    await new Promise((resolve) => setTimeout(resolve, intervalMs));
  }

  throw new Error(`Timeout waiting for email sent to ${toEmail} after ${timeoutMs}ms`);
}

/**
 * The six digits a verification or reset mail states after its label.
 *
 * Anchored on the label rather than on "the first six digits in the body".
 * Every template greets the recipient by display name before it states the
 * code, and `newLearner` builds that display name from a random base-36 suffix
 * which comes out all digits about once in two thousand learners. When it does,
 * a bare `\b\d{6}\b` returns the learner's own name: the helper typed 157304
 * into the OTP boxes, the screen answered "The code you entered is incorrect",
 * and the failure landed on whichever journey drew the unlucky suffix — on a
 * branch that had not touched auth at all.
 *
 * All four templates that carry a code write `... is: {{.Code}}` (`... là:` in
 * Vietnamese), with the HTML part wrapping the code in a tag.
 */
const OTP_AFTER_LABEL = /(?:is|là)\s*:\s*(?:<[^>]*>\s*)*(\d{6})\b/i;

export function extractOtpCode(content: string): string {
  const labelled = content.match(OTP_AFTER_LABEL);
  if (labelled?.[1]) {
    return labelled[1];
  }

  // A template that stops saying "is:" fails loudly here rather than quietly
  // going back to matching the greeting.
  throw new Error(
    `Could not extract a labelled 6-digit OTP code from email content: ${content}`,
  );
}

export function extractResetUrl(content: string): string {
  const match = content.match(/https?:\/\/[^\s"<>]+reset-password[^\s"<>]+/);
  if (!match || !match[0]) {
    throw new Error(`Could not extract password reset URL from email content: ${content}`);
  }
  return match[0];
}

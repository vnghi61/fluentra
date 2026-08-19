#!/usr/bin/env python3
"""Prove that no OTP code and no personal data reaches the log store.

P5.5 asks for this to be "verified by inspecting real output rather than
assuming". `TestTheCodeNeverReachesTheLogs` greps a buffer inside a unit test,
which is a different and weaker claim: it says the service did not format the
code into a record, not that nothing between the service and Loki put it there.

So this drives the real stack. It registers an account, reads the code Mailpit
actually received, verifies it, and then asks Loki whether any of it is
searchable. It ends with a control query for a line that must be present --
without that, five zeros could just as easily mean the query never matches.

Usage (with `make dev` running):

    python scripts/audit-log-privacy.py

Exits non-zero when anything is found, so it can gate a release check.
"""

import json
import re
import secrets
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

API = "http://127.0.0.1:8080"
MAILPIT = "http://127.0.0.1:8025"
LOKI = "http://127.0.0.1:3100"

# Long enough for the collector's 1s batch and Loki's ingester to settle. A
# shorter wait makes this pass for the wrong reason.
SETTLE_SECONDS = 10
WINDOW_SECONDS = 1800


def call(url, payload=None):
    body = json.dumps(payload).encode() if payload is not None else None
    request = urllib.request.Request(
        url,
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST" if body else "GET",
    )
    try:
        with urllib.request.urlopen(request) as response:
            return response.status, response.read().decode()
    except urllib.error.HTTPError as error:
        return error.code, error.read().decode()
    except urllib.error.URLError as error:
        sys.exit(f"cannot reach {url}: {error.reason}. Is `make dev` up?")


def otp_for(address):
    for _ in range(30):
        _, listing = call(f"{MAILPIT}/api/v1/messages")
        for message in json.loads(listing)["messages"]:
            if message["To"][0]["Address"] != address:
                continue
            _, detail = call(f"{MAILPIT}/api/v1/message/{message['ID']}")
            detail = json.loads(detail)
            found = re.search(
                r"\b(\d{6})\b", (detail.get("Text") or "") + (detail.get("HTML") or "")
            )
            if found:
                return found.group(1)
        time.sleep(1)
    sys.exit("no OTP arrived in Mailpit; the worker may not be running")


def occurrences(needle, label):
    query = '{service_name="fluentra-api"} |= `' + needle + "`"
    now = int(time.time())
    url = (
        f"{LOKI}/loki/api/v1/query_range?query={urllib.parse.quote(query)}"
        f"&start={now - WINDOW_SECONDS}000000000&end={now}000000000&limit=20"
    )
    _, body = call(url)
    streams = json.loads(body)["data"]["result"]
    lines = [value[1] for stream in streams for value in stream["values"]]
    print(f"  {label:<26} {len(lines)}")
    for line in lines[:3]:
        print(f"      !! {line[:200]}")
    return len(lines)


def main():
    address = f"pii-audit-{int(time.time())}@example.com"
    # Generated, never a literal. A hardcoded one would be a credential-shaped
    # string committed to the repository — which gitleaks reports, correctly,
    # and which would be an odd thing to carry in the script that exists to
    # prove secrets do not leak. Random also clears the breach-corpus check the
    # password policy applies.
    password = secrets.token_urlsafe(18)
    display_name = "Audit Subject"

    status, body = call(
        f"{API}/api/v1/auth/register",
        {"email": address, "password": password, "display_name": display_name},
    )
    if status != 201:
        sys.exit(f"register returned {status}: {body}")
    challenge = json.loads(body)["challenge_id"]

    code = otp_for(address)
    status, body = call(
        f"{API}/api/v1/auth/challenges/{challenge}/verify", {"code": code}
    )
    if status != 200:
        sys.exit(f"verify returned {status}: {body}")

    print(f"registered {address}, code delivered, verified. Waiting for the pipeline…")
    time.sleep(SETTLE_SECONDS)

    print("\nMaterial that must never be searchable:")
    leaked = sum(
        (
            occurrences(code, "the OTP code"),
            occurrences(address, "the email address"),
            occurrences(address.split("@")[0], "the email local part"),
            occurrences(display_name, "the display name"),
            occurrences(password, "the password"),
        )
    )

    print("\nControl — must be non-zero, or the searches above prove nothing:")
    control = occurrences("otp challenge issued", "a known log line")

    if control == 0:
        sys.exit("\nFAIL: no logs reached Loki at all, so the zeros above mean nothing.")
    if leaked:
        sys.exit(f"\nFAIL: {leaked} line(s) contain material that must not be logged.")
    print("\nPASS: nothing sensitive is searchable, and the pipeline is delivering.")


if __name__ == "__main__":
    main()

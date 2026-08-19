# The Google journeys (P5.4 journeys 3, 4 and 5)

These three run against **real Google**, with the OAuth client whose redirect URI
points at localhost. They are deliberately **not** part of CI:

- CI has no external network, which P5.4's card requires.
- Google blocks automated consent, so the consent screen needs a person.
- Pointing the backend at a stubbed IdP would mean making the Google token and
  authorization endpoints configurable, and
  `internal/modules/auth/service/oauth/google/exchange.go` refuses that on
  purpose: a deployment that can move those endpoints can be pointed at a server
  that mints its own ID tokens.

So they are a manual gate, run before a release rather than on every push.

## Running them

Bring the stack up, then:

```bash
cd web && E2E_GOOGLE=1 E2E_GOOGLE_EMAIL=you@gmail.com pnpm exec playwright test --project=google-manual
```

The browser opens headed. When it reaches Google's consent screen, sign in and
approve as you normally would — each spec then waits up to three minutes for the
app to finish the flow and asserts on where it lands.

`E2E_GOOGLE_EMAIL` must be the address of the Google account you sign in with:
journeys 4 and 5 create a local account on that same address first, which is the
whole point of what they test.

# ErrorBanner

An error says what failed, since when, and what happens next, without hiding what still works.

**Banner** `cl-banner` at the top of the sky: `color-surface-urgent`, a 2 px `color-status-urgent` edge, ■, a `text-body` sentence ("Connection lost — reconnecting (attempt 3), next in 5 s"). It replaces the old full-screen "CONNECTION LOST".

**Inline** `cl-inline-error`: ■, the sentence in `text-small`, then a secondary `Button` ("Retry").

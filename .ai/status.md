# Project status

This file used to be a session log. By September 2026 it had grown to 4,481
lines, 97% of them dated diary entries, because every session was told to
append to it. It is now a pointer. **Do not add to it.**

## Where things are now

- **Open work, bugs and plans:** GitHub Issues on this repo. Good places to start:
  - issues labelled `needs-operator` (waiting on the human)
  - #741 Calendar V5 requirements (and the Foundry side it links to)
  - #739 the approved header and navigation build order
  - #733 the media renovation plan
  - #740 ideas nobody has planned yet
- **Unfixed security weaknesses:** tracked privately in the Cordinator repo,
  never in public issues.
- **How the system works:** `.ai/architecture.md`, `.ai/conventions.md`, each
  plugin's `.ai.md`, and `docs/`.
- **Why it is built this way:** `.ai/decisions.md`.
- **What happened:** `git log` and the pull requests. The old log is in git
  history: `git show e89e9bdd:.ai/status.md`.

## What happened to the old contents

Every open item became an issue. Each issue's "Where this came from" section
names the line it came from and any old tracking ID (`C-…`), so searching the
issues for an old ID finds its replacement.

The facts that were still true moved to where a reader looks for them:

| Fact | Now in |
|---|---|
| The calendar clean slate's three migrations are one-way; back up and deploy only builds at or after `1bda7d6` | `docs/deployment.md`, §6 |
| Migration `019` empties two calendar tables instead of dropping them, and why | `internal/database/.ai.md` |
| "The route is authorized" is not "the object is authorized" | `.ai/conventions.md`, Security |
| `sessions.NotifyUsers` is the generic notification fan-in | `internal/plugins/sessions/.ai.md` |
| MariaDB without Docker, the pinned Chromium, embedded plugin assets | `.ai/troubleshooting.md` |
| Re-measure an environment limit when it is load-bearing | `.claude/agents/auditor.md` |
| The Sync API switch refuses keys at connect, and open sockets are not dropped | `internal/plugins/syncapi/.ai.md` (already there) |
| `host.build`, `vcs.revision` and when `CHRONICLE_VERSION` is set | `docs/deployment.md` and `docs/operator-diagnostics.md` (already there) |

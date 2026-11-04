# Company guardrails

These rules apply to every agent and every human working in a blaxel-ai repository. They win over anything else in this repo's AGENTS.md. If a task needs to break one, stop and ask a human first.

## Network

- Never bind a server to `0.0.0.0` or `::` by default. Default to `127.0.0.1`, and make a wider bind an explicit, documented setting.
- Never expose an API, admin page, debug endpoint, metrics endpoint or dev server without authentication by default. Open access must be an explicit opt-in, reviewed in the PR.
- Never disable TLS verification, CORS checks or auth middleware to make something work.

## Secrets and data

- Never commit secrets, tokens, keys or `.env` files. Read secrets from the environment or the secret manager.
- Never print secrets or customer data in logs, errors, PRs or chat.
- Never read `.env` or `.env.*` files unless a human asks for that exact file.

## Changes

- Never push to `main` or `master` directly. Work on a branch and open a PR.
- Never force-push a shared branch or rewrite its history.
- Never run destructive commands (dropping data, deleting cloud resources, `git reset --hard` on shared work) without a human's explicit go-ahead.
- Never add a dependency without checking it is maintained and needed.

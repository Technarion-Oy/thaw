# Security Policy

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues, pull requests,
or discussions.**

If you believe you have found a security vulnerability in Thaw, report it privately using
**one** of the following channels:

1. **GitHub private vulnerability reporting** (preferred) — go to the repository's
   **Security** tab → **Report a vulnerability**. This opens a private advisory visible only
   to the maintainers.
2. **Email** — send the details to **kalle.siukola@technarion.com**. Please use a subject line
   that begins with `SECURITY:` so it is triaged quickly. If you wish to encrypt the report,
   ask for a PGP key in an initial (non-sensitive) message.

Please include as much of the following as you can:

- A description of the vulnerability and its impact.
- The affected version, platform (macOS / Windows / Linux), and configuration.
- Step-by-step reproduction instructions or a proof of concept.
- Any relevant logs, screenshots, or crash reports (with secrets redacted).

## What to expect

- **Acknowledgement** within **3 business days**.
- An initial assessment and severity triage within **10 business days**.
- Regular updates on remediation progress until the issue is resolved.
- Coordinated disclosure: we will agree a disclosure timeline with you and credit you in the
  advisory (unless you prefer to remain anonymous).

Please give us a reasonable opportunity to fix the issue before any public disclosure.

## Scope

In scope: the Thaw application source in this repository (Go backend, React/TypeScript
frontend, and the release/CI tooling).

Out of scope: vulnerabilities in Snowflake itself, in third-party dependencies (report those
upstream), or issues that require a compromised local machine or physical access.

## Secure development notes

Thaw builds all SQL through the domain-package `Build*Sql` builders, which quote identifiers
via `internal/snowflake` helpers — user input is never concatenated into SQL inline. Git
tokens are never written to disk; the AI API key is written with mode `0600`. See the
**Security** section of [`CONTRIBUTING.md`](CONTRIBUTING.md) for the coding rules contributors
must follow.

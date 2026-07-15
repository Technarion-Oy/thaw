# Collaboration Guide

Thank you for your interest in contributing to **Thaw**. This guide explains how outside
contributors get involved and what to expect from the process. It is intentionally short: the
mechanics of branching, commits, PRs, and the docs-with-code rule already live in
[`CONTRIBUTING.md`](CONTRIBUTING.md) — please read that first. This document covers the
*social* side: how to propose work, how review happens, and what we expect of each other.

## Ways to contribute

- **Report bugs** — open a [bug report](.github/ISSUE_TEMPLATE/bug_report.yml). Include your
  OS, Thaw version, and clear reproduction steps.
- **Request features** — open a [feature request](.github/ISSUE_TEMPLATE/feature_request.yml)
  describing the problem you want solved, not just the solution you have in mind.
- **Improve documentation** — docs fixes are first-class contributions and are often the best
  first PR.
- **Submit code** — fix a bug or implement a feature. For anything non-trivial, please open an
  issue first so we can agree on the approach before you invest time.

## Before you start coding

1. **Search existing issues and PRs** to avoid duplicate work.
2. **Open or comment on an issue** for anything beyond a small fix, and wait for a maintainer
   to confirm the direction. This saves everyone from rejected PRs.
3. **Read the relevant docs** for the area you're touching — the per-package `README.md`,
   [`docs/concepts/`](docs/concepts/), and the semantic map (see
   [`CONTRIBUTING.md`](CONTRIBUTING.md#codebase-navigation--the-semantic-map)).

## The contribution workflow

The full mechanics are in [`CONTRIBUTING.md`](CONTRIBUTING.md). In brief:

1. **Fork** the repository and create a topic branch — never work on `main`.
2. Make your change, **including docs and tests in the same PR** (the docs-with-code rule).
3. Ensure the quality gates pass locally (type-check, `go test ./...`, lint).
4. **Title your PR using [Conventional Commits](https://www.conventionalcommits.org/)** — e.g.
   `feat: add copy-as-CSV to the results grid`. CI (`pr-title-check`) enforces this, and the
   [PR template](.github/PULL_REQUEST_TEMPLATE.md) will walk you through the checklist.
5. **Sign the [Contributor License Agreement](CLA.md)** — the CLA bot comments on your first PR
   with a one-click signing link. A PR cannot be merged until the CLA is signed.

## Review process

- A maintainer will review your PR, usually within a week. Be patient — this is maintained by a
  small team.
- Reviews focus on correctness, fit with existing patterns, test coverage, and documentation.
  Expect questions and change requests; they are a normal part of collaboration, not a
  rejection.
- Keep PRs **focused and small** where possible. A large PR that mixes unrelated changes is
  slower to review and more likely to stall.
- For fork PRs, CI on self-hosted runners requires maintainer approval before it runs — this is
  a security measure, not a comment on your contribution.
- Once approved and green, a maintainer merges (squash-merge is the default).

## Expectations

- Be respectful and constructive. All participation is governed by our
  [Code of Conduct](CODE_OF_CONDUCT.md).
- Assume good faith. Maintainers and contributors are volunteers giving their time.
- Discuss significant design decisions in the open (issue or PR thread) so the reasoning is
  recorded for future contributors.

## Questions

Open a [GitHub Discussion](../../discussions) (if enabled) or a regular issue for general
questions. For security concerns, follow [`SECURITY.md`](SECURITY.md) instead — never file a
public issue for a vulnerability.

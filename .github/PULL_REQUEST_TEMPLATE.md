<!--
Thanks for contributing to Thaw!

PR TITLE — required. Your PR title MUST use Conventional Commits, or CI (pr-title-check) fails.
Use one of these prefixes:

  feat:      a new feature
  feat!:     a breaking feature change
  fix:       a bug fix
  perf:      a performance improvement
  refactor:  a code change that neither fixes a bug nor adds a feature
  docs:      documentation only
  style:     formatting, no code-behavior change
  test:      adding or fixing tests
  build:     build system or dependencies
  chore:     tooling / housekeeping
  ci:        CI configuration

  Example:  feat: add copy-as-CSV to the results grid
-->

## Summary

<!-- What does this PR do, and why? Link the issue it closes. -->

Closes #

## Changes

<!-- Bullet the notable changes. -->

-

## How was this tested?

<!-- Describe manual testing and which automated tests you ran. -->

- [ ] `go test ./...`
- [ ] `cd frontend && npx tsc --noEmit`
- [ ] `cd frontend && npm test`
- [ ] Manually verified in the running app (`wails dev`)

## Checklist

- [ ] PR title follows [Conventional Commits](https://www.conventionalcommits.org/) (see the
      comment above)
- [ ] I have read [`CONTRIBUTING.md`](../CONTRIBUTING.md) and [`COLLABORATION.md`](../COLLABORATION.md)
- [ ] **Docs updated in this PR** where behavior changed (README / FEATURES.md / the relevant
      `README.md` / `docs/concepts/*` — the docs-with-code rule)
- [ ] The semantic map is still accurate (regenerated with `go generate ./internal/architecture/`
      if I added/moved packages or annotated files)
- [ ] I have signed the [Contributor License Agreement](../CLA.md) (the CLA bot will prompt me)
- [ ] No secrets, credentials, or large build artifacts are included in this PR

/** @type {import('semantic-release').GlobalConfig} */
export default {
  branches: ["main"],
  plugins: [
    [
      "@semantic-release/commit-analyzer",
      {
        preset: "angular",
        releaseRules: [
          { type: "feat", release: "minor" },
          { type: "fix", release: "patch" },
          { type: "perf", release: "patch" },
          { type: "refactor", release: false },
          { type: "chore", release: false },
          { type: "docs", release: false },
          { type: "style", release: false },
          { type: "test", release: false },
          { type: "build", release: false },
          { type: "ci", release: false },
        ],
      },
    ],
    [
      "@semantic-release/release-notes-generator",
      {
        preset: "angular",
      },
    ],
    [
      "@semantic-release/changelog",
      {
        changelogFile: "release/CHANGELOG.md",
      },
    ],
    [
      "@semantic-release/exec",
      {
        prepareCmd: "node scripts/update-version.js ${nextRelease.version}",
      },
    ],
    [
      "@semantic-release/git",
      {
        assets: ["wails.json", "release/CHANGELOG.md"],
        message:
          "chore(release): ${nextRelease.version} [skip ci]\n\n${nextRelease.notes}",
      },
    ],
    [
      "@semantic-release/github",
      {
        // Binaries are attached by build.yml via softprops/action-gh-release.
        // semantic-release only creates the GitHub Release (notes + tag).
        assets: [],
        // Don't post "released in vX" comments on referenced issues/PRs.
        // A commit body referencing a PR number (e.g. #332) makes the success
        // step query repository.issue(number:), which returns NOT_FOUND for a
        // PR and crashes the whole release job. failComment/failTitle disabled
        // so a failed release doesn't open a tracking issue either.
        successComment: false,
        failComment: false,
        failTitle: false,
      },
    ],
  ],
};

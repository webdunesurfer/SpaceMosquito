# Release

How to cut a GitHub Release. CI: [`.github/workflows/release.yml`](../.github/workflows/release.yml) (runs on tag `v*`).

## Before tagging

1. Main is green: `cd spacemosquito && go test -race ./...`
2. [`CHANGELOG.md`](../CHANGELOG.md) has the version section you want (move items out of Unreleased).
3. Clear shipped items from [`BACKLOG.md`](BACKLOG.md); leave open work.
4. Update [`STATUS.md`](STATUS.md) if focus changes.

## Cut the release

```sh
git tag v0.1.0          # use the version from CHANGELOG
git push origin v0.1.0
```

Watch **Actions → Release**. It tests, builds four binaries, uploads them + `SHA256SUMS`, and creates the GitHub Release for that tag.

Alternative: GitHub **Releases → Draft a new release → create tag `v0.1.0` → Publish**. That also pushes the tag and triggers the same workflow (assets attach to the release).

## After

1. Confirm assets on the release page.
2. Set [`STATUS.md`](STATUS.md) next focus (usually next backlog item).

## Local binaries only (no GitHub)

```sh
cd spacemosquito
./scripts/build-release.sh v0.1.0
ls dist/
```

# Release

## Prepare

1. Ensure the build is green: `make test`
2. Update [`CHANGELOG.md`](../CHANGELOG.md) to the new version (move items out of Unreleased). Keep changeloge entries conscise, focus on user-facing functionality changes, skip technical details.
3. Clear shipped items from [`BACKLOG.md`](BACKLOG.md); leave open work.
4. Update [`STATUS.md`](STATUS.md) if the focus changes.
5. Bump firefox-extension and chrome-extension `manifest.json` + `package.json` to X.Y.Z

## Cut the release

```sh
git tag vX.Y.Z         # use the version from CHANGELOG
git push origin vX.Y.Z
```

Watch **Actions → Release**. It tests, builds four binaries + Firefox/Chrome
extension zips, uploads them + `SHA256SUMS`, and creates the GitHub Release for
that tag.

Alternative: GitHub **Releases → Draft a new release → create tag `vX.Y.Z` → Publish**. That also pushes the tag and triggers the same workflow (assets attach to the release).

## After

1. Confirm assets on the release page:
   - `spacemosquito-{darwin,linux,windows}-*` binaries
   - `spacemosquito-firefox-v*.zip` and `spacemosquito-chrome-v*.zip`
   - `SHA256SUMS` covering all of the above
2. Spot-check: unzip an extension zip — `manifest.json` must be at the archive root.
3. Set [`STATUS.md`](STATUS.md) next focus (usually next backlog item).

## Local release artifacts (no GitHub)

Produces the same binary + extension zip layout as CI:

```sh
cd spacemosquito
./scripts/build-release.sh vX.Y.Z
ls dist/
```

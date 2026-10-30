# Releasing agni

agni is a **single Go module**, `github.com/panyam/agni`.  `examples/` and `docsite/` carry their own
`go.mod` files but are not meant to be imported, so `SUB_MODS_TO_TAG` in the `Makefile` is empty and a
release creates exactly one tag.  Add a path there the moment something under it is meant to be
imported, or `go get` against it resolves to a pseudo-version instead of the release.

## Versioning

Semantic versioning.  Tags are `v`-prefixed: `v0.2.0`.

Pre-1.0, breaking API changes may ride a **minor** release.  They are called out under a Breaking
heading in the release note with a migration line each.  Patch releases never break API.

A pre-release is `vX.Y.Z-bN`, for example `v0.2.0-b1`.  **The hyphen is mandatory**; `v0.2.0b1` is not
valid Go SemVer and will not resolve.  Pre-releases sort below the final version, so `@latest` keeps
returning the highest stable tag and a consumer opts in explicitly.

## The version string has one source

The git tag.  Nothing in the tree records the version.

`internal/version` reports it, and the binary gets it at build time through
`-ldflags "-X .../internal/version.stamped=$VERSION"`.  The image passes it as a `--build-arg`, because
the image has no `.git` to derive it from.  There is no `VERSION` file and no constant to bump, so
there is nothing that can drift out of step with the tag.

## Cutting a release

From a green `main`:

1. **Merge and confirm you are current.**  `git fetch origin && git merge --ff-only origin/main`.
   Tagging a stale checkout is the easy mistake here, and it is not safely reversible.

2. **Run the gate.**  `make testall`.  Read `docsite/content/build/the-gate.md` before trusting the
   result: it has three traps that make a red gate read green, and one that makes a green tree read
   red.  For a release, also run `make oracle`, which is not in the gate: it needs the 19MB
   both-views corpus rather than the 3MB `testall` fetches.  `make browser-test` needs no separate
   run, having been part of `testall` since PR 629.

3. **Write the notes.**  `RELEASES/v<X.Y.Z>.md` is the canonical write-up and doubles as the GitHub
   Release body.  Add the summary entry to `CHANGELOG.md`, newest on top, linking the release file.
   Commit both on their own, separate from any code change.

4. **Tag.**  `make tag V=vX.Y.Z` creates the tag locally so you can inspect it.  `make tag-push
   V=vX.Y.Z` does both.  A pushed tag is immediately resolvable by the Go module proxy and is not
   safely retractable, so inspect first when in doubt.

5. **Watch the release workflow.**  Pushing a `v[0-9]+.[0-9]+.[0-9]+` tag fires
   `.github/workflows/release.yml`, which builds the image for amd64 and arm64, pushes it to GHCR as
   both `:vX.Y.Z` and `:latest`, then verifies the published image reports the tag and can actually run
   a check.  A dispatch re-run takes the tag as an input so it can never publish an image built from an
   untagged `main`.  The same run builds the CLI's release binaries (`make dist`) and attaches them
   to the tag's GitHub Release.  It also builds the Python client with the tag's version, installs the
   wheel into a fresh venv to check it (`make python-dist`), and publishes it to PyPI as `agni-eda`.

6. **Publish the GitHub Release.**

   ```
   make compose-release V=vX.Y.Z
   GH_TOKEN="$GH_PERSONAL_TOKEN" hack/release_attach.sh vX.Y.Z dist/compose.yaml dist/env.example
   GH_TOKEN="$GH_PERSONAL_TOKEN" gh release edit vX.Y.Z \
     --title vX.Y.Z --notes-file RELEASES/vX.Y.Z.md --draft=false
   ```

   The two files are the compose bundle a customer downloads to try the release on their own
   boards (agni issue 1057), `deploy/compose.yaml` with the release's version as its default. The
   release workflow has already brought it up over the published image (`hack/compose_smoke.sh`).

   The workflow's `binaries` job attaches the CLI's tarballs, the viewer bundle and `checksums.txt`
   (`make dist`, WS16-003) through the same script, which creates the release as a draft when it
   gets there first. So this step attaches and edits rather than creating, and works in either order.
   Publish only once the job has attached them: a `go install` binary downloads its viewer from the
   published release, and `agni open` on one fails until the bundle is there.

   Add `--prerelease` to the edit for a `-bN` tag.

7. **Verify resolution** from a scratch module:

   ```
   GOPROXY=direct GOFLAGS=-mod=mod go list -m github.com/panyam/agni@vX.Y.Z
   ```

## Token note

The default `gh` login on this machine is an account with only `pull` on `panyam/agni`, so
`gh release create` returns 403 under it.  Pass the personal token explicitly:
`GH_TOKEN="$GH_PERSONAL_TOKEN" gh ...`.

Pushing tags is unaffected, since that goes over SSH via the `panyam-github` host alias.

## GHCR visibility

Nothing to configure, which is worth stating because the widely repeated advice says otherwise.  A
GHCR package pushed by a workflow living in the repository it belongs to is linked to that repository
and takes its visibility, so publishing from this public repo yields a publicly pullable image.  The
workflow's verify step authenticates as the workflow, so it proves the image runs and reports its tag
but says nothing about whether a stranger can pull it.  Only an anonymous pull from a
logged-out client tests that.

## PyPI

The Python client publishes as `agni-eda`, because PyPI's `agni` is an unrelated project.  The
`pypi` job uses trusted publishing, so no token is stored in the repository or its secrets.  PyPI
accepts the workflow's OIDC identity because the `agni-eda` project names it as a publisher, which
is a one-time setup on pypi.org under the owner's account:

- Publisher: GitHub, owner `panyam`, repository `agni`, workflow `release.yml`, environment `pypi`.
  Before the first upload this is a pending publisher, added under "Your account" then
  "Publishing", and the first upload creates the project.
- GitHub creates the `pypi` environment the first time the job runs, with no protection rules.
  Adding a required reviewer to it would hold every release at `waiting`, as the Pages environment
  once did.

The pyproject.toml in the tree says `0.0.0`.  `make python-dist V=vX.Y.Z` stamps the tag into the
copy it builds, so the tag stays the version's one source.  A version PyPI has accepted cannot be
uploaded again, even after deleting it, so a broken client release is fixed by the next tag.

## Checklist

- [ ] `git merge --ff-only origin/main` clean, working tree clean
- [ ] `make testall` green
- [ ] `make oracle` green (`browser-test` rides in `testall`)
- [ ] `RELEASES/vX.Y.Z.md` written; `CHANGELOG.md` entry added and linked
- [ ] Notes committed and pushed to `main`
- [ ] `make tag-push V=vX.Y.Z`
- [ ] `release.yml` green, image pullable and reporting the tag
- [ ] Both images pull ANONYMOUSLY, from a logged-out client: `docker logout ghcr.io`, then
      `docker pull ghcr.io/panyam/agni:vX.Y.Z` and `docker pull ghcr.io/panyam/agnids:vX.Y.Z`. The
      first release after agni 744 is the first to publish `agnids`, so its visibility is untested
      until then (see GHCR visibility above)
- [ ] GitHub Release published from the notes file
- [ ] `pip install agni-eda==X.Y.Z` in a fresh venv, then `python -c "import agni"` from outside
  the checkout
- [ ] `go list -m github.com/panyam/agni@vX.Y.Z` resolves

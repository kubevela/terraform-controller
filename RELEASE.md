# Releasing terraform-controller

terraform-controller is released as the controller image and its Helm chart. KubeVela uses it in two ways: its
`terraform` addon ([kubevela/catalog](https://github.com/kubevela/catalog/tree/master/addons/terraform)) installs the
chart, and KubeVela's `go.mod` requires the Go module for the `Configuration` API types. A fix reaches users through a
terraform-controller patch release, then the addon's chart version and KubeVela's `go.mod`.

## Branches

| Branch | What it is | Tags | KubeVela `go.mod` |
| --- | --- | --- | --- |
| `master` | development; the next minor release | none | — |
| `release-0.9` | the 0.9 line | `v0.9.x` (`v0.9.1` today) | `master`, `release-1.12` |
| `release-0.8` | the 0.8 line | `v0.8.x` (`v0.8.2` today) | `release-1.10`, `release-1.11` |

The release branches get security and bug fixes; new features go to `master`. A new minor line starts with a
`release-x.y` branch cut from `master` at the commit of its first tag.

## Backports

A fix goes to `master` first, then to the release branches that need it. The
[Backport workflow](.github/workflows/backport.yml) opens the backport pull requests:

- **Before merging**, add a label per branch: `backport release-0.8`, `backport release-0.9`. When the pull request is
  merged, the workflow cherry-picks its commits (with `-x`) onto each branch and opens one pull request per branch.
- **After merging**, a maintainer comments `/backport release-0.8` (or `/backport 0.8`; several branches separated by
  spaces) on the merged pull request. Adding a label after the merge does nothing.

If a commit doesn't apply, the workflow says so in a comment and the run fails; backport that branch by hand
(`git cherry-pick -x`) in a pull request titled `[Backport release-x.y] <title>`.

## What the workflows publish

[BuildImage](.github/workflows/build-image.yml) builds the controller for `linux/amd64` + `linux/arm64`:

| Trigger | Result |
| --- | --- |
| a pull request to `master` or `release-*` | built, not pushed |
| a manual run on `master` or `release-x.y` (Actions → BuildImage → Run workflow) | `<branch>-<short sha>` on `docker.io/oamdev/terraform-controller` and `ghcr.io/kubevela/oamdev/terraform-controller`: a candidate, not for users. Other refs are refused |
| a `vX.Y.Z` tag | `vX.Y.Z` on both registries; `latest` moves only if it is the highest final release (not an older line's patch or a pre-release) |

[HelmChart](.github/workflows/chart.yml), on a `v*` tag: packages `chart/` with the chart version and image tag taken
from the tag (`vX.Y.Z` → chart `X.Y.Z`, image `vX.Y.Z`) and publishes it to <https://kubevela.github.io/charts>
(`helm repo add kubevela https://kubevela.github.io/charts`). It refuses to run on anything but a tag.

[Release](.github/workflows/release.yml), run by hand on a tag after its GitHub Release exists, builds the
`backup_restore` tool and attaches it to that release.

[Unit Test](.github/workflows/unit-test.yml), [E2E Test](.github/workflows/e2e-test.yml),
[Format](.github/workflows/format.yml) and [License](.github/workflows/license.yml) run on every push to `master` and
`release-*`, on pull requests to them, and (the tests) on tags.

## Making a release

1. Merge the fixes into the release branch (backports above).
2. Build a candidate: Actions → BuildImage → Run workflow on `release-x.y` publishes
   `oamdev/terraform-controller:release-x.y-<short sha>`.
3. Test the candidate: scan it, and install the branch's chart with the candidate image
   (`helm install ./chart --set image.tag=release-x.y-<short sha>`); apply a `Configuration`, and upgrade from the
   previous release with one already applied.
4. Tag the tested commit, `vX.Y.Z` on `release-x.y`. Tags are never moved or reused: a fix after tagging is the next
   patch.
5. Check the published image (it is rebuilt from the tag, so its digest differs from the candidate's), `latest`, and
   the chart in the Helm repository; then publish the GitHub Release with the release notes: what changed, the image
   digest, and how to upgrade. Mark only the highest final release as Latest; a `-alpha.N` / `-rc.N` tag is a
   pre-release.
6. Update KubeVela: `go.mod` on the KubeVela branches that use the line (table above), and the chart version of the
   `terraform` addon in kubevela/catalog. Then update the table above and in the [README](README.md).

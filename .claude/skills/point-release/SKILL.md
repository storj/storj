---
name: point-release
description: Create a Storj point release by cherry-picking commits onto an existing release branch, opening a PR, and tagging. Use when the user wants to cherry-pick a fix into a release branch after it was cut (e.g. "create a point release for v1.38", "cherry-pick <hash> onto release-v1.38").
allowed-tools: bash, git
---

# Storj Point Release

Create a point release when you need to land a fix on a release branch **after** that
release branch has already been cut. This is done by cherry-picking commit(s) onto the
release branch, opening a pull request against it, and — once merged — creating and
pushing the point release tag.

Point release versions increment the PATCH component: `v1.38` → `v1.38.1` → `v1.38.2`, etc.

## Inputs you need

Before starting, determine (ask the user if not provided):

- **Release branch** — e.g. `release-v1.38`.
- **Commit hash(es)** to cherry-pick — the fix(es) already merged on `main`.
- **Point release version** — e.g. `v1.38.1`. If unknown, inspect existing tags to pick
  the next patch number (see step 5).

## Steps

### 1. Fetch the latest changes

Make sure your local repo has the latest remote state before branching:

```bash
git fetch origin --tags
```

### 2. Check out the release branch

```bash
git checkout release-v1.38
git pull origin release-v1.38
```

Replace `release-v1.38` with the actual release branch. Confirm it exists on the remote
first if unsure: `git branch -r | grep release-v1.38`.

### 3. Cherry-pick the commit(s)

```bash
git cherry-pick <commit-hash>
```

For multiple commits, list them in the order they should be applied (oldest first):

```bash
git cherry-pick <hash1> <hash2>
```

**If there are conflicts**: stop and surface them to the user. Show the conflicting files
(`git status`), do not guess at resolutions for release code. After the user resolves,
continue with `git cherry-pick --continue`. To abort: `git cherry-pick --abort`.

### 4. Create a point release branch and push it

Do **not** push the cherry-pick directly to the release branch. Create a separate branch:

```bash
git checkout -b point_release
git push origin point_release
```

Use a more descriptive branch name when helpful, e.g. `point_release_v1.38.1`, to avoid
collisions with previous point releases.

### 5. Open a pull request against the release branch

Open a PR **targeting the release branch** (not `main`):

```bash
gh pr create --base release-v1.38 --head point_release \
  --title "Point release v1.38.1" \
  --body "Cherry-pick <hash> onto release-v1.38 for point release v1.38.1"
```

Then **wait for the PR to be reviewed and merged** before continuing. Do not tag until the
PR is merged into the release branch.

### 6. Create the release tag

Only after the PR is merged. First, refresh the release branch so it includes the merged
cherry-pick, then confirm the next patch version by looking at existing tags:

```bash
git checkout release-v1.38
git pull origin release-v1.38
git tag -l 'v1.38.*' | sort -V
```

Create the tag with the release script (never plain `git tag` — the script sets release
defaults and verifies a clean working tree):

```bash
./scripts/tag-release.sh v1.38.1
```

`scripts/tag-release.sh` requires a clean working tree and a version matching
`vMAJOR.MINOR.PATCH`.

### 7. Push the branch and the tag

```bash
git push origin
git push origin v1.38.1
```

## Guardrails

- **Never push to any remote without explicit confirmation from the user.** Steps 4 and 7
  push to `origin` — confirm with the user before running them.
- Do **not** create or push the tag until the PR is merged into the release branch.
- Do **not** resolve cherry-pick conflicts on release code yourself — surface them and let
  the user decide.
- The tag version must match the release branch (`release-v1.38` → `v1.38.x`).
- Always use `./scripts/tag-release.sh`, not `git tag`, so release-mode defaults are set.

## Quick reference

```bash
git fetch origin --tags
git checkout release-v1.38 && git pull origin release-v1.38
git cherry-pick <commit-hash>
git checkout -b point_release
git push origin point_release
gh pr create --base release-v1.38 --head point_release --title "Point release v1.38.1"
# ... wait for review + merge ...
git checkout release-v1.38 && git pull origin release-v1.38
./scripts/tag-release.sh v1.38.1
git push origin
git push origin v1.38.1
```

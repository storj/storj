export BRANCH_DIR="$(pwd)"
export RELEASE_DIR="$(pwd)/release"
latestReleaseTag=$(git describe --tags `git rev-list --tags --max-count=1`)
latestReleaseCommit=$(git rev-list -n 1 "$latestReleaseTag")
echo "Checking out latest release tag: $latestReleaseTag"
git worktree add -f $RELEASE_DIR "$latestReleaseCommit"

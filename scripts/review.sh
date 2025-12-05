#!/usr/bin/env bash

prompt=$(cat <<'EOF'

Review the change of `git show HEAD` with the code-reviewer. IMPORANT: ignore ALL uncommitted changes. Check only the changes which are committed.

EOF
)


claude --verbose --print --dangerously-skip-permissions "$prompt"

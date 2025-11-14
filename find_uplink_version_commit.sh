#!/bin/bash
# find_uplink_version_commit.sh
# 
# This script finds the last commit where go.mod contained a specific version
# of the storj.io/uplink dependency. It can search for exact versions or version
# prefixes.
#
# Usage:
#   ./find_uplink_version_commit.sh <version_pattern>
#
# Examples:
#   ./find_uplink_version_commit.sh "v1.4.5"        # Find v1.4.5 (with or without postfix)
#   ./find_uplink_version_commit.sh "v1.4.5-"       # Find v1.4.5-<postfix> only
#   ./find_uplink_version_commit.sh "v1.13.2"       # Find v1.13.2 versions

set -euo pipefail

VERSION_PATTERN="${1:-v1.4.5}"

echo "============================================================================"
echo "Finding last commit where go.mod contained storj.io/uplink ${VERSION_PATTERN}*"
echo "============================================================================"
echo ""

# Get commits in reverse chronological order (newest first) that might match
# Using -S for pickaxe search which finds commits that add or remove the pattern
commits=$(git log --all -S "storj.io/uplink ${VERSION_PATTERN}" --pickaxe-regex --pretty=format:"%H" -- go.mod)

if [ -z "$commits" ]; then
    echo "No commits found containing storj.io/uplink ${VERSION_PATTERN}"
    exit 1
fi

found=false

for commit in $commits; do
    # Get the uplink line from this commit
    line_info=$(git show "$commit:go.mod" 2>/dev/null | grep -n "storj.io/uplink" || echo "")
    
    if [ -n "$line_info" ]; then
        line_num=$(echo "$line_info" | cut -d: -f1)
        line_value=$(echo "$line_info" | cut -d: -f2- | xargs)
        
        # Check if this line contains our version pattern
        if echo "$line_value" | grep -q "storj.io/uplink ${VERSION_PATTERN}"; then
            if [ "$found" = false ]; then
                echo "RESULT: Last commit where go.mod contained storj.io/uplink ${VERSION_PATTERN}*:"
                echo ""
                echo "Commit Hash: $commit"
                echo "Commit Date: $(git log -1 --pretty=format:"%ai" "$commit")"
                echo "Author: $(git log -1 --pretty=format:"%an <%ae>" "$commit")"
                echo "Subject: $(git log -1 --pretty=format:"%s" "$commit")"
                echo ""
                echo "In go.mod:"
                echo "  Line Number: $line_num"
                echo "  Line Content: $line_value"
                echo ""
                
                # Check if this was ever on line 95
                current_line=$(git show "$commit:go.mod" 2>/dev/null | sed -n '95p' || echo "")
                if [ "$line_num" = "95" ]; then
                    echo "NOTE: In this commit, storj.io/uplink WAS on line 95!"
                else
                    echo "NOTE: In this commit, storj.io/uplink was on line $line_num, not line 95."
                    if [ -n "$current_line" ]; then
                        echo "      Line 95 in that commit contained: $current_line"
                    else
                        # Store line count in variable for proper error handling
                        line_count=$(git show "$commit:go.mod" 2>/dev/null | wc -l)
                        echo "      Line 95 did not exist in that commit (file had ${line_count} lines)"
                    fi
                fi
                echo ""
                echo "Full commit details:"
                git log -1 --stat "$commit"
                
                found=true
                break
            fi
        fi
    fi
done

if [ "$found" = false ]; then
    echo "No commits found where go.mod contained storj.io/uplink ${VERSION_PATTERN}*"
    exit 1
fi

echo ""
echo "============================================================================"

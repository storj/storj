# Finding the Last Commit for storj.io/uplink v1.4.5

## Problem Statement
Find the last commit where line 95 of go.mod contained `storj.io/uplink v1.4.5-<postfix>`.

## Investigation Results

### Summary
After searching through the complete git history of the repository:

1. **Line 95 specifically**: Line 95 of go.mod has NEVER contained `storj.io/uplink v1.4.5` (with or without postfix).

2. **Any line in go.mod**: The `storj.io/uplink v1.4.5` dependency appeared only ONCE in the history:
   - **Commit**: `424d2787eb91416a50bbc9e0cc51d220339eac70`
   - **Date**: 2021-01-14 12:23:33 +0200
   - **Author**: Kaloyan Raev <kaloyan@storj.io>
   - **Subject**: go.mod: bump deps to uplink v1.4.5
   - **Line Number**: 50 (not 95)
   - **Exact Value**: `storj.io/uplink v1.4.5` (no postfix)

3. **Version with postfix**: There are NO commits in the repository history where `storj.io/uplink v1.4.5-<postfix>` existed. The version was exactly `v1.4.5` without any postfix/suffix.

4. **Why not line 95**: At the time of commit `424d2787eb91416a50bbc9e0cc51d220339eac70`, the go.mod file only had 51 lines total, so line 95 didn't exist yet.

## How to Verify

A script `find_uplink_version_commit.sh` has been created in the repository root that can search for any version of the uplink dependency:

```bash
# Find v1.4.5 (with or without postfix)
./find_uplink_version_commit.sh "v1.4.5"

# Find v1.4.5 with postfix only
./find_uplink_version_commit.sh "v1.4.5-"

# Find any other version
./find_uplink_version_commit.sh "v1.13.2"
```

## Conclusion

**Answer**: If the question is asking for line 95 specifically containing `v1.4.5-<postfix>`, the answer is: **NO SUCH COMMIT EXISTS**.

The closest match is:
- **Commit `424d2787eb91416a50bbc9e0cc51d220339eac70`**: Last (and only) commit with `storj.io/uplink v1.4.5` (without postfix), but it was on line 50, not line 95.

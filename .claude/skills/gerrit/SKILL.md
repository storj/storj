---
name: gerrit
description: >
    Interact with Storj Gerrit team code collaboration service, which we just called it gerrit,
    when the user ask to do it
allowed-tools: bash, git, WebFetch
---

# Storj Gerrit

Storj has a Gerrit service hosted on review.dev.storj.tools sub-domain. This document refers to it
as Gerrit.

The service allows read-only public access to the open source repositories. Write access requires
users to have an account.

This skill focus on users with write access.

Repositories are under `storj/` path. This repository URL is review.dev.storj.tools/c/storj/storj

This document use curly-brackets as variable values substitutions for URL, command, etc., patterns.

## Access configuration

Verify that the user has a Gerrit remote configured with SSH and the "commit-msg" hook is present.
`commit-msg` hook is in `.git/hooks/commit-msg`, must have executable permissions and its content
must have "From Gerrit Code Review".

Access is configured if user has both.

### "commit-msg" hook

If user doesn't have the hook download it executing

```
mkdir -p `git rev-parse --git-dir`/hooks/ \
    && curl -Lo `git rev-parse --git-dir`/hooks/commit-msg https://review.dev.storj.tools/tools/hooks/commit-msg \
    && chmod +x `git rev-parse --git-dir`/hooks/commit-msg
```

If it has it, but it doesn't contain "From Gerrit Code Review", tell them about it and to download
it manually and decide how to merge their logic.

### Configure remote

The use doesn't have a Gerrit remote configured.

Tell them that you need a remote to interact with Gerrit and they need to have an account; ask them
if they have one.

If they don't have it, tell them to ask to some Storj employee how to get one and to ask you again
to configure the access when they get it.

When they have an account, check if they have already a remote called `origin` and `gerrit`.
Ask them what's their username and what name they want for the Gerrit remote, suggesting `origin` if
it doesn't exist, otherwise `gerrit`, and if they have both list them to the user and don't suggest
any.

Add the new remote with
`git remote add {remote-name} "ssh://{username}@review.dev.storj.tools:29418/storj/storj"`

And download the hook as mentioned in the above "commit-msg hook" section.

## Post a review to a change

You can post review comments if you POST a json file to '/changes/{change-id}/revisions/{revision-id}/review'

`review.json` example:

```json
  {
    "tag": "jenkins",
    "message": "Some nits need to be fixed.",
    "labels": {
      "Code-Review": -1
    },
    "comments": {
      "gerrit-server/src/main/java/com/google/gerrit/server/project/RefControl.java": [
        {
          "line": 23,
          "message": "[nit] trailing whitespace"
        },
        {
          "line": 49,
          "message": "[nit] s/conrtol/control"
        },
        {
          "range": {
            "start_line": 50,
            "start_character": 0,
            "end_line": 55,
            "end_character": 20
          },
          "message": "Incorrect indentation"
        }
      ]
    }
  }
```

You should use `./scripts/submit_review.sh` script to post reviews.

Example:

./scripts/submit_review.sh review.json $(git rev-parse HEAD)
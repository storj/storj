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

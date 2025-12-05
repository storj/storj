# Purpose

To provide useful information for developers and maintainers of Storj.

# Tooling

## Development

[`storj-up`](https://github.com/storj/up) provides a convenient way to configure and spin up a cluster locally for
active development. In addition to `storj-up`, you will need the following tools:

- TODO

## Version Control and Code Review

All of our source code is hosted on [GitHub](https://github.com/storj) and most of our reviews are done in
[Gerrit](https://review.dev.storj.tools). Reviews require 2 `Code Review +2` and a passing build in order to be merged.

## Continuous Integration

Currently, builds are performed by a [Jenkins](https://build.dev.storj.io) cluster hosted in Google Cloud. Because of
issues with how Google Cloud connects their disk to your virtual machine, we've disabled debug logging in our tests for
the time being. If a build fails and the failure message is insufficient for troubleshooting the test failure, rerunning
the test locally will output debug logs to aid in the troubleshooting.

# Practices

## Git Workflow

Storj uses a [Gerrit][gerrit-link] based git workflow. While pull requests can be made against the public GitHub
repository, many engineers at Storj prefer Gerrit for code reviews. For an overview of how this works, you can read
the [Intro to Gerrit Walkthrough][gerrit-walkthrough-link]. Be sure to [sign in][] to Gerrit before attempting to
contribute any changes. You'll likely need to upload an SSH key in order to push any changes.

[sign in]: https://review.dev.storj.tools
[gerrit-link]: https://review.dev.storj.tools/Documentation/index.html
[gerrit-walkthrough-link]: https://review.dev.storj.tools/Documentation/intro-gerrit-walkthrough-github.html

Below, you'll find several common workflows that I've used when contributing code.

### Cloning a project

When a project uses Gerrit, there is some additional work that needs to be done before you can start contributing.

1. Clone the repository.
   ```shell
   git clone git@github.com:storj/storj.git && cd storj
   ```
2. Setup Gerrit commit hook and aliases.
   ```shell
   curl -L storj.io/clone | sh
   ```

That should be it. At this point, the repository should be setup properly for Gerrit.

### Starting a new change set

When starting a new change, I find it useful to start a branch from the latest main (just like you would with any other
GitHub project). Unlike GitHub, we will not be pushing this branch remotely.

```shell
git checkout main
git pull
git checkout -b branch-name
```

This is where you'll work on and commit your code. When you commit your changes, your commit message should not only
describe what is changing, but why the change is being made. Storj uses the following convention for commit messages:

```
{scope}: {message}

{detail}

Change-Id: {changeID}
```

- `{scope}` refers to path within the repo that's changing. For example `satellite/metainfo` or `web/storagenode`.
  Multiple scopes _can_ be provided, but should be minimized.
- `{message}` should provide a clear and succinct message about what is changing. Using words like `add`, `remove`,
  `reuse`, or `refactor` are great here.
- `{detail}` provides additional information about the change. This is a good place to example why the change is being
  made. If you're making performance related changes, then this should also include a performance report comparing the
  before and after ([example][performance-example-link]).
- `{changeID}` refers to the automatically generated change id.

[performance-example-link]: https://github.com/storj/picobuf/commit/1d3412eb3ac13a476e56fa0e732552ed2ee89ecf

To produce this commit message, I find it easiest to start with the scope and message and then amend the commit to add
the detail. This allows the commit hook to automatically add the change id before filling in the longer detail of the
commit message. In addition to that, amending the commit provides some text guides that can be a useful reference when
determining where to wrap your detail text.

```shell
git commit -m "{scope}: {message}"
git commit --amend
```

Once a change has been commit, you can use `git push-wip` to push a work-in-progress change or `git push-review` to push
a new review. A work-in-progress change will allow you to push code without running tests or notifying the team of a new
review. All active reviews will get pulled into our review notification system and run our CI pipeline.

### Updating an existing change set

Updating an existing change set is a fairly common task. This can happen while iterating on a change or even updating it
with the latest set of changes from main. In the former case (updating while iterating on a change), you simply need to
amend your existing commit with your latest changes.

```shell
git add <updates>
git commit --amend --no-edit
```

The `--amend` flag tells git that you want the current change set to be added to the last git commit on your current
branch. The `--no-edit` flag instructs git to leave the commit message as is, and not to prompt the end user for
modifications to the message.

In the later case (when updating with the latest set of changes from main), you simply need to rebase your branch on the
updates from main.

```shell
git checkout main
git pull
git checkout branch-name
git rebase main
```

After any kind of change, you simply need to `push-wip` or `push-review` again.

### Managing relation chains

In Gerrit, multiple changes can be linked together for larger features. This is done by adding multiple commits to the
same branch. Consider the `oidc/implementation` branch below.

```
commit 533b6a2624f2a2eab45f0d68a6dd6e4fd1ec1124 (HEAD -> oidc/implementation)
Author: Mya <redacted>
Date:   Mon Feb 14 14:06:35 2022 -0600

    web/satellite: add consent screen for oauth
    
    When an application wants to interact with resources on behalf of
    an end-user, it needs to be granted access. In OAuth, this is done
    when a user submits the consent screen.
    
    Change-Id: Id838772f76999f63f5c9dbdda0995697b41c123a

commit ed9b75b11ecde87649621f6a02d127408828152e
Author: Mya <redacted>
Date:   Tue Feb 8 15:28:11 2022 -0600

    satellite/oidc: add integration test
    
    This change adds an integration test that performs an OAuth
    workflow and verifies the OIDC endpoints are functioning as
    expected.
    
    Change-Id: I18a8968b4f0385a1e4de6784dee68e1b51df86f7

commit 187aa654ff18387d0011b3741cca62270b95d6e6
Author: Mya <redacted>
Date:   Thu Feb 3 14:49:38 2022 -0600

    satellite/console: added oidc endpoints
    
    This change adds endpoints for supporting OpenID Connect (OIDC) and
    OAuth requests. This allows application developers to easily
    develop apps with Storj using common mechanisms for authentication
    and authorization.
    
    Change-Id: I2a76d48bd1241367aa2d1e3309f6f65d6d6ea4dc

```

These commits result in 3 distinct Gerrit changes, each linked together. If the last change in the link is merged, then
all other changes are merged along with it.

#### Editing or dropping a commit

In many cases, we need to go back and amend a previous change because of some feedback, a failing test, etc. If you're
editing an existing change or dropping and old one then you can use an interactive rebase to `edit` or `drop` a previous
commit. The command below will start an interactive rebase with the 3 most recent commits. (You should adjust this
number as needed.)

```shell
git rebase -i HEAD~3
```

This should bring up a screen where you can choose which commits you want to `edit` and which ones you want to `drop`.
The code block below shows how this screen looks for the associated changes above.

```
pick 187aa654f satellite/console: added oidc endpoints
pick ed9b75b11 satellite/oidc: add integration test
pick 533b6a262 web/satellite: add consent screen for oauth

# Rebase d4cf2013e..533b6a262 onto d4cf2013e (3 commands)
#
# Commands:
# p, pick <commit> = use commit
# r, reword <commit> = use commit, but edit the commit message
# e, edit <commit> = use commit, but stop for amending
# s, squash <commit> = use commit, but meld into previous commit
# f, fixup [-C | -c] <commit> = like "squash" but keep only the previous
#                    commit's log message, unless -C is used, in which case
#                    keep only this commit's message; -c is same as -C but
#                    opens the editor
# x, exec <command> = run command (the rest of the line) using shell
# b, break = stop here (continue rebase later with 'git rebase --continue')
# d, drop <commit> = remove commit
# l, label <label> = label current HEAD with a name
# t, reset <label> = reset HEAD to a label
# m, merge [-C <commit> | -c <commit>] <label> [# <oneline>]
# .       create a merge commit using the original merge commit's
# .       message (or the oneline, if no original merge commit was
# .       specified); use -c <commit> to reword the commit message
#
# These lines can be re-ordered; they are executed from top to bottom.
#
# If you remove a line here THAT COMMIT WILL BE LOST.
#
# However, if you remove everything, the rebase will be aborted.
#
```

#### Adding a new commit

To add a new commit to the relation chain, `edit` the commit just before where you want to add one, then make or unstash
your modifications, and commit as you would [above](#starting-a-new-change-set). Continuing the rebase will re-apply
your later changes on top of your newly created commit.

#### Splitting a commit

Splitting an existing change into multiple changes can be significantly more difficult. You need to consider not only
how a change needs to be split, but the order in which the commits must be applied. In this case, it's preferred that
one of the split commits preserve the existing change id as it's likely your change has already accumulated some
feedback that we want to preserve. When splitting a commit, I prefer to create a new branch before rebasing the change.

```shell
git checkout oidc/implementation
git checkout -b oidc/rework

git rebase -i HEAD~3

...
```

This way, if I make a mistake or need to start over, I can go back to a known working version by aborting the rebase,
going back to the old branch, and deleting the rework.

```shell
git rebase --abort
git checkout oidc/implementation
git branch -D oidc/rework

git branch -b oidc/rework

git rebase -i HEAD~3

...
```

## Linting locally

Our code linting process requires several customized and company specific tools. These are all packaged and provided as
part of our CI container. When running the lint target, we will spin up our CI container locally, and execute the
various linters from inside the container.

```sh
make lint
```

By default, the linter runs on the entire code base but can be limited to specific packages. It is worth noting that
some linters cannot run on specific packages and will therefore be unaffected by the provided packages.

```sh
make lint LINT_TARGET="./satellite/oidc/..."
```

## Executing tests locally

The Storj project has an extensive suite of integration tests. Many of these tests require several infrastructure
dependencies. These dependencies are defined and managed by the `docker-compose.tests.yaml` file. Tests can be executed
against Postgres (`test/postgres`), or CockroachDB (`test/cockroach`), or against both (`test`). By default, the full
suite of tests is run, but can be limited using the `TEST_TARGET` make variable.

```sh
# run only against Postgres
make test/postgres TEST_TARGET="./satellite/oidc/..."
```

You can also provide multiple targets for the test harness to run in case your change spans across several packages.

```sh
# run against both Postgres and Cockroach
make test TEST_TARGET="./satellite/oidc/... ./satellite/satellitedb/..."
```

_**Note:** While you can run the full suite of tests locally, you will likely be waiting around for them to complete.
By starting with the tests from packages you have modified, you can build a great deal of confidence in your changes
before pushing them up for review._

## Developing locally with `storj-up`

Following the instructions in the `storj-up` project `README`, the following will deploy a copy of the stack.

```shell
storj-up init
docker compose up -d
```

Outside the automation `storj-up` provides, there are a handful of manual changes that can be made to support testing
and developing against different portions of the stack. In the following sections, we demonstrate how this can be done
using the satellite process as an example, but the same process should work with many of our other processes.

### Testing backend changes

To test local backend changes, all you need to do is tell `storj-up` to mount a local binary for those containers.
Before mounting the local binary, you'll need to ensure your local binary is up-to-date.

```shell
# on Linux
go install ./cmd/satellite      
storj-up local-bin satellite-core satellite-admin satellite-api

# on OSX
GOOS=linux GOARCH=amd64 go install ./cmd/satellite
storj-up local-bin -s linux_amd64 satellite-core satellite-admin satellite-api
```

Once `storj-up` completes, you'll need to redeploy the containers to pick up the new configuration.

```shell
docker compose up -d
```

From here on out, all you'll need to do is recompile the binary and restart the container to pick up your latest
changes.

```shell
# on Linux
go install ./cmd/satellite
docker restart storj-satellite-core-1 storj-satellite-api-1 storj-satellite-admin-1

# on OSX
GOOS=linux GOARCH=amd64 go install ./cmd/satellite
docker restart storj-satellite-core-1 storj-satellite-api-1 storj-satellite-admin-1
```

_Aside_ - A few more notes here on OSX:

- Ensure your `GOPATH` is mountable by Docker. On OSX, only certain folders are mounted into the VM so you'll need to
  make sure it falls under one of those. Otherwise, `storj-up` will attempt to mount a directory under `GOBIN` that is
  unavailable on the VM.
- Even if you're on a Mac M1 (like me), you should target an `amd64` architecture. OSX provides some supporting tools
  that allows you to run `amd64` workloads on `arm64` processors.

### Testing frontend changes

In order to support mounting local web builds into the container, manual changes needed to be made to the
`satellite-api` volumes block of the `docker-compose.yaml` file. The following block adds a bind mount for the web
assets. Be sure to replace `${SOURCE_DIR}` with the path to the storj source repository (`pwd` on OSX and Linux).

```yaml
    - type: bind
      source: ${SOURCE_DIR}/web/satellite/dist/
      target: /var/lib/storj/storj/web/satellite/dist/
      bind:
        create_host_path: true
```

You’ll need to restart the container to pick up the new volume.

```shell
docker compose up -d
```

Now, you just need to rebuild the web assets, and they’ll automatically be picked up by the `satellite-api` deployment.

```shell
cd web/satellite

npm run build
```

#### Testing changes to static files (including wasm)

Any changes to the `static` directory (including wasm) will require an additional bind mount for the `satellite-api`.

```yaml
    - type: bind
      source: ${SOURCE_DIR}/web/satellite/static/
      target: /var/lib/storj/storj/web/satellite/static/
      bind:
        create_host_path: true
```

Similarly, you’ll need to restart the container to pick up the new volume.

```shell
docker compose up -d
```

Now, any changes to the `static` directory will automatically be picked up by the backend. If you’re iterating on the
wasm module, you can invoke the `wasm` or `wasm-dev` npm targets.

```shell
cd web/satellite

npm run wasm
```

# Specific Contribution Workflows

<!-- translations? -->

## Security patches

If you're submitting a patch related to a core vulnerability of the product, the review should be submit as a
work-in-progress, marked private, and the following users should be added.

- JT Olio
- Kaloyan Raev
- Egon Elibre
- Jeff Wendling
- Márton Elek
- Mya Pitzeruse

Together, we'll work through how to best roll the changes out to our network.

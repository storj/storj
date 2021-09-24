We are looking for ways to normalize and streamline the uplink CLI. This is currently focused around access grants, but additional restructuring might be worth considering.

### Things to do!

* Unify all access related commands under the access sub-command.
  * Move the `uplink revoke` command to `uplink access revoke`.
* `uplink access save`
  * Save given access into your config with a name.
  * This replaces `uplink import`
* `uplink access create`
  * Create a new access grant from either an existing grant or setup token.
  * Should allow also setting permission caveats (similar to how the `uplink share` does it).
* Create a new credential type: “setup token”
  * Satellite Address (Node ID + Domain)
  * Permission Caveats
* `uplink access list`
  * Remove the Node ID from the default listing (but have it in a verbose listing option)
  * Make the listing easier to read by aligning the columns
  * Also, display the default access grant in the list
  * By default list accesses by satellite (but allow alternate sorting/groupings)
* Rename `uplink setup` to `uplink configure`:
  * Near term: `uplink configure` is an alias for `uplink access create`
  * Long term: `uplink configure` will be a TUI for managing your config including (e.g. `rclone config`):
    * access grants and their names
    * metrics collection flags
    * debug ports
    * etc
* Prompting automatically if interactive and config doesn’t exist for acceptance of metrics collection.
* If a config doesn’t exist yet, and the user runs a non-interactive command, then we MUST automatically create a config with safe defaults.
* Update documentation to reflect all these changes.
  * rename “API Key” to “Setup Token”
  * ???
* A profile named `default` is special. All places which read in a profile name should treat this special and know not to create a non-special profile called `default` (e.g. `uplink access save --profile default` should not create an entry `accesses.default`, but instead should update the entry for `access`).
* The config will be relocated to more typical location `~/.config/storj/uplink`
* Access grants should automatically get a `not before` caveat added. This helps with revocation flows (where you can’t reshare the file unless the macaroon tail changes) and is also a good security practice.
* Make error messages displayed to users more informative and actionable. For
  example: https://github.com/storj/storj/issues/4018
  * The real problem is that they didn't reach 80 nodes not 35.
  * Repair is not relevant to the user on upload.
  * The upload failed because not enough nodes could be reached. The user needs
    to retry (and for better success changes split the file up) or use
    multipart (when it is available).

### Design Philosophy

* When possible follow a design pattern of:
  * Required parameters should be in positional arguments
  * Optional parameters should be flags
* Avoid putting sensitive information in flags or environment variables.
  * Promote interactively getting the information as the default option.
  * Allow using flags and environment variables as an advanced option.

### Configuration would be created

Any time a config would be created by running a command (e.g. `uplink access save`, `uplink access create`, `uplink configure`), then basic config options will be prompted (unless non-interactive flag/env is configured or detected):

```
$ uplink access create --token my-token
Allow anonymized metrics collection (y/n)? y
```

If the `--interactive=false`, then we would fall back to using the safe option of false.

TODO: Add examples for detected non-interactive.

### uplink access revoke

Move the `uplink revoke` command to `uplink access revoke`. There may be more normalization needed to have it align with the other access sub-commands.

### uplink access save

Be interactive if the information isn’t provided via flags/env:

```
$ uplink access save
Access Grant:
```

Without specific name, the access is saved as the default access grant.

```
$ uplink access save --access my-access-grant
```

```
$ uplink access save --access my-access-grant --name my-special-name
```

If an access with that name already exists the CLI should exit with an error unless `--force` is specified:

```
$ uplink access save --access my-access-grant
Error: Access grant \`default\` already exists (overwrite by specifying \`--force\`).
(Exit 1)
```

```
$ uplink access save --access my-access-grant --force
Warning: Access grant \`default\` overwritten.
(Exit 0)
```

```
$ uplink access save --access my-access-grant --name my-special-name
Error: Access grant \`my-access-grant\` already exists (overwrite by specifying \`--force\`).
(Exit 1)
```

```
$ uplink access save --access my-access-grant --name my-special-name --force
Warning: Access grant \`my-access-grant\` overwritten.
(Exit 0)
```

### uplink access create

#### Create from a Token

Token not provide (should prompt for it):

```
$ uplink access create
Enter setup token:
Enter encryption passphrase:
some-access-grant-just-created
Access grant \`default\` saved in ~/.config/uplink/uplink.conf
```

Passphrase not provided (should prompt for it):

```
$ uplink access create --token my-token
Enter encryption passphrase:
some-access-grant-just-created
Access grant \`default\` saved in ~/.config/uplink/uplink.conf
```

Setup token and passphrase provided:

```
$ uplink access create --token my-token --passphrase my-special-phrase
some-access-grant-just-created
Access grant \`default\` saved in ~/.config/uplink/uplink.conf
```

```
$ UPLINK\_TOKEN=my-token UPLINK\_PASSPHRASE=my-special-phrase uplink access create
some-access-grant-just-created
Access grant \`default\` saved in ~/.config/uplink/uplink.conf
```

Disable automatic saving of generated grant:

```
$ uplink access create --token my-token --passphrase my-special-phrase --save=false
some-access-grant-just-created
```

No existing config (prompt for basic config setup such as metrics collection):

```
$ uplink access create --token my-token
Allow anonymized metrics collection (y/n)? y
Enter encryption passphrase: my-special-phrase
some-access-grant-just-created
Access grant \`default\` saved in ~/.config/uplink/uplink.conf
```

Adding Permission Caveats:

```
$ uplink access create \\
  --token my-token \\
  --passphrase my-special-phrase \\
  --readonly \\
  --path sj://bucket/a
  --path sj://bucket/b
```

#### Create from an Access Grant

```
$ uplink access create --access my-access
Enter apassphrase
```


### uplink access list

Proposal A: group by satellite then alphabetically by name

```
$ uplink access list
europe-north-1.tardigrade.io:7777
- the-real-load-test
- strategy-caleb

europe-west-1.tardigrade.io:7777
- load-test

us-central-1.tardigrade.io:7777
- storj-general
- storj-general-backup
```

Proposal B: alphabetically by name then satellite

```
$ uplink access list
load-test               asia-east.tardigrade.io:7777
storj-caleb             europe-north-1.tardigrade.io:7777
storj-general           us-central-1.tardigrade.io:7777
storj-general-backup    us-central-1.tardigrade.io:7777
the-real-load-test      europe-north-1.tardigrade.io:7777
```

#### No Config Available

```
$ uplink ls
No default access grant configured. Please login at tardigrade.io and
create an access grant.
(Exit 1)
```

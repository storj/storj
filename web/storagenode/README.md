# Storagenode web dashboard

## Development

The common commands to use when developing or making changes to this sources are mapped in the
package.json (`scripts` field).

To run any of them execute `npm run <command>`.

The most used commands for development are:
- `lint`: To lint the sources and fix issues that are auto-fixable.
- `dev`: To run the web server and watch changes to auto refresh.
- `test`: To run the unit tests.


### Load local changes in storj-up

Sometimes the unit tests and the local development server isn't enough to see how the new changes
look like with data and satellite API calls.

To visualize them, you can use [storj-up](https://github.com/storj/up).

storj-up uses docker compose to run Storj network on your local machine. It uses published docker
images to run, so it won't be see you local changes without indicating it.

To make storj-up to see your local changes in the Storagenode dashboard, you have to modify the
`docker-compose.yaml` file that it generates.

Before you must make a clean installation, and build the frontend with `npm ci & npm run build`.

If you made changes in the storanode service (backend), you must build the new binary for Linux
amd64. On Linux machine with the same architecture, you only need to execute from the root of this
repository `go build -o /some/path/storagenode ./cmd/storagenode`, on an Intel MacOS/OSX it is
something like
`CC=x86_64-unknown-linux-gnu-gcc GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o /some/path/storagenode ./cmd/storagenode`

Then, you must apply the following changes to one of the storagenode services changing the
paths accordingly to your local machine:
- Add a new volume that maps to the frontend (`source` is the path to your local machine to this
  directory):
  ```yaml
    volumes:
      - type: bind
        source: /some/path/storj/web/storagenode/
        target: /var/lib/storj/web/storagenode/
        bind:
          create_host_path: true
  ```
- Add a this new environment variable
  `STORJ_CONSOLE_STATIC_DIR: /var/lib/storj/web/storagenode`
- If you modified the storagenode backend, also add this volume (`source` is the path to your local
  machine where the new compiled binary is):
  ```yaml
    volumes:
      - type: bind
        source: /some/path/storagenode
        target: /var/lib/storj/go/bin/storagenode
        bind:
          create_host_path: true
  ```

Remember to run stop and run the docker services again.

You can find your modified frontend on http://localhost:3000

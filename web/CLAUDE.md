# CLAUDE

This directory contains the frontend of the different Storj applications that have a web UI.

## Chrome Dev Tools

Use `chrome-devtools` when it's configured.

If you needed it and not configured, suggest the user to read
https://github.com/ChromeDevTools/chrome-devtools-mcp

When it's installed as a plugin chrome binary must be accessible, if it isn't, warn the user.

When it's configured to use a specific URL, ex:
```json
{
    "mscpServers": {
        "chrome-devtools": {
            "command": "npx",
            "args": [
                "-y",
                "chrome-devtools-mcp@latest",
                "--browserUrl=http://localhost:9222"
            ]
        }
    }
}
```
And you cannot access, remind the user to launch chrome in the configured debug port and passing
the `--user-data-dir` flag, ex:
`chrome --remote-debugging-port=9222 --user-data-dir=workspace/storj/chrome-debug/`

## NPM scripts

All the applications has the following common scripts

* `build`: Build forntend for production
* `lint`: Lint frontend code
* `test`: Run fontend tests

Run them with `npm <name>` when needed.

## Dependencies

Always use exact versions of dependencies in package.json, never use `^`, `~`, etc.

This is good
```json
{
  "dependencies": {
    "@mdi/font": "7.4.47",
    "chart.js": "4.5.1",
    "pinia": "3.0.4",
    "vue": "3.5.25",
    "vue-router": "4.6.3",
    "vuetify": "3.11.2"
  }
}
```

This is bad
```json
{
  "dependencies": {
    "@mdi/font": "7.x",
    "chart.js": ">=4.5.1",
    "pinia": ">3.0.4",
    "vue": "3.5.X",
    "vue-router": "~4.6.3",
    "vuetify": "^3.11.2"
  }
}
```

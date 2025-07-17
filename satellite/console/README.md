# Configuration

## Placement Configuration

This configures the state of placement related features on the satellite.

### Configuration Format

```yaml
STORJ_CONSOLE_PLACEMENT_SELF_SERVE_ENABLED: true/false
STORJ_CONSOLE_PLACEMENT_SELF_SERVE_DETAILS: |
  - id: 0
    id-name: "global"
    name: "Global"
    title: "Globally Distributed"
    description: "The data is globally distributed."
```

### STORJ_CONSOLE_PLACEMENT_SELF_SERVE_DETAILS
This is a list of placement definitions that can be used in Self-Serve placement selection.  This can be configured via YAML or JSON
string, or a YAML file. JSON is supported for backwards compatibility, but YAML is preferred.

```yaml
STORJ_CONSOLE_PLACEMENT_SELF_SERVE_DETAILS: |
  - id: 0
    id-name: "global"
    name: "Global"
    title: "Globally Distributed"
    description: "The data is globally distributed."
    wait-list-url: "url-to-wait-list"
```
**OR**

```yaml
STORJ_CONSOLE_PLACEMENT_SELF_SERVE_DETAILS: |
    [
        {
        "id": 0,
        "id-name": "global",
        "name": "Global",
        "title": "Globally Distributed",
        "description": "The data is globally distributed.",
        "wait-list-url": "url-to-wait-list"
        }
    ]
```

### Fields

- `id`: The numeric identifier for the placement. **This id must be present in the placement definitions of the satellite.**
- `id-name`: The internal name for the placement. **This must be present in the placement definitions of the satellite and correspond to the id.**
- `name`: Human-readable placement name
- `title`: Title for displaying placement, used in UI
- `description`: Description of the placement, used in UI
- `wait-list-url`: Optional URL for a wait list if the placement is not available yet.

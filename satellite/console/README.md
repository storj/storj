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
This is a list of placement definitions that can be used in Self-Serve placement selection.
This can be configured via YAML string or a YAML file.

```yaml
STORJ_CONSOLE_PLACEMENT_SELF_SERVE_DETAILS: |
  - id: 0
    id-name: "global"
    name: "Global"
    title: "Globally Distributed"
    description: "The data is globally distributed."
    wait-list-url: "url-to-wait-list"
```

### Fields

- `id`: The numeric identifier for the placement. **This id must be present in the placement definitions of the satellite.**
- `id-name`: The internal name for the placement. **This must be present in the placement definitions of the satellite and correspond to the id.**
- `name`: Human-readable placement name
- `title`: Title for displaying placement, used in UI
- `description`: Description of the placement, used in UI
- `wait-list-url`: Optional URL for a wait list if the placement is not available yet.

## Legacy Placement Product Mapping for Migration

### STORJ_CONSOLE_LEGACY_PLACEMENT_PRODUCT_MAPPING_FOR_MIGRATION

This configuration provides an override mapping from legacy placement IDs to product IDs,
used during project pricing migration from classic to new pricing tiers.

#### Purpose

When migrating "classic" projects to the new pricing model,
legacy placements need to be mapped to product IDs in the new billing system.
This config allows satellite operators to define custom mappings for specific legacy placements that may require different product assignments than the default behavior.

#### How It Works

During project pricing migration (via the `/projects/{id}/migrate-pricing` endpoint),
the system builds a composite mapping of placements to products in the following priority order:

1. Partner-specific mappings (if applicable)
2. Default placement-to-product mappings (from `STORJ_PAYMENTS_PLACEMENT_PRICE_OVERRIDES`)
3. **Override mappings from this config** (highest priority)

This config provides the final override layer,
ensuring that specific legacy placements can be mapped to the correct product IDs regardless of partner or default settings.

#### Configuration Format

The value is a JSON object mapping placement IDs (as strings) to product IDs (as integers):

```
STORJ_CONSOLE_LEGACY_PLACEMENT_PRODUCT_MAPPING_FOR_MIGRATION: '{"0":1,"12":2}'
```

In this example:
- Legacy placement `0` maps to product ID `1`
- Legacy placement `12` maps to product ID `2`

#### Related Configuration

- `STORJ_CONSOLE_LEGACY_PLACEMENTS`: Defines which placement IDs are considered "legacy"
- `STORJ_PAYMENTS_PLACEMENT_PRICE_OVERRIDES`: Default placement-to-product mappings for new placements
- `STORJ_CONSOLE_PLACEMENT_ALLOWED_PLACEMENT_IDS_FOR_NEW_PROJECTS`: Placements available after migration

## Partner UI Configuration (STORJ_CONSOLE_PARTNER_UI_CONFIG)
This section refers to partner-specific UI configurations for the Satellite UI.
The UI customizations refer to configs that were previously stored in the [tardigrade satellite theme](https://github.com/storj/tardigrade-satellite-theme) repository.
Depending on which satellite's folder in the repo is being viewed, there are files for different sections of the UI to be customized. All the files will look like;

```json
{
  "<partner>": {
    "some-field": "some-value",
    ...
  }
}
```
- `billingConfig.json`: Configurations related to billing UI.
- `onboardingConfig.json`: Configurations related to onboarding UI.
- `pricingPlanConfig.json`: Configurations related to pricing plan UI.
- `registrationViewConfig.json`: Configurations related to signup UI.
- `upgradeConfig.json`: Configurations related to upgrade UI.

The config `STORJ_CONSOLE_PARTNER_UI_CONFIG` should contain a YAML string or a YAML file path. The YAML maps partner identifiers to their UI configuration sections. For example;

```yaml
partner1:
  billing:
    some-field: "some-value"
    # ... additional billing config
  onboarding:
    some-field: "some-value"
    # ... additional onboarding config
  pricing-plan:
    some-field: "some-value"
    # ... additional pricing plan config
  signup:
    some-field: "some-value"
    # ... additional signup config
  upgrade:
    some-field: "some-value"
    # ... additional upgrade config
partner2:
  billing:
    some-field: "some-other-value"
  # ... additional partner2 config
```

Alternatively, you can provide a path to a YAML file:

```
STORJ_CONSOLE_PARTNER_UI_CONFIG: /path/to/partner-ui-config.yaml
```

### Structure
The configuration is a mapping of partner identifiers (strings) to UIConfig objects. Each UIConfig contains the following optional keys all of which are maps of string keys to any values:
- `billing`: Billing UI configurations.
- `onboarding`: Onboarding UI configurations.
- `pricing-plan`: Pricing plan UI configurations.
- `signup`: Signup UI configurations.
- `upgrade`: Upgrade UI configurations.
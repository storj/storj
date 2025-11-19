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

## White Label Configuration (STORJ_CONSOLE_WHITE_LABEL)

This section configures tenant-specific branding for the Satellite console UI.
The white label configuration allows different tenants to have custom branding when accessing the console through their specific hostnames.

### Configuration Format

The white label config should be provided as a YAML string or a YAML file path containing a mapping of tenant IDs to their branding configurations:

```yaml
STORJ_CONSOLE_WHITE_LABEL: |
  customer1:
    host-name: "customer1.example.com"
    name: "Customer One"
    logo-urls:
      full-dark: "https://customer1.example.com/logo-full-dark.png"
      full-light: "https://customer1.example.com/logo-full-light.png"
      small-dark: "https://customer1.example.com/logo-small-dark.png"
      small-light: "https://customer1.example.com/logo-small-light.png"
    favicon-urls:
      16x16: "https://customer1.example.com/favicon-16x16.ico"
      32x32: "https://customer1.example.com/favicon-32x32.ico"
      apple-touch: "https://customer1.example.com/apple-touch-icon.png"
    colors:
      primary-light: "#FF0000"
      primary-dark: "#FF0000"
      secondary-light: "#00FF00"
      secondary-dark: "#00FF00"
    support-url: "https://support.customer1.example.com"
    docs-url: "https://docs.customer1.example.com"
    homepage-url: "https://customer1.example.com"
    get-in-touch-url: "https://customer1.example.com/contact"
  customer2:
    host-name: "customer2.example.com"
    name: "Customer Two"
    # ... additional customer2 config
```

Alternatively, you can provide a path to a YAML file:

```
STORJ_CONSOLE_WHITE_LABEL: /path/to/white-label-config.yaml
```

### Fields

Each tenant configuration supports the following fields:

- `host-name` (required): The hostname that will trigger this white label configuration
- `name`: The display name for the tenant (e.g., "Customer One")
- `logo-urls`: Map of logo URLs with keys:
  - `full-dark`: Full logo for dark theme
  - `full-light`: Full logo for light theme
  - `small-dark`: Small logo for dark theme
  - `small-light`: Small logo for light theme
- `favicon-urls`: Map of favicon URLs with keys:
  - `16x16`: 16x16 pixel favicon
  - `32x32`: 32x32 pixel favicon
  - `apple-touch`: Apple touch icon
- `colors`: Map of custom colors (e.g., `primary`, `secondary`)
- `support-url`: Custom support/help URL
- `docs-url`: Custom documentation URL
- `homepage-url`: Custom homepage URL
- `get-in-touch-url`: Custom contact/get-in-touch URL

### API Endpoint

The branding configuration is exposed via the `/api/v0/config/branding` endpoint, which:
- Returns tenant-specific branding based on the `Host` header
- Does not require authentication
- Returns default Storj branding if no tenant context is found
- Includes `Cache-Control: public, max-age=3600` header for caching
- Returns HTTP 404 if a tenant ID is identified but no configuration exists

### How It Works

1. The tenant context is determined from the request's `Host` header
2. If a matching hostname is found in the white label configuration, the corresponding branding is returned
3. If no tenant context exists or hostname doesn't match, default Storj branding is returned
4. Frontend applications can fetch this configuration at startup to apply custom branding dynamically

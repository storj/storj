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

## Single White Label Configuration

This section configures custom branding for dedicated satellite deployments.
The single white label configuration allows a satellite to use custom branding
instead of the default Storj branding.

### Configuration Format

The single white label config is configured directly in YAML without CLI flag support.
It should be nested under the `console` section of the satellite configuration:

```yaml
console:
  single-white-label:
    tenant-id: "my-brand"
    name: "My Brand"
    external-address: "https://console.example.test"
    logo-urls:
      full-dark: "/static/static/images/whitelabel/mybrand/logo-full-dark.svg"
      full-light: "/static/static/images/whitelabel/mybrand/logo-full-light.svg"
      small-dark: "/static/static/images/whitelabel/mybrand/logo-small-dark.svg"
      small-light: "/static/static/images/whitelabel/mybrand/logo-small-light.svg"
      mail: "https://console.example.test/static/static/images/whitelabel/mybrand/logo-mail.png"
    favicon-urls:
      16x16: "/static/static/images/whitelabel/mybrand/favicon-16x16.png"
      32x32: "/static/static/images/whitelabel/mybrand/favicon-32x32.png"
      apple-touch: "/static/static/images/whitelabel/mybrand/apple-touch-icon.png"
    colors:
      primary: "#FF0000"
      primary-light: "#FF0000"
      primary-dark: "#FF0000"
      on-primary-light: "#FFFFFF"
      on-primary-dark: "#FFFFFF"
      secondary-light: "#00FF00"
      secondary-dark: "#00FF00"
      on-secondary-light: "#000000"
      on-secondary-dark: "#000000"
      background-light: "#F0F0F0"
      background-dark: "#202020"
      surface-light: "#FFFFFF"
      surface-dark: "#303030"
      on-surface-light: "#000000"
      on-surface-dark: "#FFFFFF"
      info-light: "#2196F3"
      info-dark: "#90CAF9"
      success-light: "#4CAF50"
      success-dark: "#81C784"
      warning-light: "#FFC107"
      warning-dark: "#FFD54F"
    support-url: "https://support.example.test"
    docs-url: "https://docs.example.test"
    homepage-url: "https://example.test"
    get-in-touch-url: "https://example.test/contact"
    source-code-url: "https://code.example.test"
    social-url: "https://social.example.test"
    blog-url: "https://blog.example.test"
    privacy-policy-url: "https://example.test/privacy"
    terms-of-service-url: "https://example.test/tos"
    terms-of-use-url: "https://example.test/terms"
    gateway-url: "https://gateway.example.test"
    company-name: "My Brand Inc."
    address-line1: "123 Example Street"
    address-line2: "Suite 456, City, ST 12345"
    smtp:
      server-address: "smtp.example.test:587"
      from: "noreply@example.test"
      auth-type: "plain"
      login: "smtp-user"
      password-env: "SMTP_PASSWORD"
```

### Fields

The single white label configuration supports the following fields:

- `tenant-id` (required): Unique identifier for the tenant. All users created on this satellite will be associated with this tenant ID.
- `name` (required): The display name for the brand (e.g., "My Brand"). **Enables white label mode when set.**
- `external-address`: The full external URL (e.g., "https://console.example.test"). Used to construct links in emails.
- `logo-urls`: Map of logo URLs with keys:
  - `full-dark`: Full logo for dark theme
  - `full-light`: Full logo for light theme
  - `small-dark`: Small logo for dark theme
  - `small-light`: Small logo for light theme
  - `mail`: Logo to be used in emails. **Required for branded emails.**
- `favicon-urls`: Map of favicon URLs with keys:
  - `16x16`: 16x16 pixel favicon
  - `32x32`: 32x32 pixel favicon
  - `apple-touch`: Apple touch icon
- `colors`: Map of custom colors. **The primary color is required for theming emails**.
- `support-url`: Custom support/help URL
- `docs-url`: Custom documentation URL
- `homepage-url`: Custom homepage URL. **Required for branded emails.**
- `get-in-touch-url`: Custom contact/get-in-touch URL
- `source-code-url`: URL to source code repository
- `social-url`: URL to social media page
- `blog-url`: URL to blog
- `privacy-policy-url`: URL to privacy policy page
- `terms-of-service-url`: URL to terms of service page
- `terms-of-use-url`: URL to terms of use page
- `gateway-url`: URL to the white-labeled gateway
- `company-name`: Legal company name. **Required for branded emails.**
- `address-line1`: First line of company address. **Required for branded emails.**
- `address-line2`: Second line of company address. **Required for branded emails.**
- `smtp`: SMTP configuration for custom email sending:
  - `auth-type`: Authentication type (e.g., "plain", "login", "simulated")
  - `server-address`: SMTP server address (e.g., "smtp.example.com:587")
  - `from`: Email address to use as sender
  - `login`: SMTP login username
  - `password-env`: The environment variable that contains SMTP password

### API Endpoint

The branding configuration is exposed via the `/api/v0/config/branding` endpoint, which:
- Returns the single white label branding when enabled (name is set)
- Returns default Storj branding when single white label is not enabled
- Does not require authentication
- Includes `Cache-Control: public, max-age=3600` header for caching

### How It Works

1. When `name` is set in the single white label configuration, white label mode is enabled
2. All branding endpoints return the configured custom branding
3. All users created on the satellite are associated with the configured `tenant-id`
4. Emails sent to users use the custom branding if configured
5. Frontend applications fetch branding at startup to apply custom branding dynamically

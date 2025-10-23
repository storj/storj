# Product Pricing Configuration

This document describes the configuration of product pricing and possible overrides.

## Products

This configures the pricing structures for different products in the Storj network.
It is a YAML formatted string that defines various products with their respective pricing details.
It can also be a YAML file path.

### Configuration Format

```yaml
STORJ_PAYMENTS_PRODUCTS: |
  - id: product_id
    name: "Product Name"
    price-summary: "price summary"
    storage: "price_per_TB_per_month"
    storage-sku: "storage_sku_value"
    egress: "price_per_TB"
    egress-sku: "egress_sku_value"
    included-egress-sku: "included_egress_sku_value"
    egress-discount-ratio: ratio_value
    egress-overage-mode: boolean_value
    segment: "price_per_segment_per_month"
    segment-sku: "segment_sku_value"
    small-object-fee: "fee_per_TB"
    small-object-fee-sku: "small_object_sku_value"
    minimum-retention-fee: "fee_per_TB_per_month"
    minimum-retention-fee-sku: "minimum_retention_sku_value"
    use-gb-units: boolean_value
    storage-remainder: "memory.Size"
```

### Fields

- `name`: Human-readable product name
- `price-summary`: Summary of the cost of storage and egress for the product. This will be shown on the UI's upgrade and account setup dialogs.
- `storage`: Price for storage per month in dollars/TB
- `storage-sku`: SKU for storage
- `egress`: Price for egress in dollars/TB
- `egress-sku`: SKU for egress
- `included-egress-sku`: SKU for included egress
- `egress-overage-mode`: Boolean indicating if egress overage mode is enabled. In overage mode, a product's egress will
be communicated as "free egress, with some overage fees beyond a certain point".
- `egress-discount-ratio`: Ratio of free egress per unit-month of storage. This can be omitted if it is not relevant to the product.
In that case, it will default to 0 (no free egress).
- `segment`: Price for segments stored on network per month in dollars/segment. This can be omitted if not relevant to the product.
In that case, it will default to 0 (no segment charges).
- `segment-sku`: SKU for segment storage
- `small-object-fee`: Fee for small objects per TB
- `small-object-fee-sku`: SKU for small object fee
- `minimum-retention-fee`: Minimum retention fee per TB per month
- `minimum-retention-fee-sku`: SKU for minimum retention fee
- `use-gb-units`: Boolean flag to use GB units instead of MB units on invoices (true for GB, false for MB)
- `storage-remainder`: Remainder storage in `memory.Size` parsable format (e.g., "50KB", "1MB")

### Example Configuration

```yaml
STORJ_PAYMENTS_PRODUCTS: |
  - id: 1
    name: "Basic Plan"
    storage: "4"
    storage-sku: "storage_sku_value_1"
    egress: "7"
    egress-sku: "egress_sku_value_1"
    segment: "0.0000088"
    segment-sku: "segment_sku_value_1"
    egress-discount-ratio: 0.4
  2:
    name: "Premium Plan"
    storage: "4"
    storage-sku: "storage_sku_value_2"
    egress: "8"
    egress-sku: "egress_sku_value_2"
    segment: "0.0000077"
    segment-sku: "segment_sku_value_2"
    egress-discount-ratio: 0.1
  3:
    name: "Enterprise Plan"
    storage: "3"
    storage-sku: "storage_sku_value_3"
    egress: "5"
    egress-sku: "egress_sku_value_3"
    segment: "0.0000066"
    segment-sku: "segment_sku_value_3"
    egress-discount-ratio: 0.0
  4:
    name: "Global Collaboration"
    price-summary: "15$/TB Storage, 20$/TB Egress"
    storage: 15
    storage-sku: "storage_sku_value_4"
    egress: 20
    egress-sku: "egress_sku_value_4"
    egress-discount-ratio: 1
    included-egress-sku: "included_egress_sku_value_4"
    small-object-fee: 15
    small-object-fee-sku: "small_object_sku_value_4"
    minimum-retention-fee: 15
    minimum-retention-fee-sku: "minimum_retention_sku_value_4"
    egress-overage-mode: true
    use-gb-units: true
    storage-remainder: "50KB"
```

## PlacementPriceOverrides

This maps specific placements to product IDs, allowing different pricing for different placements.
Expected configuration is a YAML or JSON string, or a YAML file. JSON support is for backwards compatibility, but YAML is preferred.

### Configuration Format

```yaml
STORJ_PAYMENTS_PLACEMENT_PRICE_OVERRIDES: |
  productID1:
    - placement1
    - placement2
  productID2:
    - placement3
```
**OR**
```json
{
    "productID1": [ 1, 2 ],
    "productID2": [ 3 ]
}
```
If a YAML file is used, it should be in the format:

```yaml
product-placements:
  1:
    - 0
```
`product-placements` are the same as `STORJ_PAYMENTS_PLACEMENT_PRICE_OVERRIDES`.

### Example Configuration

```yaml
STORJ_PAYMENTS_PLACEMENT_PRICE_OVERRIDES: |
  1:
    - 11  # US 1
    - 12  # US 2
  2:
    - 21  # EU 1
    - 22  # EU 2
  3:
    - 31  # AP 1
    - 32  # AP 2
```

### Usage Notes

- Products must be defined in the `Products` configuration first
- Placements must correspond to valid placement constraints in the network
- If a placement is not explicitly mapped, it will use default pricing

## PartnersPlacementPriceOverrides

This configures partner-specific placement-to-product mappings, allowing different pricing structures for different partners.
Expected configuration is a YAML or JSON string, or a YAML file. JSON support is for backwards compatibility, but YAML is preferred.

### Configuration Format

```yaml
STORJ_PAYMENTS_PARTNERS_PLACEMENT_PRICE_OVERRIDES: |
  partner1:
    productID1:
      - placement1
      - placement2
    productID2:
      - placement3
  partner2:
    productID1:
      - placement4
```
**OR**
```json
{
    "partner1": {
        "productID1": [ 1, 2 ],
        "productID2": [ 3 ]
    },
    "partner2": {
        "productID1": [ 4 ]
    }
}
```

If a YAML file is used, it should be in the format:

```yaml
partner-product-placements:
  somepartner:
    1:
      - 3
    3:
      - 1
```
`partner-product-placements` are the same as `STORJ_PAYMENTS_PARTNERS_PLACEMENT_PRICE_OVERRIDES`.

Both `partner-product-placements` and `product-placements` can be defined in the same file as such:
```yaml
product-placements:
  1:
    - 0
partner-product-placements:
  somepartner:
    1:
      - 3
    3:
      - 1
```

### Example Configuration

```yaml
STORJ_PAYMENTS_PARTNERS_PLACEMENT_PRICE_OVERRIDES: |
  enterprise-partner:
    3:
      - 21  # EU 1 with enterprise pricing
  startup-partner:
    2:
      - 11  # US 1 with startup pricing
      - 22  # EU 2 with startup pricing
  research-partner:
    1:
      - 31  # AP 1 with basic pricing
```

### Usage Notes

- If a partner uses a placement not defined in their specific configuration, they fall back to the general `PlacementPriceOverrides`
- Products referenced must exist in the `Products` configuration

## Configuration Dependencies

1. **Products must be defined first**: Both placement override configurations depend on products being defined in the `Products` field
2. **Valid placement IDs**: All placement IDs must exist in the satellite's placement definitions
3. **Non-zero product IDs**: Product IDs must be greater than 0
4. **YAML format**: All configurations use YAML format strings

## Example Complete Configuration

```yaml
STORJ_PAYMENTS_PRODUCTS: |
  1:
    name: "Standard"
    storage: "4"
    egress: "7"
    segment: "0.0000088"
    egress-discount-ratio: 0.1
  2:
    name: "Professional"
    storage: "3"
    egress: "6"
    segment: "0.0000077"
    egress-discount-ratio: 0.0

STORJ_PAYMENTS_PLACEMENT_PRICE_OVERRIDES: |
  1:
    - 11  # US 1
    - 22  # EU 2
  2:
    - 21  # EU 1
    - 12  # EU 2

STORJ_PAYMENTS_PARTNERS_PLACEMENT_PRICE_OVERRIDES: |
  enterprise-client:
    2:
      - 11  # Enterprise gets professional pricing in US1
  startup-client:
    1:
      - 21  # Startup gets standard pricing EU 1
```

This configuration:
- Defines two products
- Maps US 1 and EU 1 to standard pricing and US 2 and EU 2 to professional pricing by default
- Give enterprise partners professional pricing US 1, other placements use default pricing in `STORJ_PAYMENTS_PRODUCTS`
- Give startup partners standard pricing in EU 1, other placements use default pricing in `STORJ_PAYMENTS_PRODUCTS`
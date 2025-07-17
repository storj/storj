# Product Pricing Configuration

This document describes the configuration of product pricing and possible overrides.

## Products

This configures the pricing structures for different products in the Storj network. It is a JSON/YAML formatted string that defines various products with their respective pricing details.
It can also be a YAML file path.

**NB:** JSON support is only for backwards compatibility with current config in production. New configurations should use YAML format.

### Configuration Format

```yaml
STORJ_PAYMENTS_PRODUCTS: |
  - id: product_id
    name: "Product Name"
    storage: "price_per_TB_per_month"
    storage-sku: "storage_sku_value"
    egress: "price_per_TB"
    egress-sku: "egress_sku_value"
    segment: "price_per_segment_per_month"
    segment-sku: "segment_sku_value"
    egress-discount-ratio: ratio_value
```
**OR**
```json
{
    "productID": {
        "name": "Product Name",
        "storage": "price_per_TB_per_month",
        "storage_sku": "storage_sku_value",
        "egress": "price_per_TB",
        "egress_sku": "egress_sku_value",
        "segment": "price_per_segment_per_month",
        "segment_sku": "segment_sku_value",
        "egress_discount_ratio": "ratio_value"
    }
}
```

### Fields

- `name`: Human-readable product name
- `storage`: Price for storage per month in dollars/TB
- `storage-sku`: SKU for storage
- `egress`: Price for egress in dollars/TB
- `egress-sku`: SKU for egress
- `segment`: Price for segments stored on network per month in dollars/segment
- `segment-sku`: SKU for segment storage
- `egress-discount-ratio`: Ratio of free egress per unit-month of storage

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
```
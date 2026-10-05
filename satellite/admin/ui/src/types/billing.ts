// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Stripe collection methods an account's invoices can be created with. An empty value
 * leaves Stripe's default of charging the customer's saved payment method.
 */
export enum CollectionMethod {
    Default = '',
    ChargeAutomatically = 'charge_automatically',
    SendInvoice = 'send_invoice',
}

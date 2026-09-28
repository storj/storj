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

/**
 * Common Stripe payment method types offered as suggestions. Stripe accepts more,
 * so any other type can still be typed in.
 */
export const suggestedPaymentMethods = [
    'card',
    'customer_balance',
    'us_bank_account',
    'sepa_debit',
];

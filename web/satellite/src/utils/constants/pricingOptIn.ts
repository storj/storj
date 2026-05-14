// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

export enum PricingOptInVariant {
    GlobalArchiveOnly = 'global-archive-only',
    GlobalArchiveAndRegional = 'global-archive-and-regional',
}

export interface PricingOptInCard {
    label: string;
    planName: string;
    features: string[];
}

export const GLOBAL_ARCHIVE_CARD: PricingOptInCard = {
    label: 'Global & Archive',
    planName: 'Standard',
    features: [
        'Storage: $7/TB per month',
        'Egress: $7/TB',
        'Storage locations: Global distribution',
        'Object Mount included 2 seats free',
    ],
};

export const REGIONAL_CARD: PricingOptInCard = {
    label: 'Regional',
    planName: 'Advanced',
    features: [
        'Storage: $10/TB per month',
        'Egress: $7/TB',
        'Storage locations: U.S. SOC2 Type 2 data centers',
        'Object Mount included 2 seats free',
    ],
};

// Hardcoded variant for the current deployment. Edit this constant to switch which layout
// ships. The resolver function below is the single seam where future logic — which may need
// to combine values from multiple API calls or stores — will live.
export const PRICING_OPT_IN_VARIANT_DEFAULT: PricingOptInVariant = PricingOptInVariant.GlobalArchiveOnly;

export function resolvePricingOptInVariant(): PricingOptInVariant {
    return PRICING_OPT_IN_VARIANT_DEFAULT; // Returns the one price tier
    // return PricingOptInVariant.GlobalArchiveAndRegional // Returns the two price tiers
}

export function cardsForVariant(variant: PricingOptInVariant): PricingOptInCard[] {
    return variant === PricingOptInVariant.GlobalArchiveAndRegional
        ? [GLOBAL_ARCHIVE_CARD, REGIONAL_CARD]
        : [GLOBAL_ARCHIVE_CARD];
}

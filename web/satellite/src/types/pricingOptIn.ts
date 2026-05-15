// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { useConfigStore } from '@/store/modules/configStore';

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

// The pricing opt-in variant is determined by the satellite the frontend is talking to.
// When the satellite name reported by /api/v0/config contains "us1" (case-insensitive), we
// show both the Global & Archive and the Regional cards. Every other satellite shows only
// Global & Archive.
export function resolvePricingOptInVariant(): PricingOptInVariant {
    const name = useConfigStore().state.config.satelliteName ?? '';
    if (name.toLowerCase().includes('us1')) {
        return PricingOptInVariant.GlobalArchiveAndRegional;
    }
    return PricingOptInVariant.GlobalArchiveOnly;
}

export function cardsForVariant(variant: PricingOptInVariant): PricingOptInCard[] {
    return variant === PricingOptInVariant.GlobalArchiveAndRegional
        ? [GLOBAL_ARCHIVE_CARD, REGIONAL_CARD]
        : [GLOBAL_ARCHIVE_CARD];
}

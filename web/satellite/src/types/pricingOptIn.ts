// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

import { useConfigStore } from '@/store/modules/configStore';
import { Time } from '@/utils/time';

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
        'Minimum monthly invoice: $5',
        'Storage locations: Global distribution',
        'Object Mount: 2 licenses included',
    ],
};

export const REGIONAL_CARD: PricingOptInCard = {
    label: 'Regional',
    planName: 'Advanced',
    features: [
        'Storage: $10/TB per month',
        'Egress: $7/TB',
        'Minimum monthly invoice: $5',
        'Storage locations: U.S. SOC2 Type 2 data centers',
        'Object Mount: 2 licenses included',
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

export function formatConfigDate(raw: string | Date | null): string {
    if (!raw) {
        return '';
    }
    return Time.formattedDate(raw, { month: 'long', day: 'numeric', year: 'numeric', timeZone: 'UTC' });
}

export function parseConfigDate(raw: string): Date | null {
    if (!raw) {
        return null;
    }
    const date = new Date(raw);
    return isNaN(date.getTime()) ? null : date;
}

function shiftUTCDay(date: Date, days: number): Date {
    return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate() + days));
}

export function optOutDeadline(freezeDate: string): Date | null {
    const date = parseConfigDate(freezeDate);
    return date ? shiftUTCDay(date, -1) : null;
}

/**
 * Return when the opt-in dialog stops automatically popping up.
 */
export function popupAutoShowCutoff(freezeDate: string): Date | null {
    const date = parseConfigDate(freezeDate);
    return date ? shiftUTCDay(date, 1) : null;
}

export function freezeDateInFuture(freezeDate: string): boolean {
    const date = parseConfigDate(freezeDate);
    return !!date && new Date() < date;
}


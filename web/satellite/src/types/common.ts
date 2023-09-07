// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export enum SortDirection {
    ASCENDING = 1,
    DESCENDING,
    asc = 1,
    desc = 2,
}

export enum OnboardingOS {
    WINDOWS = 'windows',
    MAC = 'macos',
    LINUX = 'linux',
}

export class PricingPlanInfo {
    constructor(
        public type: PricingPlanType = PricingPlanType.FREE,
        // Info for the pricing plan container
        public title: string = '',
        public containerSubtitle: string = '',
        public containerDescription: string = '',
        public containerFooterHTML: string | null = null,
        public activationButtonText: string | null = null,
        // Info for the pricing plan modal (pre-activation)
        public activationSubtitle: string | null = null,
        public activationDescriptionHTML: string = '',
        public activationPriceHTML: string | null = null,
        // Info for the pricing plan modal (post-activation)
        public successSubtitle: string = '',
    ) {}
}

export enum PricingPlanType {
    FREE = 'free',
    PARTNER = 'partner',
    PRO = 'pro',
}

// TODO: fully implement these types and their methods according to their Go counterparts
export type UUID = string
export type MemorySize = string
export type Time = string

export function tableSizeOptions(itemCount: number, isObjectBrowser = false): {title: string, value: number}[] {
    const opts = [
        { title: '10', value: 10 },
        { title: '25', value: 25 },
        { title: '50', value: 50 },
        { title: '100', value: 100 },
    ];
    if (itemCount < 1000 && !isObjectBrowser) {
        return [{ title: 'All', value: itemCount }, ...opts];
    }
    return opts;
}

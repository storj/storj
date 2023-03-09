// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export enum SortDirection {
    ASCENDING = 1,
    DESCENDING,
}

export enum OnboardingOS {
    WINDOWS = 'windows',
    MAC = 'macos',
    LINUX = 'linux',
}

export class PartneredSatellite {
    constructor(
        public name: string = '',
        public address: string = '',
    ) {}
}

export class PricingPlanInfo {
    constructor(
        public type: PricingPlanType = PricingPlanType.FREE,
        // Info for the pricing plan container
        public title: string = '',
        public containerSubtitle: string = '',
        public containerDescription: string = '',
        public price: string | null = null,
        public oldPrice: string | null = null,
        // Info for the pricing plan modal (pre-activation)
        public activationSubtitle: string | null = null,
        public activationDescription: string = '',
        // Info for the pricing plan modal (post-activation)
        public successSubtitle: string = '',
    ) {}
}

export enum PricingPlanType {
    FREE = 'free',
    PARTNER = 'partner',
    PRO = 'pro',
}

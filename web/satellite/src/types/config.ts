// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { FrontendConfig } from '@/types/config.gen';
export * from '@/types/config.gen';

/**
 * Exposes functionality related to retrieving the frontend config.
 */
export interface FrontendConfigApi {
    /**
     * Returns the frontend config.
     *
     * @throws Error
     */
    get(): Promise<FrontendConfig>;

    /**
     * Returns branding config based on the tenant.
     *
     * @throws Error
     */
    getBranding(): Promise<BrandingConfig>;

    /**
     * Returns UI config of some kind for a partner.
     *
     * @param kind
     * @param partner
     */
    getPartnerUIConfig(kind: string, partner: string): Promise<unknown>;
}

export const defaultBrandingName = 'Storj';

/**
  * Creates a default Storj branding configuration.
  * This is used as a fallback when custom branding is not available.
  */
export function createDefaultBranding(): BrandingConfig {
    return new BrandingConfig(
        defaultBrandingName,
        new Map([
            [LogoKey.FullLight, '/static/static/images/logo.svg'],
            [LogoKey.FullDark, '/static/static/images/logo-dark.svg'],
        ]),
        new Map(), // No custom favicons, use defaults from HTML.
        new Map([
            [ColorKey.PrimaryLight, '#0052FF'],
            [ColorKey.PrimaryDark, '#0052FF'],
            [ColorKey.SecondaryLight, '#091C45'],
            [ColorKey.SecondaryDark, '#537CFF'],
        ]),
    );
}

export class BrandingConfig {
    public constructor(
        public name: string = '',
        public logoUrls: Map<string, string> = new Map(),
        public faviconUrls: Map<string, string> = new Map(),
        public colors: Map<string, string> = new Map(),
        public supportUrl: string = '',
        public docsUrl: string = '',
        public homepageUrl: string = '',
        public getInTouchUrl: string = '',
    ) {}

    public getColor(key: ColorKey): string | undefined {
        return this.colors.get(key);
    }

    public getLogo(key: LogoKey): string | undefined {
        return this.logoUrls.get(key);
    }

    public getFavicon(key: FaviconKey): string | undefined {
        return this.faviconUrls.get(key);
    }
}

export enum LogoKey {
    FullLight = 'full-light',
    FullDark = 'full-dark',
    SmallLight = 'small-light',
    SmallDark = 'small-dark',
}

export enum FaviconKey {
    Small = '16x16',
    Large = '32x32',
    AppleTouch = 'apple-touch',
}

export enum ColorKey {
    PrimaryLight = 'primary-light',
    PrimaryDark = 'primary-dark',
    SecondaryLight = 'secondary-light',
    SecondaryDark = 'secondary-dark',
}

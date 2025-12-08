// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import { BrandingConfig, FrontendConfig, FrontendConfigApi } from '@/types/config';
import { APIError } from '@/utils/error';

/**
 * FrontendConfigHttpApi is an HTTP implementation of the frontend config API.
 */
export class FrontendConfigHttpApi implements FrontendConfigApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/config';

    /**
     * Returns the frontend config.
     *
     * @throws Error
     */
    public async get(): Promise<FrontendConfig> {
        const response = await this.http.get(this.ROOT_PATH);
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Cannot get frontend config',
                requestID: response.headers.get('x-request-id'),
            });
        }
        return await response.json() as FrontendConfig;
    }

    /**
     * Returns branding config based on the tenant.
     *
     * @throws Error
     */
    public async getBranding(): Promise<BrandingConfig> {
        const response = await this.http.get(`${this.ROOT_PATH}/branding`);
        const result = await response.json();

        if (response.ok) {
            return new BrandingConfig(
                result.name,
                result.logoUrls ? new Map(Object.entries(result.logoUrls)) : new Map(),
                result.faviconUrls ? new Map(Object.entries(result.faviconUrls)) : new Map(),
                result.colors ? new Map(Object.entries(result.colors)) : new Map(),
                result.supportUrl,
                result.docsUrl,
                result.homepageUrl,
                result.getInTouchUrl,
            );
        }

        throw new APIError({
            status: response.status,
            message: result.error || 'Cannot get branding config',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Returns UI config of some kind for a partner.
     *
     * @param kind
     * @param partner
     */
    public async getPartnerUIConfig(kind: string, partner: string): Promise<unknown> {
        const response = await this.http.get(`/api/v0/${kind}-config/${partner}`);
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Cannot get partner UI config',
                requestID: response.headers.get('x-request-id'),
            });
        }
        return await response.json();
    }
}

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import { FrontendConfig, FrontendConfigApi } from '@/types/config';
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
}

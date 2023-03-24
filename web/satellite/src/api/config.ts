// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import { FrontendConfig, FrontendConfigApi } from '@/types/config';

/**
 * FrontendConfigHttpApi is an HTTP implementation of the frontend config API.
 */
export class FrontendConfigHttpApi implements FrontendConfigApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/config';

    /**
     * Returns the frontend config.
     *
     * @throws Error
     */
    public async get(): Promise<FrontendConfig> {
        const response = await this.http.get(this.ROOT_PATH);
        if (!response.ok) {
            throw new Error('Cannot get frontend config');
        }
        return await response.json() as FrontendConfig;
    }
}

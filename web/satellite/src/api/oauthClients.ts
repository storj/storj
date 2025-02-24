// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';

export interface OAuthClient {
    id: string;
    redirectURL: string;
    appName: string;
    appLogoURL: string;
}

export class OAuthClientsAPI {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/oauth/v2/clients';

    public async get(id: string): Promise<OAuthClient> {
        const path = `${this.ROOT_PATH}/${id}`;
        const response = await this.http.get(path);
        const result = await response.json();

        if (response.ok) return result;

        let errMsg = result.error || 'Failed to lookup oauth client';
        switch (response.status) {
        case 400:
            errMsg = 'Invalid request for OAuth client';
            break;
        case 404:
            errMsg = 'OAuth client was not found';
            break;
        case 429:
            errMsg = 'API rate limit exceeded';
        }

        throw new APIError({
            status: response.status,
            message: errMsg,
            requestID: response.headers.get('x-request-id'),
        });
    }
}

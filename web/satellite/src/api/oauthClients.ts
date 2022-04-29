// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import {HttpClient} from "@/utils/httpClient";
import {ErrorTooManyRequests} from "@/api/errors/ErrorTooManyRequests";

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
        if (response.ok) {
            return response.json().then((body) => body as OAuthClient);
        }

        switch (response.status) {
        case 400:
            throw new Error('Invalid request for OAuth client');
        case 404:
            throw new Error('OAuth client was not found');
        case 429:
            throw new ErrorTooManyRequests('API rate limit exceeded');
        }

        throw new Error('Failed to lookup oauth client');
    }
}

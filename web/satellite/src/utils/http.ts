// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { AuthToken } from '@/utils/authToken';

/**
 * Http is a helper util to perform raw http requests.
 */
export class Http {
    private async sendRequest(method: string, path: string, body: any = null): Promise<any> {
        // get the authentication token from local storage if it exists
        const token = AuthToken.get();

        const response = await fetch(path, {
            method: method,
            body: body,
            headers: {
                authorization: token ? `Bearer ${token}` : '',
                'Content-Type': 'application/json',
            },
        });

        if (!response.ok) {
            const errorMessage = await response.text();
            throw new Error(errorMessage);
        }

        return response;
    }

    /**
     * Performs POST http request.
     * @param path
     * @param body
     */
    public async post(path: string, body: any = null): Promise<any> {
        return this.sendRequest('POST', path, body);
    }

    /**
     * Performs GET http request.
     * @param path
     */
    public async get(path: string): Promise<any> {
        return this.sendRequest('GET', path, null);
    }

    /**
     * Performs DELETE http request.
     * @param path
     * @param body
     */
    public async delete(path: string, body: any = null): Promise<any> {
        return this.sendRequest('DELETE', path, body);
    }
}

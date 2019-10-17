// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { AuthToken } from '@/utils/authToken';

/**
 * HttpClient is a custom wrapper around fetch api.
 * Exposes get, post and delete methods for JSON strings.
 */
export class HttpClient {
    /**
     *
     * @param method holds http method type
     * @param path
     * @param body serialized JSON
     * @param auth indicates if authentication is needed
     */
    private async sendJSON(method: string, path: string, body: string | null, auth: boolean): Promise<Response> {
        const request: RequestInit = {
            method: method,
            body: body,
        };

        const headers: Record<string, string> = {
            'Content-Type': 'application/json',
        };

        if (auth) {
            headers['Authorization'] = `Bearer ${AuthToken.get()}`;
        }

        request.headers = headers;

        return await fetch(path, request);
    }

    /**
     * Performs POST http request with JSON body.
     * @param path
     * @param body serialized JSON
     * @param auth indicates if authentication is needed
     */
    public async post(path: string, body: string | null, auth: boolean = true): Promise<Response> {
        return this.sendJSON('POST', path, body, auth);
    }

    /**
     * Performs PATCH http request with JSON body.
     * @param path
     * @param body serialized JSON
     * @param auth indicates if authentication is needed
     */
    public async patch(path: string, body: string | null, auth: boolean = true): Promise<Response> {
        return this.sendJSON('PATCH', path, body, auth);
    }

    /**
     * Performs GET http request.
     * @param path
     * @param auth indicates if authentication is needed
     */
    public async get(path: string, auth: boolean = true): Promise<Response> {
        return this.sendJSON('GET', path, null, auth);
    }

    /**
     * Performs DELETE http request.
     * @param path
     * @param auth indicates if authentication is needed
     */
    public async delete(path: string, auth: boolean = true): Promise<Response> {
        return this.sendJSON('DELETE', path, null, auth);
    }
}

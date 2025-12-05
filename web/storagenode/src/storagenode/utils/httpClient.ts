// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

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
     */
    private async sendJSON(method: string, path: string, body: string | null): Promise<Response> {
        const request: RequestInit = {
            method: method,
            body: body,
        };

        request.headers = {
            'Content-Type': 'application/json',
        };

        return await fetch(path, request);
    }

    /**
     * Performs POST http request with JSON body.
     * @param path
     * @param body serialized JSON
     */
    public async post(path: string, body: string | null): Promise<Response> {
        return this.sendJSON('POST', path, body);
    }

    /**
     * Performs PATCH http request with JSON body.
     * @param path
     * @param body serialized JSON
     */
    public async patch(path: string, body: string | null): Promise<Response> {
        return this.sendJSON('PATCH', path, body);
    }

    /**
     * Performs PUT http request with JSON body.
     * @param path
     * @param body serialized JSON
     * @param _auth indicates if authentication is needed
     */
    public async put(path: string, body: string | null, _auth = true): Promise<Response> {
        return this.sendJSON('PUT', path, body);
    }

    /**
     * Performs GET http request.
     * @param path
     * @param _auth indicates if authentication is needed
     */
    public async get(path: string, _auth = true): Promise<Response> {
        return this.sendJSON('GET', path, null);
    }

    /**
     * Performs DELETE http request.
     * @param path
     * @param _auth indicates if authentication is needed
     */
    public async delete(path: string, _auth = true): Promise<Response> {
        return this.sendJSON('DELETE', path, null);
    }
}

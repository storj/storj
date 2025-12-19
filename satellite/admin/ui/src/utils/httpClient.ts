// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';

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

        const response = await fetch(path, request);
        if (response.status === 401) {
            await this.handleUnauthorized();
            throw new ErrorUnauthorized();
        }

        return response;
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
     */
    public async put(path: string, body: string | null): Promise<Response> {
        return this.sendJSON('PUT', path, body);
    }

    /**
     * Performs GET http request.
     * @param path
     */
    public async get(path: string): Promise<Response> {
        return this.sendJSON('GET', path, null);
    }

    /**
     * Performs DELETE http request.
     * @param path
     * @param body serialized JSON
     */
    public async delete(path: string, body: string | null = null): Promise<Response> {
        return this.sendJSON('DELETE', path, body);
    }

    /**
     * Handles unauthorized actions.
     * Call logout and redirect to login.
     */
    private async handleUnauthorized(): Promise<void> {
        try {
            const logoutPath = '/api/v0/auth/logout';
            const request: RequestInit = {
                method: 'POST',
                body: null,
            };

            request.headers = {
                'Content-Type': 'application/json',
            };

            await fetch(logoutPath, request);
            // eslint-disable-next-line no-empty,@typescript-eslint/no-unused-vars
        } catch (_) {}

        setTimeout(() => {
            if (!window.location.href.includes('/login')) {
                window.location.href = window.location.origin + '/login';
            }
        }, 2000);
    }
}

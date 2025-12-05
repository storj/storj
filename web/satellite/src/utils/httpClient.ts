// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { ROUTES } from '@/router';

interface AdditionalHeaders {
    csrfProtectionToken?: string;
    authToken?: string;
}

/**
 * HttpClient is a custom wrapper around fetch api.
 * Exposes get, post and delete methods for JSON strings.
 */
export class HttpClient {
    private readonly csrfHeader: string = 'X-CSRF-Token';
    private readonly authHeader: string = 'Authorization';

    /**
     *
     * @param method holds http method type
     * @param path
     * @param body serialized JSON
     * @param additionalHeaders custom headers used for a request
     */
    private async sendJSON(method: string, path: string, body: string | null, additionalHeaders?: AdditionalHeaders): Promise<Response> {
        const request: RequestInit = {
            method: method,
            body: body,
            headers: {
                'Content-Type': 'application/json',
            },
        };

        if (additionalHeaders && request.headers) {
            if (additionalHeaders.csrfProtectionToken) {
                request.headers[this.csrfHeader] = additionalHeaders.csrfProtectionToken;
            }
            if (additionalHeaders.authToken) {
                request.headers[this.authHeader] = `Bearer ${additionalHeaders.authToken}`;
            }
        }

        const response = await fetch(path, request);
        if (response.status === 401) {
            await this.handleUnauthorized();
            throw new ErrorUnauthorized('Authorization required', response.headers.get('x-request-id'));
        }

        return response;
    }

    /**
     * Performs POST http request with JSON body.
     * @param path
     * @param body serialized JSON
     * @param additionalHeaders custom headers used for a request
     */
    public async post(path: string, body: string | null, additionalHeaders?: AdditionalHeaders): Promise<Response> {
        return this.sendJSON('POST', path, body, additionalHeaders);
    }

    /**
     * Performs PATCH http request with JSON body.
     * @param path
     * @param body serialized JSON
     * @param additionalHeaders custom headers used for a request
     */
    public async patch(path: string, body: string | null, additionalHeaders?: AdditionalHeaders): Promise<Response> {
        return this.sendJSON('PATCH', path, body, additionalHeaders);
    }

    /**
     * Performs PUT http request with JSON body.
     * @param path
     * @param body serialized JSON
     * @param additionalHeaders custom headers used for a request
     */
    public async put(path: string, body: string | null, additionalHeaders?: AdditionalHeaders): Promise<Response> {
        return this.sendJSON('PUT', path, body, additionalHeaders);
    }

    /**
     * Performs GET http request.
     * @param path
     * @param additionalHeaders custom headers used for a request
     */
    public async get(path: string, additionalHeaders?: AdditionalHeaders): Promise<Response> {
        return this.sendJSON('GET', path, null, additionalHeaders);
    }

    /**
     * Performs DELETE http request.
     * @param path
     * @param body serialized JSON
     * @param additionalHeaders custom headers used for a request
     */
    public async delete(path: string, body: string | null = null, additionalHeaders?: AdditionalHeaders): Promise<Response> {
        return this.sendJSON('DELETE', path, body, additionalHeaders);
    }

    /**
     * Handles unauthorized actions.
     * Call logout and redirect to login.
     */
    private async handleUnauthorized(): Promise<void> {
        let path = window.location.href;
        if (!this.isAuthRoute(path)) {
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
                // eslint-disable-next-line no-empty
            } catch { }

            setTimeout(() => {
                // path may have changed after timeout.
                path = window.location.href;
                if (this.isAuthRoute(path)) {
                    return;
                }
                window.location.href = `${window.location.origin}/login`;
            }, 2000);
        }
    }

    private isAuthRoute(path: string): boolean {
        return ROUTES.AuthRoutes.some((route) => path.includes(route));
    }
}

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
        // The API also responds with 401 when the user lacks permissions. Those
        // responses are returned so that the caller surfaces the error.
        if (response.status === 401 && await HttpClient.isSessionExpired()) {
            HttpClient.redirectToLogin();
            throw new ErrorUnauthorized('Session expired. Redirecting to login...');
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

    private static redirectingToLogin = false;

    // sessionStorage key of the time of the last reload for an expired session.
    private static readonly lastReloadKey = 'admin-session-expired-reload';
    // reloads closer together than this are skipped to avoid a reload loop.
    private static readonly minReloadIntervalMs = 30_000;

    /**
     * Checks whether the session is gone. The endpoint responds with 401 when
     * there is no valid OIDC session, and an auth proxy may redirect it to the
     * identity provider instead.
     */
    private static async isSessionExpired(): Promise<boolean> {
        try {
            const response = await fetch('/auth/current-user', { redirect: 'manual' });
            return response.status === 401 || response.type === 'opaqueredirect';
        } catch {
            return false;
        }
    }

    /**
     * Reloads the page, giving time for the error to be shown. Without a session,
     * the server (or the auth proxy) redirects page loads to its login.
     * It doesn't reload again shortly after a reload, in case the reloaded page
     * isn't redirected, so that the page doesn't keep reloading.
     */
    private static redirectToLogin(): void {
        if (HttpClient.redirectingToLogin) return;

        const now = Date.now();
        try {
            const lastReload = Number(sessionStorage.getItem(HttpClient.lastReloadKey) ?? 0);
            if (now - lastReload < HttpClient.minReloadIntervalMs) return;
            sessionStorage.setItem(HttpClient.lastReloadKey, String(now));
        } catch {
            // without storage the page is reloaded anyway.
        }

        HttpClient.redirectingToLogin = true;
        setTimeout(() => window.location.reload(), 2000);
    }
}

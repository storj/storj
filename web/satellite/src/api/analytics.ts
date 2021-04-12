// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';

/**
 * AnalyticsHttpApi is a console Analytics API.
 * Exposes all analytics-related functionality
 */
export class AnalyticsHttpApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/analytics';

    /**
     * Used to get authentication token.
     *
     * @param eventName - name of the event
     * @throws Error
     */
    public async eventTriggered(eventName: string): Promise<void> {
        const path = `${this.ROOT_PATH}/event`;
        const body = {
            eventName: eventName,
        };
        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            return;
        }

        throw new Error('Can not track event');
    }

    public async linkEventTriggered(eventName: string, link: string): Promise<void> {
        const path = `${this.ROOT_PATH}/event`;
        const body = {
            eventName: eventName,
            link: link,
        };
        const response = await this.http.post(path, JSON.stringify(body));
        if (response.ok) {
            return;
        }
        throw new Error('Can not track event');
    }
}

// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

/**
 * AnalyticsHttpApi is a console Analytics API.
 * Exposes all analytics-related functionality
 */
export class AnalyticsHttpApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/analytics';

    /**
     * Used to notify the satellite about arbitrary events that occur.
     * Does not throw any errors so that expected UI behavior is not interrupted if the API call fails.
     *
     * @param eventName - name of the event
     * @param props - additional properties to send with the event
     */
    public async eventTriggered(eventName: string, props?: { [p: string]: string }): Promise<void> {
        try {
            const path = `${this.ROOT_PATH}/event`;
            const body = {
                eventName: eventName,
            };
            if (props) {
                body['props'] = props;
            }
            const response = await this.http.post(path, JSON.stringify(body));
            if (response.ok) {
                return;
            }
            console.error('Attempted to notify Satellite that ' + eventName + ' occurred. Got bad response status code: ' + response.status);
        } catch (error) {
            console.error('Could not notify satellite about ' + eventName + ' event occurrence (most likely blocked by browser).');
        }
    }

    /**
     * Used to notify the satellite about arbitrary external link clicked events that occur.
     * Does not throw any errors so that expected UI behavior is not interrupted if the API call fails.
     *
     * @param eventName - name of the event
     * @param link - link that was clicked
     */
    public async linkEventTriggered(eventName: string, link: string): Promise<void> {
        try {
            const path = `${this.ROOT_PATH}/event`;
            const body = {
                eventName: eventName,
                link: link,
            };
            const response = await this.http.post(path, JSON.stringify(body));
            if (response.ok) {
                return;
            }
            console.error('Attempted to notify Satellite that ' + eventName + ' occurred. Got bad response status code: ' + response.status);
        } catch (error) {
            console.error('Could not notify satellite about ' + eventName + ' event occurrence (most likely blocked by browser).');
        }
    }

    /**
     * Used to notify the satellite about arbitrary page visits that occur.
     * Does not throw any errors so that expected UI behavior is not interrupted if the API call fails.
     *
     * @param pageName - name of the page
     */
    public async pageVisit(pageName: string): Promise<void> {
        try {
            const path = `${this.ROOT_PATH}/page`;
            const body = {
                pageName: pageName,
            };
            const response = await this.http.post(path, JSON.stringify(body));
            if (response.ok) {
                return;
            }
            console.error('Attempted to notify Satellite that ' + pageName + ' occurred. Got bad response status code: ' + response.status);
        } catch (error) {
            console.error('Could not notify satellite about ' + pageName + ' event occurrence (most likely blocked by browser).');
        }
    }

    public async pageView(body: { url: string; props: { source: string } }): Promise<void> {
        try {
            const path = `${this.ROOT_PATH}/pageview`;
            const response = await this.http.post(path, JSON.stringify(body));
            if (response.ok) {
                return;
            }
            console.error('Attempted to notify Satellite that pageview occurred. Got bad response status code: ' + response.status);
        } catch (error) {
            console.error('Could not notify satellite about pageview event occurrence (most likely blocked by browser).');
        }
    }

    /**
     * Used to notify the satellite about error events that occur.
     * Does not throw any errors so that expected UI behavior is not interrupted if the API call fails.
     *
     * @param source - place where event happened
     */
    public async errorEventTriggered(source: AnalyticsErrorEventSource): Promise<void> {
        try {
            const path = `${this.ROOT_PATH}/event`;
            const body = {
                eventName: AnalyticsEvent.UI_ERROR,
            };

            if (source) {
                body['errorEventSource'] = source;
            }

            const response = await this.http.post(path, JSON.stringify(body));
            if (response.ok) {
                return;
            }
            console.error(`Attempted to notify Satellite that UI error occurred here: ${source}. Got bad response status code: ${response.status}`);
        } catch (error) {
            console.error(`Could not notify satellite about UI error here: ${source} (most likely blocked by browser).`);
        }
    }
}

// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { HttpClient } from '@/utils/httpClient';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { APIError } from '@/utils/error';
import { JoinCunoFSBetaForm, ObjectMountConsultationForm, UserFeedbackForm } from '@/types/analytics';

/**
 * AnalyticsHttpApi is a console Analytics API.
 * Exposes all analytics-related functionality
 */
export class AnalyticsHttpApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/analytics';

    /**
     * Used to track user filled the form to join cunoFS beta.
     *
     * @param data - form data
     * @param csrfProtectionToken - CSRF token
     */
    public async joinCunoFSBeta(data: JoinCunoFSBetaForm, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/join-cunofs-beta`;

        const response = await this.http.post(path, JSON.stringify(data), { csrfProtectionToken });
        if (!response.ok) {
            const result = await response.json();

            throw new APIError({
                status: response.status,
                message: result.error,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to track user filled the form to join placement waitlist.
     *
     * @param storageNeeds - form data
     * @param placement - the placement the form is for
     * @param csrfProtectionToken - CSRF token
     */
    public async joinPlacementWaitlist(storageNeeds: string, placement: number, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/join-placement-waitlist`;

        const response = await this.http.post(path, JSON.stringify({ storageNeeds, placement }), { csrfProtectionToken });
        if (!response.ok) {
            const result = await response.json();

            throw new APIError({
                status: response.status,
                message: result.error,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to send user feedback.
     *
     * @param data - feedback data
     * @param csrfProtectionToken - CSRF token
     */
    public async sendUserFeedback(data: UserFeedbackForm, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/send-feedback`;

        const response = await this.http.post(path, JSON.stringify(data), { csrfProtectionToken });
        if (!response.ok) {
            const result = await response.json();

            throw new APIError({
                status: response.status,
                message: result.error,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to request a consultation for object mount.
     *
     * @param data - consultation request data
     * @param csrfProtectionToken - CSRF token
     */
    public async requestObjectMountConsultation(data: ObjectMountConsultationForm, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/object-mount-consultation`;

        const response = await this.http.post(path, JSON.stringify(data), { csrfProtectionToken });
        if (!response.ok) {
            const result = await response.json();

            throw new APIError({
                status: response.status,
                message: result.error,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to notify the satellite about arbitrary events that occur.
     * Throws an error if event hasn't been submitted.
     *
     * @param eventName - name of the event
     * @param csrfProtectionToken - CSRF token
     * @param props - additional properties to send with the event
     */
    public async ensureEventTriggered(eventName: string, csrfProtectionToken: string, props?: { [p: string]: string }): Promise<void> {
        const path = `${this.ROOT_PATH}/event`;

        const body = { eventName };
        if (props) body['props'] = props;

        const response = await this.http.post(path, JSON.stringify(body), { csrfProtectionToken });
        if (!response.ok) {
            const result = await response.json();

            throw new APIError({
                status: response.status,
                message: result.error,
                requestID: response.headers.get('x-request-id'),
            });
        }
    }

    /**
     * Used to notify the satellite about arbitrary events that occur.
     * Does not throw any errors so that expected UI behavior is not interrupted if the API call fails.
     *
     * @param eventName - name of the event
     * @param csrfProtectionToken - CSRF token
     * @param props - additional properties to send with the event
     */
    public async eventTriggered(eventName: string, csrfProtectionToken: string, props?: { [p: string]: string }): Promise<void> {
        try {
            const path = `${this.ROOT_PATH}/event`;
            const body = {
                eventName: eventName,
            };
            if (props) {
                body['props'] = props;
            }
            const response = await this.http.post(path, JSON.stringify(body), { csrfProtectionToken });
            if (response.ok) {
                return;
            }
            console.error('Attempted to notify Satellite that ' + eventName + ' occurred. Got bad response status code: ' + response.status);
        } catch {
            console.error('Could not notify satellite about ' + eventName + ' event occurrence (most likely blocked by browser).');
        }
    }

    /**
     * Used to notify the satellite about arbitrary external link clicked events that occur.
     * Does not throw any errors so that expected UI behavior is not interrupted if the API call fails.
     *
     * @param eventName - name of the event
     * @param link - link that was clicked
     * @param csrfProtectionToken - CSRF token
     */
    public async linkEventTriggered(eventName: string, link: string, csrfProtectionToken: string): Promise<void> {
        try {
            const path = `${this.ROOT_PATH}/event`;
            const body = {
                eventName: eventName,
                link: link,
            };
            const response = await this.http.post(path, JSON.stringify(body), { csrfProtectionToken });
            if (response.ok) {
                return;
            }
            console.error('Attempted to notify Satellite that ' + eventName + ' occurred. Got bad response status code: ' + response.status);
        } catch {
            console.error('Could not notify satellite about ' + eventName + ' event occurrence (most likely blocked by browser).');
        }
    }

    /**
     * Used to notify the satellite about arbitrary page visits that occur.
     * Does not throw any errors so that expected UI behavior is not interrupted if the API call fails.
     *
     * @param pageName - name of the page
     * @param csrfProtectionToken - CSRF token
     */
    public async pageVisit(pageName: string, csrfProtectionToken: string): Promise<void> {
        try {
            const path = `${this.ROOT_PATH}/page`;
            const body = {
                pageName: pageName,
            };
            const response = await this.http.post(path, JSON.stringify(body), { csrfProtectionToken });
            if (response.ok) {
                return;
            }
            console.error('Attempted to notify Satellite that ' + pageName + ' occurred. Got bad response status code: ' + response.status);
        } catch {
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
        } catch {
            console.error('Could not notify satellite about pageview event occurrence (most likely blocked by browser).');
        }
    }

    /**
     * Used to notify the satellite about error events that occur.
     * Does not throw any errors so that expected UI behavior is not interrupted if the API call fails.
     *
     * @param source - place where event happened
     * @param csrfProtectionToken - CSRf token
     * @param requestID - request ID if available
     * @param statusCode = status code if available
     */
    public async errorEventTriggered(source: AnalyticsErrorEventSource, csrfProtectionToken: string, requestID: string | null = null, statusCode?: number): Promise<void> {
        try {
            const path = `${this.ROOT_PATH}/event`;
            const body = {
                eventName: AnalyticsEvent.UI_ERROR,
                errorEventSource: source,
            };

            if (requestID) body['errorEventRequestID'] = requestID;
            if (statusCode) body['errorEventStatusCode'] = statusCode;

            const response = await this.http.post(path, JSON.stringify(body), { csrfProtectionToken });
            if (response.ok) {
                return;
            }
            console.error(`Attempted to notify Satellite that UI error occurred here: ${source}. Got bad response status code: ${response.status}`);
        } catch {
            console.error(`Could not notify satellite about UI error here: ${source} (most likely blocked by browser).`);
        }
    }
}

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { APIError } from '@/utils/error';

/**
 * Exposes UI notifications functionality.
 */
export class Notificator {
    public constructor() {}

    public success(message: string, messageNode?: string): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifySuccess(message, messageNode);
    }

    public notifyError(error: Error, source: AnalyticsErrorEventSource | null): void {
        const notificationsStore = useNotificationsStore();

        if (error instanceof APIError) {
            let template = `
                <p class="message-title">${error.message}</p>
            `;
            if (error.requestID) {
                template = `
                    ${template}
                    <p class="message-footer text-caption">Request ID: ${error.requestID}</p>
                `;
            }
            notificationsStore.notifyError({ message: '', source }, template);
            return;
        }
        notificationsStore.notifyError({ message: error.message, source });
    }

    public error(message: string, source: AnalyticsErrorEventSource | null): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifyError({ message, source });
    }

    public notify(message: string): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifyInfo(message);
    }

    public warning(message: string): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifyWarning(message);
    }
}

export default {
    install(app): void {
        const instance = new Notificator();
        app.config.globalProperties = instance;
        app.provide('notify', instance);
    },
};

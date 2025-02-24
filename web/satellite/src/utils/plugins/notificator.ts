// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { h } from 'vue';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { NotificationMessage } from '@/types/DelayedNotification';

/**
 * Exposes UI notifications functionality.
 */
export class Notificator {
    public constructor() {}

    public success(message: NotificationMessage, title?: string, remainingTime?: number): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifySuccess(message, title, remainingTime);
    }

    public notifyError(error: Error, source?: AnalyticsErrorEventSource, title?: string, remainingTime?: number): void {
        const notificationsStore = useNotificationsStore();

        let msg: NotificationMessage = error.message;

        const hasRequestID = 'requestID' in error;
        if (hasRequestID) {
            msg = () => [
                h('p', { class: 'message-title' }, error.message),
                h('p', { class: 'message-footer' }, `Request ID: ${error.requestID}`),
            ];
        }

        notificationsStore.notifyError(
            msg,
            source,
            title,
            remainingTime,
            hasRequestID ? error.requestID as string : null,
            'status' in error ? error.status as number : undefined,
        );
    }

    public error(message: NotificationMessage, source: AnalyticsErrorEventSource | null = null, title?: string, remainingTime?: number): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifyError(message, source, title, remainingTime);
    }

    public notify(message: NotificationMessage, title?: string, remainingTime?: number): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifyInfo(message, title, remainingTime);
    }

    public warning(message: NotificationMessage, title?: string, remainingTime?: number): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifyWarning(message, title, remainingTime);
    }
}

export default {
    install(app): void {
        const instance = new Notificator();
        app.config.globalProperties = instance;
        app.provide('notify', instance);
    },
};

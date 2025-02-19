// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { h } from 'vue';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { APIError } from '@/utils/error';
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
        if (error instanceof APIError && error.requestID) {
            msg = () => [
                h('p', { class: 'message-title' }, error.message),
                h('p', { class: 'message-footer' }, `Request ID: ${error.requestID}`),
            ];
        }

        notificationsStore.notifyError(msg, source, title, remainingTime);
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

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { VNode, h } from 'vue';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { APIError } from '@/utils/error';
import { NotificationMessage } from '@/types/DelayedNotification';

/**
 * Exposes UI notifications functionality.
 */
export class Notificator {
    public constructor() {}

    public success(message: NotificationMessage, title?: string): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifySuccess(message, title);
    }

    public notifyError(error: Error, source?: AnalyticsErrorEventSource): void {
        const notificationsStore = useNotificationsStore();

        let msg: NotificationMessage = error.message;
        if (error instanceof APIError && error.requestID) {
            msg = () => [
                h('p', { class: 'message-title' }, error.message),
                h('p', { class: 'message-footer' }, `Request ID: ${error.requestID}`),
            ];
        }

        notificationsStore.notifyError(msg, source);
    }

    public error(message: NotificationMessage, source?: AnalyticsErrorEventSource): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifyError(message, source);
    }

    public notify(message: NotificationMessage): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifyInfo(message);
    }

    public warning(message: NotificationMessage): void {
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

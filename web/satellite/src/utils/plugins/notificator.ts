// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotificationsStore } from '@/store/modules/notificationsStore';

/**
 * Exposes UI notifications functionality.
 */
export class Notificator {
    public constructor() {}

    public success(message: string): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifySuccess(message);
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

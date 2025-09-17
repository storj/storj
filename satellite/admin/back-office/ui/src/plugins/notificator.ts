// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { NotificationMessage } from '@/types/notifications';
import { useNotificationsStore } from '@/store/notifications';

/**
 * Exposes UI notifications functionality.
 */
export class Notificator {
    public constructor() {}

    public success(message: NotificationMessage, title?: string, remainingTime?: number): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifySuccess(message, title, remainingTime);
    }

    public error(message: NotificationMessage, title?: string, remainingTime?: number): void {
        const notificationsStore = useNotificationsStore();
        notificationsStore.notifyError(message, title, remainingTime);
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

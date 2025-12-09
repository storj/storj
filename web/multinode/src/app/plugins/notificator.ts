// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { NotificationPayload } from '@/app/types/delayedNotification';
import { useNotificationsStore } from '@/app/store/notificationsStore';

/**
 * Plugin for handling notifications.
 */
export class Notify {
    constructor() {}

    /**
   * Dispatches a success notification.
   * @param payload - Payload for the notification.
   */
    public success(payload: NotificationPayload) {
        if (!payload.title) {
            payload.title = 'Success';
        }
        useNotificationsStore().notifySuccess(payload);
    }

    /**
   * Dispatches a error notification.
   * @param payload - Payload for the notification.
   */
    public error(payload: NotificationPayload) {
        if (!payload.title) {
            payload.title = 'Error';
        }
        useNotificationsStore().notifyError(payload);
    }

    /**
   * Dispatches a warning notification.
   * @param payload - Payload for the notification.
   */
    public warning(payload: NotificationPayload) {
        if (!payload.title) {
            payload.title = 'Warning';
        }
        useNotificationsStore().notifyWarning(payload);
    }

    /**
   * Dispatches a info notification.
   * @param payload - Payload for the notification.
   */
    public info(payload: NotificationPayload) {
        if (!payload.title) {
            payload.title = 'Info';
        }
        useNotificationsStore().notifyInfo(payload);
    }
}

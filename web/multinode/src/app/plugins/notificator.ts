// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { store } from '@/app/store';
import { NotificationPayload } from '@/app/types/delayedNotification';

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
        store.dispatch('notification/success', payload);
    }

    /**
   * Dispatches a error notification.
   * @param payload - Payload for the notification.
   */
    public error(payload: NotificationPayload) {
        if (!payload.title) {
            payload.title = 'Error';
        }
        store.dispatch('notification/error', payload);
    }

    /**
   * Dispatches a warning notification.
   * @param payload - Payload for the notification.
   */
    public warning(payload: NotificationPayload) {
        if (!payload.title) {
            payload.title = 'Warning';
        }
        store.dispatch('notification/warning', payload);
    }

    /**
   * Dispatches a info notification.
   * @param payload - Payload for the notification.
   */
    public info(payload: NotificationPayload) {
        if (!payload.title) {
            payload.title = 'Info';
        }
        store.dispatch('notification/info', payload);
    }
}

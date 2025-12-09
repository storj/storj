// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { DelayedNotification, NotificationPayload, NotificationType } from '@/app/types/delayedNotification';

class NotificationsState {
    public notificationQueue: DelayedNotification[] = [];
}

export const useNotificationsStore = defineStore('notifications', () => {
    const state = reactive<NotificationsState>(new NotificationsState());

    function deleteNotification(id: string): void {
        if (state.notificationQueue.length < 1) {
            return;
        }

        const selectedNotification = state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
            state.notificationQueue.splice(state.notificationQueue.indexOf(selectedNotification), 1);
        }
    }

    function pauseNotification(id: string): void {
        const selectedNotification = state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
        }
    }

    function resumeNotification(id: string): void {
        const selectedNotification = state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.start();
        }
    }

    function notifySuccess(payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Success,
            payload.message,
            payload.title,
        );

        state.notificationQueue.push(notification);
    }

    function notifyInfo(payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Info,
            payload.message,
            payload.title,
        );

        state.notificationQueue.push(notification);
    }

    function notifyWarning(payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Warning,
            payload.message,
            payload.title,
        );

        state.notificationQueue.push(notification);
    }

    function notifyError(payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Error,
            payload.message,
            payload.title,
        );

        state.notificationQueue.push(notification);
    }

    function clear(): void {
        state.notificationQueue.forEach(notification => notification.pause());
        state.notificationQueue = [];
    }

    return {
        state,
        notifySuccess,
        notifyInfo,
        notifyWarning,
        notifyError,
        pauseNotification,
        deleteNotification,
        resumeNotification,
        clear,
    };
});
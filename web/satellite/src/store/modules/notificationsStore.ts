// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { DelayedNotification, NotificationMessage, NotificationType } from '@/types/DelayedNotification';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

export class NotificationsState {
    public notificationQueue: DelayedNotification[] = [];
}

export const useNotificationsStore = defineStore('notifications', () => {
    const state = reactive<NotificationsState>(new NotificationsState());

    const analyticsStore = useAnalyticsStore();

    function addNotification(notification: DelayedNotification) {
        state.notificationQueue.push(notification);
    }

    function deleteNotification(id: string) {
        if (state.notificationQueue.length < 1) {
            return;
        }

        const selectedNotification = state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
            state.notificationQueue.splice(state.notificationQueue.indexOf(selectedNotification), 1);
        }
    }

    function pauseNotification(id: string) {
        const selectedNotification = state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
        }
    }

    function resumeNotification(id: string) {
        const selectedNotification = state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.start();
        }
    }

    function notifySuccess(message: NotificationMessage, title?: string, remainingTime?: number): void {
        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Success,
            message,
            title,
            remainingTime,
        );

        addNotification(notification);
    }

    function notifyInfo(message: NotificationMessage, title?: string, remainingTime?: number): void {
        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Info,
            message,
            title,
            remainingTime,
        );

        addNotification(notification);
    }

    function notifyWarning(message: NotificationMessage, title?: string, remainingTime?: number): void {
        const notification = new DelayedNotification(
            remainingTime ? () => deleteNotification(notification.id) : () => {},
            NotificationType.Warning,
            message,
            title,
            remainingTime,
        );

        addNotification(notification);
    }

    function notifyError(message: NotificationMessage, source: AnalyticsErrorEventSource | null = null, title?: string, remainingTime?: number, requestID: string | null = null, statusCode?: number): void {
        if (source) analyticsStore.errorEventTriggered(source, requestID, statusCode);

        const notification = new DelayedNotification(
            remainingTime ? () => deleteNotification(notification.id) : () => {},
            NotificationType.Error,
            message,
            title,
            remainingTime,
        );

        addNotification(notification);
    }

    function clear(): void {
        state.notificationQueue = [];
    }

    return {
        state,
        notifyInfo,
        notifyWarning,
        notifySuccess,
        notifyError,
        pauseNotification,
        resumeNotification,
        deleteNotification,
        clear,
    };
});

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { VNode, reactive } from 'vue';
import { defineStore } from 'pinia';

import { DelayedNotification, NotificationMessage, NotificationType } from '@/types/DelayedNotification';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

export class NotificationsState {
    public notificationQueue: DelayedNotification[] = [];
    public analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
}

export const useNotificationsStore = defineStore('notifications', () => {
    const state = reactive<NotificationsState>(new NotificationsState());

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

    function notifySuccess(message: NotificationMessage, title?: string): void {
        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Success,
            message,
            title,
        );

        addNotification(notification);
    }

    function notifyInfo(message: NotificationMessage, title?: string): void {
        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Info,
            message,
            title,
        );

        addNotification(notification);
    }

    function notifyWarning(message: NotificationMessage): void {
        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Warning,
            message,
        );

        addNotification(notification);
    }

    function notifyError(message: NotificationMessage, source?: AnalyticsErrorEventSource): void {
        if (source) {
            state.analytics.errorEventTriggered(source);
        }

        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NotificationType.Error,
            message,
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

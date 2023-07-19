// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { DelayedNotification, NOTIFICATION_TYPES } from '@/types/DelayedNotification';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

export class NotificationsState {
    public notificationQueue: DelayedNotification[] = [];
    public analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
}

interface ErrorPayload {
    message: string,
    source: AnalyticsErrorEventSource | null,
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

    function notifySuccess(message: string, messageNode?: string): void {
        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NOTIFICATION_TYPES.SUCCESS,
            message,
            messageNode,
        );

        addNotification(notification);
    }

    function notifyInfo(message: string): void {
        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NOTIFICATION_TYPES.NOTIFICATION,
            message,
        );

        addNotification(notification);
    }

    function notifyWarning(message: string): void {
        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NOTIFICATION_TYPES.WARNING,
            message,
        );

        addNotification(notification);
    }

    function notifyError(payload: ErrorPayload, messageNode?: string): void {
        if (payload.source) {
            state.analytics.errorEventTriggered(payload.source);
        }

        const notification = new DelayedNotification(
            () => deleteNotification(notification.id),
            NOTIFICATION_TYPES.ERROR,
            payload.message,
            messageNode,
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

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { DelayedNotification, NotificationMessage, NotificationType } from '@/types/notifications';

export class NotificationsState {
    public notificationQueue: DelayedNotification[] = [];
}

export const useNotificationsStore = defineStore('notifications', () => {
    const state = reactive<NotificationsState>(new NotificationsState());

    const deleteCallback = (id: symbol) => deleteNotification(id);

    function addNotification(notification: DelayedNotification) {
        state.notificationQueue.push(notification);
    }

    function deleteNotification(id: symbol) {
        if (state.notificationQueue.length < 1) {
            return;
        }

        const selectedNotification = state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
            state.notificationQueue.splice(state.notificationQueue.indexOf(selectedNotification), 1);
        }
    }

    function pauseNotification(id: symbol) {
        const selectedNotification = state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
        }
    }

    function resumeNotification(id: symbol) {
        const selectedNotification = state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.start();
        }
    }

    function notifySuccess(message: NotificationMessage, title?: string): void {
        const notification = new DelayedNotification(
            deleteCallback,
            NotificationType.Success,
            message,
            title,
        );

        addNotification(notification);
    }

    function notifyInfo(message: NotificationMessage, title?: string): void {
        const notification = new DelayedNotification(
            deleteCallback,
            NotificationType.Info,
            message,
            title,
        );

        addNotification(notification);
    }

    function notifyWarning(message: NotificationMessage): void {
        const notification = new DelayedNotification(
            deleteCallback,
            NotificationType.Warning,
            message,
        );

        addNotification(notification);
    }

    function notifyError(message: NotificationMessage): void {
        const notification = new DelayedNotification(
            deleteCallback,
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

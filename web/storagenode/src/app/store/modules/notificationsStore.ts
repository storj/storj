// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { NotificationsState, UINotification } from '@/app/types/notifications';
import { NotificationsService } from '@/storagenode/notifications/service';
import { NotificationsHttpApi } from '@/storagenode/api/notifications';

export const useNotificationsStore = defineStore('notificationsStore', () => {
    const state = reactive<NotificationsState>(new NotificationsState());

    const service: NotificationsService = new NotificationsService(new NotificationsHttpApi());

    async function fetchNotifications(pageIndex: number): Promise<void> {
        const notificationsResponse = await service.notifications(pageIndex);

        const notifications = notificationsResponse.page.notifications.map(notification => new UINotification(notification));
        const newState = new NotificationsState(notifications, notificationsResponse.page.pageCount, notificationsResponse.unreadCount);

        state.notifications = newState.notifications;
        state.pageCount = newState.pageCount;
        state.unreadCount = newState.unreadCount;

        if (pageIndex === 1) {
            state.latestNotifications = newState.notifications;
        }
    }

    async function markAsRead(id: string): Promise<void> {
        await service.readSingeNotification(id);
        state.notifications = state.notifications.map((notification: UINotification) => {
            if (notification.id === id) {
                notification.markAsRead();
            }

            return notification;
        });
    }

    async function readAll(): Promise<void> {
        await service.readAllNotifications();

        state.notifications = state.notifications.map((notification: UINotification) => {
            notification.markAsRead();

            return notification;
        });

        state.unreadCount = 0;
    }

    return {
        state,
        fetchNotifications,
        markAsRead,
        readAll,
    };
});

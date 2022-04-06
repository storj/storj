// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/app/store';
import { NotificationsState, UINotification } from '@/app/types/notifications';
import { NotificationsService } from '@/storagenode/notifications/service';

export const NOTIFICATIONS_MUTATIONS = {
    SET_NOTIFICATIONS: 'SET_NOTIFICATIONS',
    MARK_AS_READ: 'MARK_AS_READ',
    READ_ALL: 'READ_ALL',
    SET_LATEST: 'SET_LATEST',
};

export const NOTIFICATIONS_ACTIONS = {
    GET_NOTIFICATIONS: 'getNotifications',
    MARK_AS_READ: 'markAsRead',
    READ_ALL: 'readAll',
};

interface NotificationsContext {
    state: NotificationsState;
    commit: (string, ...unknown) => void;
}

/**
 * creates notifications module with all dependencies
 *
 * @param service - payments service
 */
export function newNotificationsModule(service: NotificationsService): StoreModule<NotificationsState> {
    return {
        state: new NotificationsState(),
        mutations: {
            [NOTIFICATIONS_MUTATIONS.SET_NOTIFICATIONS](state: NotificationsState, notificationsState: NotificationsState): void {
                state.notifications = notificationsState.notifications;
                state.pageCount = notificationsState.pageCount;
                state.unreadCount = notificationsState.unreadCount;
            },
            [NOTIFICATIONS_MUTATIONS.SET_LATEST](state: NotificationsState, notificationsState: NotificationsState): void {
                state.latestNotifications = notificationsState.notifications;
            },
            [NOTIFICATIONS_MUTATIONS.MARK_AS_READ](state: NotificationsState, id: string): void {
                state.notifications = state.notifications.map((notification: UINotification) => {
                    if (notification.id === id) {
                        notification.markAsRead();
                    }

                    return notification;
                });
            },
            [NOTIFICATIONS_MUTATIONS.READ_ALL](state: NotificationsState): void {
                state.notifications = state.notifications.map((notification: UINotification) => {
                    notification.markAsRead();

                    return notification;
                });

                state.unreadCount = 0;
            },
        },
        actions: {
            [NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS]: async function ({commit}: NotificationsContext, pageIndex: number): Promise<void> {
                const notificationsResponse = await service.notifications(pageIndex);

                const notifications = notificationsResponse.page.notifications.map(notification => new UINotification(notification));

                const notificationState = new NotificationsState(notifications, notificationsResponse.page.pageCount, notificationsResponse.unreadCount);

                commit(NOTIFICATIONS_MUTATIONS.SET_NOTIFICATIONS, notificationState);

                if (pageIndex === 1) {
                    commit(NOTIFICATIONS_MUTATIONS.SET_LATEST, notificationState);
                }
            },
            [NOTIFICATIONS_ACTIONS.MARK_AS_READ]: async function ({commit}: NotificationsContext, id: string): Promise<void> {
                await service.readSingeNotification(id);
                commit(NOTIFICATIONS_MUTATIONS.MARK_AS_READ, id);
            },
            [NOTIFICATIONS_ACTIONS.READ_ALL]: async function ({commit}: NotificationsContext): Promise<void> {
                await service.readAllNotifications();

                commit(NOTIFICATIONS_MUTATIONS.READ_ALL);
            },
        },
    };
}

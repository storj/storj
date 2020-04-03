// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    Notification,
    NotificationsApi,
    NotificationsCursor,
    NotificationsState,
} from '@/app/types/notifications';

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

/**
 * creates notifications module with all dependencies
 *
 * @param api - payments api
 */
export function makeNotificationsModule(api: NotificationsApi) {
    return {
        state: new NotificationsState(),
        mutations: {
            [NOTIFICATIONS_MUTATIONS.SET_NOTIFICATIONS](state: NotificationsState, notificationsResponse: NotificationsState): void {
                state.notifications = notificationsResponse.notifications;
                state.pageCount = notificationsResponse.pageCount;
                state.unreadCount = notificationsResponse.unreadCount;
            },
            [NOTIFICATIONS_MUTATIONS.SET_LATEST](state: NotificationsState, notificationsResponse: NotificationsState): void {
                state.latestNotifications = notificationsResponse.notifications;
            },
            [NOTIFICATIONS_MUTATIONS.MARK_AS_READ](state: NotificationsState, id: string): void {
                state.notifications = state.notifications.map((notification: Notification) => {
                    if (notification.id === id) {
                        notification.markAsRead();
                    }

                    return notification;
                });
            },
            [NOTIFICATIONS_MUTATIONS.READ_ALL](state: NotificationsState): void {
                state.notifications = state.notifications.map((notification: Notification) => {
                    notification.markAsRead();

                    return notification;
                });

                state.unreadCount = 0;
            },
        },
        actions: {
            [NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS]: async function ({commit}: any, cursor: NotificationsCursor): Promise<NotificationsState> {
                const notificationsResponse = await api.get(cursor);

                commit(NOTIFICATIONS_MUTATIONS.SET_NOTIFICATIONS, notificationsResponse);

                if (cursor.page === 1) {
                    commit(NOTIFICATIONS_MUTATIONS.SET_LATEST, notificationsResponse);
                }

                return notificationsResponse;
            },
            [NOTIFICATIONS_ACTIONS.MARK_AS_READ]: async function ({commit}: any, id: string): Promise<any> {
                await api.read(id);

                commit(NOTIFICATIONS_MUTATIONS.MARK_AS_READ, id);
            },
            [NOTIFICATIONS_ACTIONS.READ_ALL]: async function ({commit}: any): Promise<any> {
                await api.readAll();

                commit(NOTIFICATIONS_MUTATIONS.READ_ALL);
            },
        },
    };
}

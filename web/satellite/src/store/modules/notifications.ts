// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { NOTIFICATION_MUTATIONS } from '../mutationConstants';

import { DelayedNotification } from '@/types/DelayedNotification';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';
import { StoreModule } from '@/types/store';

export class NotificationsState {
    public notificationQueue: DelayedNotification[] = [];
}

interface NotificationsContext {
    state: NotificationsState
    commit: (string, ...unknown) => void
}

export function makeNotificationsModule(): StoreModule<NotificationsState, NotificationsContext> {
    return {
        state: new NotificationsState(),
        mutations: {
            // Mutation for adding notification to queue
            [NOTIFICATION_MUTATIONS.ADD](state: NotificationsState, notification: DelayedNotification): void {
                state.notificationQueue.push(notification);
            },
            // Mutation for deleting notification to queue
            [NOTIFICATION_MUTATIONS.DELETE](state: NotificationsState, id: string): void {
                if (state.notificationQueue.length < 1) {
                    return;
                }

                const selectedNotification =  getNotificationById(state.notificationQueue, id);
                if (selectedNotification) {
                    selectedNotification.pause();
                    state.notificationQueue.splice(state.notificationQueue.indexOf(selectedNotification), 1);
                }
            },
            [NOTIFICATION_MUTATIONS.PAUSE](state: NotificationsState, id: string): void {
                const selectedNotification =  getNotificationById(state.notificationQueue, id);
                if (selectedNotification) {
                    selectedNotification.pause();
                }
            },
            [NOTIFICATION_MUTATIONS.RESUME](state: NotificationsState, id: string): void {
                const selectedNotification =  getNotificationById(state.notificationQueue, id);
                if (selectedNotification) {
                    selectedNotification.start();
                }
            },
            [NOTIFICATION_MUTATIONS.CLEAR](state: NotificationsState): void {
                state.notificationQueue = [];
            },
        },
        actions: {
            // Commits mutation for adding success notification
            [NOTIFICATION_ACTIONS.SUCCESS]: function ({ commit }: NotificationsContext, message: string): void {
                const notification = new DelayedNotification(
                    () => commit(NOTIFICATION_MUTATIONS.DELETE, notification.id),
                    NOTIFICATION_TYPES.SUCCESS,
                    message,
                );

                commit(NOTIFICATION_MUTATIONS.ADD, notification);
            },
            // Commits mutation for adding info notification
            [NOTIFICATION_ACTIONS.NOTIFY]: function ({ commit }: NotificationsContext, message: string): void {

                const notification = new DelayedNotification(
                    () => commit(NOTIFICATION_MUTATIONS.DELETE, notification.id),
                    NOTIFICATION_TYPES.NOTIFICATION,
                    message,
                );

                commit(NOTIFICATION_MUTATIONS.ADD, notification);
            },
            // Commits mutation for adding error notification
            [NOTIFICATION_ACTIONS.ERROR]: function ({ commit }: NotificationsContext, message: string): void {
                const notification = new DelayedNotification(
                    () => commit(NOTIFICATION_MUTATIONS.DELETE, notification.id),
                    NOTIFICATION_TYPES.ERROR,
                    message,
                );

                commit(NOTIFICATION_MUTATIONS.ADD, notification);
            },
            // Commits mutation for adding error notification
            [NOTIFICATION_ACTIONS.WARNING]: function ({ commit }: NotificationsContext, message: string): void {
                const notification = new DelayedNotification(
                    () => commit(NOTIFICATION_MUTATIONS.DELETE, notification.id),
                    NOTIFICATION_TYPES.WARNING,
                    message,
                );

                commit(NOTIFICATION_MUTATIONS.ADD, notification);
            },
            [NOTIFICATION_ACTIONS.DELETE]: function ({ commit }: NotificationsContext, id: string): void {
                commit(NOTIFICATION_MUTATIONS.DELETE, id);
            },
            [NOTIFICATION_ACTIONS.PAUSE]: function ({ commit }: NotificationsContext, id: string): void {
                commit(NOTIFICATION_MUTATIONS.PAUSE, id);
            },
            [NOTIFICATION_ACTIONS.RESUME]: function ({ commit }: NotificationsContext, id: string): void {
                commit(NOTIFICATION_MUTATIONS.RESUME, id);
            },
            [NOTIFICATION_ACTIONS.CLEAR]: function ({ commit }): void {
                commit(NOTIFICATION_MUTATIONS.CLEAR);
            },
        },
        getters: {},
    };
}

function getNotificationById(notifications: DelayedNotification[], id: string): DelayedNotification | undefined {
    return notifications.find((notification: DelayedNotification) => notification.id === id);
}

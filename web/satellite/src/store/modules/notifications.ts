// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { DelayedNotification } from '@/types/DelayedNotification';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';

import { NOTIFICATION_MUTATIONS } from '../mutationConstants';

export class NotificationsState {
    public notificationQueue: DelayedNotification[] = [];
}

export function makeNotificationsModule(): StoreModule<NotificationsState> {
    return {
        state: new NotificationsState(),
        mutations: {
            // Mutation for adding notification to queue
            [NOTIFICATION_MUTATIONS.ADD](state: any, notification: DelayedNotification): void {
                state.notificationQueue.push(notification);
            },
            // Mutation for deleting notification to queue
            [NOTIFICATION_MUTATIONS.DELETE](state: any, id: string): void {
                if (state.notificationQueue.length < 1) {
                    return;
                }

                const selectedNotification =  getNotificationById(state.notificationQueue, id);
                if (selectedNotification) {
                    selectedNotification.pause();
                    state.notificationQueue.splice(state.notificationQueue.indexOf(selectedNotification), 1);
                }
            },
            [NOTIFICATION_MUTATIONS.PAUSE](state: any, id: string): void {
                const selectedNotification =  getNotificationById(state.notificationQueue, id);
                if (selectedNotification) {
                    selectedNotification.pause();
                }
            },
            [NOTIFICATION_MUTATIONS.RESUME](state: any, id: string): void {
                const selectedNotification =  getNotificationById(state.notificationQueue, id);
                if (selectedNotification) {
                    selectedNotification.start();
                }
            },
            [NOTIFICATION_MUTATIONS.CLEAR](state: any): void {
                state.notificationQueue = [];
            },
        },
        actions: {
            // Commits mutation for adding success notification
            [NOTIFICATION_ACTIONS.SUCCESS]: function ({commit}: any, message: string): void {
                const notification = new DelayedNotification(
                    () => commit(NOTIFICATION_MUTATIONS.DELETE, notification.id),
                    NOTIFICATION_TYPES.SUCCESS,
                    message,
                );

                commit(NOTIFICATION_MUTATIONS.ADD, notification);
            },
            // Commits mutation for adding info notification
            [NOTIFICATION_ACTIONS.NOTIFY]: function ({commit}: any, message: string): void {

                const notification = new DelayedNotification(
                    () => commit(NOTIFICATION_MUTATIONS.DELETE, notification.id),
                    NOTIFICATION_TYPES.NOTIFICATION,
                    message,
                );

                commit(NOTIFICATION_MUTATIONS.ADD, notification);
            },
            // Commits mutation for adding error notification
            [NOTIFICATION_ACTIONS.ERROR]: function ({commit}: any, message: string): void {
                const notification = new DelayedNotification(
                    () => commit(NOTIFICATION_MUTATIONS.DELETE, notification.id),
                    NOTIFICATION_TYPES.ERROR,
                    message,
                );

                commit(NOTIFICATION_MUTATIONS.ADD, notification);
            },
            // Commits mutation for adding error notification
            [NOTIFICATION_ACTIONS.WARNING]: function ({commit}: any, message: string): void {
                const notification = new DelayedNotification(
                    () => commit(NOTIFICATION_MUTATIONS.DELETE, notification.id),
                    NOTIFICATION_TYPES.WARNING,
                    message,
                );

                commit(NOTIFICATION_MUTATIONS.ADD, notification);
            },
            [NOTIFICATION_ACTIONS.DELETE]: function ({commit}: any, id: string): void {
                commit(NOTIFICATION_MUTATIONS.DELETE, id);
            },
            [NOTIFICATION_ACTIONS.PAUSE]: function ({commit}: any, id: string): void {
                commit(NOTIFICATION_MUTATIONS.PAUSE, id);
            },
            [NOTIFICATION_ACTIONS.RESUME]: function ({commit}: any, id: string): void {
                commit(NOTIFICATION_MUTATIONS.RESUME, id);
            },
            [NOTIFICATION_ACTIONS.CLEAR]: function ({commit}): void {
                commit(NOTIFICATION_MUTATIONS.CLEAR);
            },
        },
        getters: {},
    };
}

function getNotificationById(notifications: DelayedNotification[], id: string): DelayedNotification | undefined {
    return notifications.find((notification: DelayedNotification) => notification.id === id);
}

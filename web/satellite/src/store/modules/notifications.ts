// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { NOTIFICATION_MUTATIONS } from '../mutationConstants';
import { NOTIFICATION_TYPES } from '@/utils/constants/notification';
import { DelayedNotification } from '@/types/DelayedNotification';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

export const notificationsModule = {
    state: {
        // Dynamic queue for displaying notifications
        notificationQueue: [],
    },
    mutations: {
        // Mutaion for adding notification to queue
        [NOTIFICATION_MUTATIONS.ADD](state: any, notification: any): void {
            state.notificationQueue.push(notification);

            // Pause current notification if it`s not first
            if (state.notificationQueue.length > 1) {
                notification.pause();
            }
        },
        // Mutaion for deleting notification to queue
        [NOTIFICATION_MUTATIONS.DELETE](state: any): void {
            state.notificationQueue[0].pause();
            state.notificationQueue.shift();

            // Starts next notification in queue if it exist
            if (state.notificationQueue[0]) {
                state.notificationQueue[0].start();
            }
        },
        [NOTIFICATION_MUTATIONS.PAUSE](state: any): void {
            state.notificationQueue[0].pause();
        },
        [NOTIFICATION_MUTATIONS.RESUME](state: any): void {
            state.notificationQueue[0].start();
        },
        [NOTIFICATION_MUTATIONS.CLEAR](state: any): void {
            state.notificationQueue = [];
        },
    },
    actions: {
        // Commits muttation for adding success notification
        [NOTIFICATION_ACTIONS.SUCCESS]: function ({commit}: any, message: string): void {
            const notification = new DelayedNotification(
                () => commit(NOTIFICATION_MUTATIONS.DELETE),
                NOTIFICATION_TYPES.SUCCESS,
                message,
            );

            commit(NOTIFICATION_MUTATIONS.ADD, notification);
        },
        // Commits muttation for adding info notification
        [NOTIFICATION_ACTIONS.NOTIFY]: function ({commit}: any, message: string): void {

            const notification = new DelayedNotification(
                () => commit(NOTIFICATION_MUTATIONS.DELETE),
                NOTIFICATION_TYPES.NOTIFICATION,
                message,
            );

            commit(NOTIFICATION_MUTATIONS.ADD, notification);
        },
        // Commits muttation for adding error notification
        [NOTIFICATION_ACTIONS.ERROR]: function ({commit}: any, message: string): void {
            const notification = new DelayedNotification(
                () => commit(NOTIFICATION_MUTATIONS.DELETE),
                NOTIFICATION_TYPES.ERROR,
                message,
            );

            commit(NOTIFICATION_MUTATIONS.ADD, notification);
        },
        [NOTIFICATION_ACTIONS.DELETE]: function ({commit}: any): void {
            commit(NOTIFICATION_MUTATIONS.DELETE);
        },
        [NOTIFICATION_ACTIONS.PAUSE]: function ({commit}: any): void {
            commit(NOTIFICATION_MUTATIONS.PAUSE);
        },
        [NOTIFICATION_ACTIONS.RESUME]: function ({commit}: any): void {
            commit(NOTIFICATION_MUTATIONS.RESUME);
        },
        [NOTIFICATION_ACTIONS.CLEAR]: function ({commit}): void {
            commit(NOTIFICATION_MUTATIONS.CLEAR);
        },
    },
    getters: {
        currentNotification: (state: any) => {
            return state.notificationQueue[0] ? state.notificationQueue[0] : null;
        },
    },
};

// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { ActionContext, ActionTree, Module, MutationTree } from 'vuex';

import { DelayedNotification, NotificationType, NotificationPayload } from '../types/delayedNotification';

import { RootState } from '@/app/store/index';

/**
 * NotificationState is a representation of notification module state.
 */
export class NotificationsState {
    public notificationQueue: DelayedNotification[] = [];
}

/**
 * NotificationModule is a part of a global store that encapsulates all notification related logic.
 */
export class NotificationsModule implements Module<NotificationsState, RootState> {

    public readonly namespaced: boolean;
    public readonly state: NotificationsState;
    public readonly actions: ActionTree<NotificationsState, RootState>;
    public readonly mutations: MutationTree<NotificationsState>;

    constructor() {
        this.namespaced = true;
        this.state = new NotificationsState();

        this.mutations = {
            add: this.addNotification,
        };

        this.actions = {
            delete: this.deleteNotification.bind(this),
            pause: this.pauseNotification.bind(this),
            resume: this.resumeNotification.bind(this),
            success: this.notifySuccess.bind(this),
            info: this.notifyInfo.bind(this),
            warning: this.notifyWarning.bind(this),
            error : this.notifyError.bind(this),
            clear: this.clear.bind(this),
        };
    }

    /**
     * Add new notification to the queue
     * @param state - state of the module
     * @param notification - notification to be added
     */
    public addNotification(state: NotificationsState, notification: DelayedNotification): void {
        state.notificationQueue.push(notification);
    }

    /**
     * Delete notification from the queue
     * @param ctx - context of the Vuex action.
     * @param id - Id of the notification to be deleted.
     */
    public deleteNotification(ctx: ActionContext<NotificationsState, RootState>, id: string) {
        if (this.state.notificationQueue.length < 1) {
            return;
        }

        const selectedNotification = this.state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
            this.state.notificationQueue.splice(this.state.notificationQueue.indexOf(selectedNotification), 1);
        }
    }

    /**
     * Pause the curent notification
     * @param ctx - context of the Vuex action.
     * @param id - Id of the notification to be paused.
     */
    public pauseNotification(ctx: ActionContext<NotificationsState, RootState>, id: string) {
        const selectedNotification = this.state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
        }
    }

    /**
     * Resume the timer the notification
     * @param ctx - context of the Vuex action.
     * @param id - Id of the notification to be resumed.
     */
    public resumeNotification(ctx: ActionContext<NotificationsState, RootState>, id: string) {
        const selectedNotification = this.state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.start();
        }
    }

    /**
     * Notify success message
     * @param ctx - context of the Vuex action.
     * @param payload - payload(message and title) of the notification.
     */
    public notifySuccess(ctx: ActionContext<NotificationsState, RootState>, payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => this.deleteNotification(ctx, notification.id),
            NotificationType.Success,
            payload.message,
            payload.title,
        );

        ctx.commit('add', notification);
    }

    /**
     * Notify info message
     * @param ctx - context of the Vuex action.
     * @param payload - payload(message and title) of the notification.
     */
    public notifyInfo(ctx: ActionContext<NotificationsState, RootState>, payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => this.deleteNotification(ctx, notification.id),
            NotificationType.Info,
            payload.message,
            payload.title,
        );

        ctx.commit('add', notification);
    }

    /**
     * Notify warning message
     * @param ctx - context of the Vuex action.
     * @param payload - payload(message and title) of the notification.
     */
    public notifyWarning(ctx: ActionContext<NotificationsState, RootState>, payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => this.deleteNotification(ctx, notification.id),
            NotificationType.Warning,
            payload.message,
            payload.title,
        );

        ctx.commit('add', notification);
    }

    /**
     * Notify error message
     * @param ctx - context of the Vuex action.
     * @param payload - payload(message and title) of the notification.
     */
    public notifyError(ctx: ActionContext<NotificationsState, RootState>, payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => this.deleteNotification(ctx, notification.id),
            NotificationType.Error,
            payload.message,
            payload.title,
        );

        ctx.commit('add', notification);
    }

    /**
     * Clear all notifications from the queue.
     */
    public clear(): void {
        this.state.notificationQueue = [];
    }

}
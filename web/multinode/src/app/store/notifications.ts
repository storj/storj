// import { reactive } from 'vue';
import { ActionContext,ActionTree,Module,MutationTree } from 'vuex';
import { RootState } from '@/app/store/index';
import { DelayedNotification,NotificationMessage,NotificationType,NotificationPayload } from "../types/delayedNotification";

export class NotificationsState {
    public notificationQueue: DelayedNotification[] = [];
}

export class NotificationsModule implements Module<NotificationsState,RootState>{
    
    public readonly namespaced: boolean;
    public readonly state: NotificationsState;
    public readonly actions: ActionTree<NotificationsState, RootState>;
    public readonly mutations: MutationTree<NotificationsState>;

    constructor(){
        this.namespaced = true;
        this.state = new NotificationsState();

        this.mutations = {
           add: this.addNotification,
        }

        this.actions = {
            delete: this.deleteNotification.bind(this),
            pause: this.pauseNotification.bind(this),
            resume: this.resumeNotification.bind(this),
            success: this.notifySuccess.bind(this),
            info: this.notifyInfo.bind(this),
            warning: this.notifyWarning.bind(this),
            error : this.notifyError.bind(this),
            clear: this.clear.bind(this)
        }
    }

    public addNotification(state: NotificationsState,notification: DelayedNotification): void {
        state.notificationQueue.push(notification);
    }

    public deleteNotification(ctx: ActionContext<NotificationsState, RootState>,id: string) {
        if (this.state.notificationQueue.length < 1) {
            return;
        }

        const selectedNotification = this.state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
            this.state.notificationQueue.splice(this.state.notificationQueue.indexOf(selectedNotification), 1);
        }
    }

    public pauseNotification(ctx: ActionContext<NotificationsState, RootState>,id: string) {
        const selectedNotification = this.state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.pause();
        }
    }

    public resumeNotification(ctx: ActionContext<NotificationsState, RootState>,id: string) {
        const selectedNotification = this.state.notificationQueue.find(n => n.id === id);
        if (selectedNotification) {
            selectedNotification.start();
        }
    }

    public notifySuccess(ctx: ActionContext<NotificationsState, RootState>,payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => this.deleteNotification(ctx,notification.id),
            NotificationType.Success,
            payload.message,
            payload.title,
        );
        
        ctx.commit('add',notification);
    }

    public notifyInfo(ctx: ActionContext<NotificationsState, RootState>,payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => this.deleteNotification(ctx,notification.id),
            NotificationType.Info,
            payload.message,
            payload.title,
        );

        ctx.commit('add',notification);
    }

    public notifyWarning(ctx: ActionContext<NotificationsState, RootState>,payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => this.deleteNotification(ctx,notification.id),
            NotificationType.Warning,
            payload.message,
            payload.title
        );

        ctx.commit('add',notification);
    }

    public notifyError(ctx: ActionContext<NotificationsState, RootState>,payload: NotificationPayload): void {
        const notification: DelayedNotification = new DelayedNotification(
            () => this.deleteNotification(ctx,notification.id),
            NotificationType.Error,
            payload.message,
            payload.title
        );

        ctx.commit('add',notification);
    }

    public clear(): void {
        this.state.notificationQueue = [];
    }

}
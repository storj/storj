// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { NotificationIcon } from '@/app/utils/notificationIcons';

/**
 * Describes notification entity.
 */
export class Notification {
    public icon: NotificationIcon;

    public constructor(
        public id: string = '',
        public senderId: string = '',
        public type: NotificationTypes = NotificationTypes.Custom,
        public title: string = '',
        public message: string = '',
        public isRead: boolean = false,
        public createdAt: Date = new Date(),
    ) {
        this.setIcon();
    }

    /**
     * dateLabels formats createdAt into more informative strings.
     */
    public get dateLabel(): string {
        const differenceInSeconds = (Math.trunc(new Date().getTime()) - Math.trunc(new Date(this.createdAt).getTime())) / 1000;

        switch (true) {
            case differenceInSeconds < 60:
                return 'Just now';
            case differenceInSeconds < 3600:
                return `${(differenceInSeconds / 60).toFixed(0)} minute(s) ago`;
            case differenceInSeconds < 86400:
                return `${(differenceInSeconds / 3600).toFixed(0)} hour(s) ago`;
            case differenceInSeconds < 86400 * 2:
                return `Yesterday`;
            default:
                return this.createdAt.toDateString();
        }
    }

    /**
     * markAsRead mark notification as read on UI.
     */
    public markAsRead(): void {
        this.isRead = true;
    }

    /**
     * setIcon selects notification icon depends on type.
     */
    private setIcon(): void {
        switch (this.type) {
            case NotificationTypes.AuditCheckFailure:
                this.icon = NotificationIcon.FAIL;
                break;
            case NotificationTypes.UptimeCheckFailure:
                this.icon = NotificationIcon.DISQUALIFIED;
                break;
            case NotificationTypes.Disqualification:
                this.icon = NotificationIcon.SOFTWARE_UPDATE;
                break;
            case NotificationTypes.Suspension:
                this.icon = NotificationIcon.SUSPENDED;
                break;
            default:
                this.icon = NotificationIcon.INFO;
        }
    }
}

/**
 * Describes all current notifications types.
 */
export enum NotificationTypes {
    Custom = 0,
    AuditCheckFailure = 1,
    UptimeCheckFailure = 2,
    Disqualification = 3,
    Suspension = 4,
}

/**
 * Describes page offset for pagination.
 */
export class NotificationsCursor {
    public constructor(
        public page: number = 0,
        public limit: number = 7,
    ) { }
}

/**
 * Holds all notifications module state.
 */
export class NotificationsState {
    public latestNotifications: Notification[] = [];

    public constructor(
        public notifications: Notification[] = [],
        public pageCount: number = 0,
        public unreadCount: number = 0,
    ) { }
}

/**
 * Exposes all notifications-related functionality.
 */
export interface NotificationsApi {
    /**
     * Fetches notifications.
     * @throws Error
     */
    get(cursor: NotificationsCursor): Promise<NotificationsState>;

    /**
     * Marks single notification as read.
     * @throws Error
     */
    read(id: string): Promise<void>;

    /**
     * Marks all notification as read.
     * @throws Error
     */
    readAll(): Promise<void>;
}

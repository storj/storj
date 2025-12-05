// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all notifications-related functionality.
 */
export interface NotificationsApi {
    /**
     * Fetches notifications.
     * @throws Error
     */
    get(cursor: NotificationsCursor): Promise<NotificationsResponse>;

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

/**
 * Describes notification entity.
 */
export class Notification {
    public constructor(
        public id: string = '',
        public senderId: string = '',
        public type: NotificationTypes = NotificationTypes.Custom,
        public title: string = '',
        public message: string = '',
        public readAt: Date | null = null,
        public createdAt: Date = new Date(),
    ) {}
}

/**
 * Describes all current notifications types.
 */
export enum NotificationTypes {
    Custom = 0,
    AuditCheckFailure = 1,
    Disqualification = 2,
    Suspension = 3,
}

/**
 * Describes page offset for pagination.
 */
export class NotificationsCursor {
    private DEFAULT_LIMIT = 7;

    public constructor(
        public page: number = 0,
        public limit: number = 0,
    ) {
        if (!this.limit) {
            this.limit = this.DEFAULT_LIMIT;
        }
    }
}

/**
 * Describes response object from server.
 */
export class NotificationsResponse {
    public constructor(
        public page: NotificationsPage = new NotificationsPage(),
        public unreadCount: number = 0,
        public totalCount: number = 0,
    ) {}
}

/**
 * Describes page related notification information.
 */
export class NotificationsPage {
    public constructor(
        public notifications: Notification[] = [],
        public pageCount: number = 0,
    ) {}
}

// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { NotificationsApi, NotificationsCursor, NotificationsResponse } from '@/storagenode/notifications/notifications';

/**
 * PayoutService is used to store and handle node paystub information.
 * PayoutService exposes a business logic related to payouts.
 */
export class NotificationsService {
    private readonly api: NotificationsApi;

    public constructor(api: NotificationsApi) {
        this.api = api;
    }

    /**
     * Fetch notifications.
     *
     * @returns notifications response.
     * @throws Error
     */
    public async notifications(index: number, limit?: number): Promise<NotificationsResponse> {
        const cursor = new NotificationsCursor(index, limit);

        return await this.api.get(cursor);
    }

    /**
     * Marks single notification as read on server.
     * @param id
     */
    public async readSingeNotification(id: string): Promise<void> {
        await this.api.read(id);
    }

    /**
     * Marks all notifications as read on server.
     */
    public async readAllNotifications(): Promise<void> {
        await this.api.readAll();
    }
}

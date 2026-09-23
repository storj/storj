// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Component } from 'vue';

import { Notification, NotificationTypes } from '@/storagenode/notifications/notifications';
import DisqualificationIcon from '@/../static/images/notifications/disqualified.svg';
import FailIcon from '@/../static/images/notifications/fail.svg';
import InfoIcon from '@/../static/images/notifications/info.svg';
import SuspendedIcon from '@/../static/images/notifications/suspended.svg';

/**
 * Holds all notifications module state.
 */
export class NotificationsState {
    public latestNotifications: UINotification[] = [];

    public constructor(
        public notifications: UINotification[] = [],
        public pageCount: number = 0,
        public unreadCount: number = 0,
    ) { }
}

/**
 * Describes notification entity.
 */
export class UINotification {
    public isRead: boolean;
    public id: string;
    public senderId: string;
    public type: NotificationTypes;
    public title: string;
    public message: string;
    public link: string = '';
    public linkLabel: string = '';
    public readAt: Date | null;
    public createdAt: Date = new Date();

    public constructor(notification: Partial<UINotification> = new Notification()) {
        Object.assign(this, notification);
        this.isRead = !!this.readAt;
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
            return new Date(this.createdAt).toDateString();
        }
    }

    /**
     * safeLink returns the supporting link, but only when it is an https URL.
     *
     * The link is chosen by a satellite, and a node trusts several of them, so it is
     * validated again here: binding an unchecked value to href would let a
     * javascript: URL run code in the browser of the node operator when clicked.
     */
    public get safeLink(): string {
        if (!this.link) {
            return '';
        }

        try {
            const parsed = new URL(this.link);
            if (parsed.protocol !== 'https:') {
                return '';
            }

            return parsed.href;
        } catch {
            return '';
        }
    }

    /**
     * linkText is the label of the supporting link, falling back to the URL itself.
     */
    public get linkText(): string {
        return this.linkLabel || this.safeLink;
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
    public get icon(): Component {
        switch (this.type) {
        case NotificationTypes.AuditCheckFailure:
            return FailIcon;
        case NotificationTypes.Disqualification:
            return DisqualificationIcon;
        case NotificationTypes.Suspension:
            return SuspendedIcon;
        default:
            return InfoIcon;
        }
    }
}

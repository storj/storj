// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { getId } from '@/utils/idGenerator';

import SuccessIcon from '@/../static/images/notifications/success.svg';
import NotificationIcon from '@/../static/images/notifications/notification.svg';
import ErrorIcon from '@/../static/images/notifications/error.svg';
import WarningIcon from '@/../static/images/notifications/warning.svg';

export const NOTIFICATION_TYPES = {
    SUCCESS: 'SUCCESS',
    NOTIFICATION: 'NOTIFICATION',
    ERROR: 'ERROR',
    WARNING: 'WARNING',
};

export class DelayedNotification {
    private readonly successColor: string = '#DBF1D3';
    private readonly errorColor: string = '#FFD4D2';
    private readonly infoColor: string = '#D0E3FE';
    private readonly warningColor: string = '#FCF8E3';
    public readonly id: string;

    private readonly callback: () => void;
    private timerId: ReturnType<typeof setTimeout>;
    private startTime: number;
    private remainingTime: number;

    public readonly type: string;
    public readonly message: string;
    public readonly messageNode: string | undefined;
    public readonly style: { backgroundColor: string };
    public readonly icon: string;

    constructor(callback: () => void, type: string, message: string, messageNode?: string) {
        this.callback = callback;
        this.type = type;
        this.message = message;
        this.messageNode = messageNode;
        this.id = getId();
        this.remainingTime = 3000;
        this.start();

        // Switch for choosing notification style depends on notification type
        switch (this.type) {
        case NOTIFICATION_TYPES.SUCCESS:
            this.style = { backgroundColor: this.successColor };
            this.icon = SuccessIcon;
            break;

        case NOTIFICATION_TYPES.ERROR:
            this.style = { backgroundColor: this.errorColor };
            this.icon = ErrorIcon;
            break;

        case NOTIFICATION_TYPES.WARNING:
            this.style = { backgroundColor: this.warningColor };
            this.icon = WarningIcon;
            break;

        default:
            this.style = { backgroundColor: this.infoColor };
            this.icon = NotificationIcon;
            break;
        }
    }

    public pause(): void {
        clearTimeout(this.timerId);
        this.remainingTime -= new Date().getMilliseconds() - this.startTime;
    }

    public start(): void {
        this.startTime = new Date().getMilliseconds();
        this.timerId = setTimeout(this.callback, this.remainingTime);
    }
}

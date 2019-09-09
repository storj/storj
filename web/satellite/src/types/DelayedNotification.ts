// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { NOTIFICATION_IMAGES, NOTIFICATION_TYPES } from '@/utils/constants/notification';
import { getId } from '@/utils/idGenerator';

export class DelayedNotification {
    private readonly successColor: string = '#DBF1D3';
    private readonly errorColor: string = '#FFD4D2';
    private readonly infoColor: string = '#D0E3FE';
    private readonly warningColor: string = '#FCF8E3';
    public readonly id: string;

    private readonly callback: Function;
    private timerId: number;
    private startTime: number;
    private remainingTime: number;

    public readonly type: string;
    public readonly message: string;
    public readonly style: any;
    public readonly imgSource: string;

    constructor(callback: Function, type: string, message: string) {
        this.callback = callback;
        this.type = type;
        this.message = message;
        this.id = getId();
        this.remainingTime = 3000;
        this.start();

        // Switch for choosing notification style depends on notification type
        switch (this.type) {
            case NOTIFICATION_TYPES.SUCCESS:
                this.style = { backgroundColor: this.successColor };
                this.imgSource = NOTIFICATION_IMAGES.SUCCESS;
                break;

            case NOTIFICATION_TYPES.ERROR:
                this.style = { backgroundColor: this.errorColor };
                this.imgSource = NOTIFICATION_IMAGES.ERROR;
                break;

            case NOTIFICATION_TYPES.WARNING:
                this.style = { backgroundColor: this.warningColor };
                this.imgSource = NOTIFICATION_IMAGES.WARNING;
                break;

            default:
                this.style = { backgroundColor: this.infoColor };
                this.imgSource = NOTIFICATION_IMAGES.NOTIFICATION;
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

// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { VNode } from 'vue';

import { getId } from '@/app/utils/idGenerator';

/*
 * Hold notification types
 */
export enum NotificationType {
    Success = 'success',
    Info = 'info',
    Error = 'error',
    Warning = 'warning',
}

/**
 * Notification message can be a string or a render function that returns a string or a VNode
 */
type RenderFunction = () => (string | VNode | (string | VNode)[]);
export type NotificationMessage = string | RenderFunction;

/**
 * Payload for notification
 */
export type NotificationPayload = {
    message: NotificationMessage,
    title?: string
};

/**
 * Described notification object with it's methods and properties
 */
export class DelayedNotification {
    public readonly id: string;

    private readonly callback: () => void;
    private timerId!: ReturnType<typeof setTimeout>;
    private startTime!: number;
    private remainingTime: number;

    public readonly type: NotificationType;
    public readonly title: string | undefined;
    public readonly message: NotificationMessage;

    constructor(callback: () => void, type: NotificationType, message: NotificationMessage, title?: string) {
        this.callback = callback;
        this.type = type;
        this.title = title;
        this.message = message;
        this.id = getId();
        this.remainingTime = 3000;
        this.start();
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

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { VNode, createTextVNode } from 'vue';

export enum NotificationType {
    Success = 'Success',
    Info = 'Info',
    Error = 'Error',
    Warning = 'Warning',
}

type RenderFunction = () => (string | VNode | (string | VNode)[]);
export type NotificationMessage = string | RenderFunction;

export class DelayedNotification {
    public readonly id: string;

    private readonly callback: () => void;
    private timerId: ReturnType<typeof setTimeout>;
    private startTime: number;
    private remainingTime: number;

    public readonly type: NotificationType;
    public readonly title: string | undefined;
    public readonly messageNode: RenderFunction;

    constructor(callback: () => void, type: NotificationType, message: NotificationMessage, title?: string, remainingTime = 3000) {
        this.callback = callback;
        this.type = type;
        this.title = title;
        this.messageNode = typeof message === 'string' ? () => createTextVNode(message) : message;
        this.id = '_' + Math.random().toString(36).substr(2, 9);
        this.remainingTime = remainingTime;
        this.start();
    }

    public get alertType() {
        return this.type.toLowerCase() as 'error' | 'success' | 'warning' | 'info';
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

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
    public readonly id: symbol = Symbol();

    private readonly callback: (id: symbol) => void;
    private timerId: number;
    private startTime: number;
    private remainingTime: number;

    public readonly type: NotificationType;
    public readonly title: string | undefined;
    public readonly messageNode: RenderFunction;

    constructor(callback: (id: symbol) => void, type: NotificationType, message: NotificationMessage, title?: string) {
        this.callback = callback;
        this.type = type;
        this.title = title;
        this.messageNode = typeof message === 'string' ? () => createTextVNode(message) : message;
        this.remainingTime = 3000;
        this.start();
    }

    public pause(): void {
        clearTimeout(this.timerId);
        this.remainingTime -= new Date().getMilliseconds() - this.startTime;
    }

    public start(): void {
        this.startTime = new Date().getMilliseconds();
        this.timerId = window.setTimeout(() => this.callback(this.id), this.remainingTime);
    }
}

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { VNode, createTextVNode } from 'vue';

import { getId } from '@/utils/idGenerator';

import SuccessIcon from '@/../static/images/notifications/success.svg';
import NotificationIcon from '@/../static/images/notifications/notification.svg';
import ErrorIcon from '@/../static/images/notifications/error.svg';
import WarningIcon from '@/../static/images/notifications/warning.svg';

export enum NotificationType {
    Success = 'Success',
    Info = 'Info',
    Error = 'Error',
    Warning = 'Warning',
}

type RenderFunction = () => (string | VNode | (string | VNode)[]);
export type NotificationMessage = string | RenderFunction;

const StyleInfo: Record<NotificationType, { icon: string; backgroundColor: string }> = {
    [NotificationType.Success]: {
        backgroundColor: '#DBF1D3',
        icon: SuccessIcon,
    },
    [NotificationType.Error]: {
        backgroundColor: '#FFD4D2',
        icon: ErrorIcon,
    },
    [NotificationType.Warning]: {
        backgroundColor: '#FCF8E3',
        icon: WarningIcon,
    },
    [NotificationType.Info]: {
        backgroundColor: '#D0E3FE',
        icon: NotificationIcon,
    },
};

export class DelayedNotification {
    public readonly id: string;

    private readonly callback: () => void;
    private timerId: ReturnType<typeof setTimeout>;
    private startTime: number;
    private remainingTime: number;

    public readonly type: NotificationType;
    public readonly title: string | undefined;
    public readonly messageNode: RenderFunction;
    public readonly backgroundColor: string;
    public readonly icon: string;

    constructor(callback: () => void, type: NotificationType, message: NotificationMessage, title?: string) {
        this.callback = callback;
        this.type = type;
        this.title = title;
        this.messageNode = typeof message === 'string' ? () => createTextVNode(message) : message;
        this.id = getId();
        this.remainingTime = 3000;
        this.start();

        this.backgroundColor = StyleInfo[type].backgroundColor;
        this.icon = StyleInfo[type].icon;
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

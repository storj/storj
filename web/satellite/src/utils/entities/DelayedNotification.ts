// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

export class DelayedNotification {
    public type: string;
    public message: string;
    public id: string;
    private timerId: any = null;
    private startTime: any;
    private remainingTime: any;
    private callback: Function;

    constructor(callback: Function, type: string, message: string) {
        this.callback = callback;
        this.type = type;
        this.message = message;
        this.id = Date.now().toString();
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

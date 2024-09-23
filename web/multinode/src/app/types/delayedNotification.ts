import { VNode,h } from "vue";
import { getId } from "@/app/utils/idGenerator";

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

    private createTextVNode(message: string): VNode {
        return h('span', message)
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

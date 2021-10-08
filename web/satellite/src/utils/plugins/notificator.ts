// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { store as globalStore } from '@/store';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

interface Dispatcher {
    dispatch(key:string, message:string)
}

/**
 * Exposes UI notifications functionality.
 */
export class Notificator {
    public constructor(private store: Dispatcher) {}

    public async success(message: string): Promise<void> {
        await this.store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, message);
    }

    public async error(message: string): Promise<void> {
        await this.store.dispatch(NOTIFICATION_ACTIONS.ERROR, message);
    }

    public async notify(message: string): Promise<void> {
        await this.store.dispatch(NOTIFICATION_ACTIONS.NOTIFY, message);
    }

    public async warning(message: string): Promise<void> {
        await this.store.dispatch(NOTIFICATION_ACTIONS.WARNING, message);
    }
}

/**
 * Registers plugin in Vue instance.
 */
export class NotificatorPlugin {
    public install(localVue: { prototype: { $notify: Notificator } }): void {
        localVue.prototype.$notify = new Notificator(globalStore);
    }
}

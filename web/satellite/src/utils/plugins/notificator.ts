// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { store } from '@/store';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

/**
 * Exposes UI notifications functionality.
 */
export class Notificator {
    public constructor(private store) {}

    public async success(message: string) {
        await this.store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, message);
    }

    public async error(message: string) {
        await this.store.dispatch(NOTIFICATION_ACTIONS.ERROR, message);
    }

    public async notify(message: string) {
        await this.store.dispatch(NOTIFICATION_ACTIONS.NOTIFY, message);
    }

    public async warning(message: string) {
        await this.store.dispatch(NOTIFICATION_ACTIONS.WARNING, message);
    }
}

/**
 * Registers plugin in Vue instance.
 */
export class NotificatorPlugin {
    public install(Vue) {
        Vue.prototype.$notify = new Notificator(store);
    }
}

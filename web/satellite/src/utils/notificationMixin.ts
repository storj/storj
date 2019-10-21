// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Component, Vue } from 'vue-property-decorator';

import { NOTIFICATION_ACTIONS } from './constants/actionNames';

@Component
export default class NotificationMixin extends Vue {
    public async notificationSuccess(message: string): Promise<void> {
        await this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, message);
    }

    public async notificationError(message: string): Promise<void> {
        await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, message);
    }
}

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="content-overflow">
        <div class="content">
            <SNOContentTitle/>
            <SNOContentFilling/>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import SNOContentFilling from '@/app/components/SNOContentFilling.vue';
import SNOContentTitle from '@/app/components/SNOContentTitle.vue';

import { NODE_ACTIONS } from '@/app/store/modules/node';
import { NOTIFICATIONS_ACTIONS } from '@/app/store/modules/notifications';
import { PAYOUT_ACTIONS } from '@/app/store/modules/payout';
import { TelemetryViews } from '@/app/telemetry/telemetry';
import { NotificationsCursor } from '@/app/types/notifications';

@Component ({
    components: {
        SNOContentTitle,
        SNOContentFilling,
    },
})
export default class Dashboard extends Vue {
    /**
     * Lifecycle hook after initial render.
     * Fetches notifications and total payout information for all satellites.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, null);
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, new NotificationsCursor(1));
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_TOTAL);
        } catch (error) {
            console.error(error);
        }

        this.$telemetry.identify(this.$store.state.node.info.id);
        this.$telemetry.view(TelemetryViews.MainPage);
    }
}
</script>

<style scoped lang="scss">
    .content-overflow {
        padding: 0 36px;
        width: calc(100% - 72px);
        overflow-y: scroll;
        overflow-x: hidden;
        display: flex;
        justify-content: center;
    }

    .content {
        width: 822px;
        padding-top: 44px;
    }

    @media screen and (max-width: 1000px) {

        .content {
            width: 100%;
        }
    }

    @media screen and (max-width: 600px) {

        .content-overflow {
            padding: 0 15px;
            width: calc(100% - 30px);
        }
    }
</style>

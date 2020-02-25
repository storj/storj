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
import { NotificationsCursor } from '@/app/types/notifications';

const {
    SELECT_SATELLITE,
} = NODE_ACTIONS;

@Component ({
    components: {
        SNOContentTitle,
        SNOContentFilling,
    },
})
export default class Dashboard extends Vue {
    public mounted() {
        try {
            this.$store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, new NotificationsCursor(1));
            this.$store.dispatch(SELECT_SATELLITE, null);
        } catch (error) {
            console.error(error);
        }
    }
}
</script>

<style scoped lang="scss">
    .content-overflow {
        width: 100%;
        height: calc(100vh - 89px - 89px);
        overflow-y: scroll;
        overflow-x: hidden;
        display: flex;
        justify-content: center;
    }

    .content {
        width: 822px;
        padding-top: 44px;
    }
</style>

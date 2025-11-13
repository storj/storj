// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="content-overflow">
        <div class="content">
            <SNOContentTitle />
            <SNOContentFilling />
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';

import { usePayoutStore } from '@/app/store/modules/payoutStore';
import { useNodeStore } from '@/app/store/modules/nodeStore';
import { useAppStore } from '@/app/store/modules/appStore';
import { useNotificationsStore } from '@/app/store/modules/notificationsStore';

import SNOContentFilling from '@/app/components/SNOContentFilling.vue';
import SNOContentTitle from '@/app/components/SNOContentTitle.vue';

const payoutStore = usePayoutStore();
const nodeStore = useNodeStore();
const appStore = useAppStore();
const notificationsStore = useNotificationsStore();

onMounted(async () => {
    appStore.setLoading(true);

    try {
        await nodeStore.selectSatellite();
    } catch (error) {
        console.error(error);
    }

    try {
        await notificationsStore.fetchNotifications(1);
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchTotalPayments();
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchEstimation();
    } catch (error) {
        console.error(error);
    }

    appStore.setLoading(false);
});
</script>

<style scoped lang="scss">
    .content-overflow {
        padding: 0 36px;
        width: calc(100% - 72px);
        overflow: hidden scroll;
        display: flex;
        justify-content: center;
    }

    .content {
        width: 822px;
        padding-top: 44px;
    }

    @media screen and (width <= 1000px) {

        .content {
            width: 100%;
        }
    }

    @media screen and (width <= 600px) {

        .content-overflow {
            padding: 0 15px;
            width: calc(100% - 30px);
        }
    }
</style>

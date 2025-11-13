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

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { NODE_ACTIONS } from '@/app/store/modules/node';
import { NOTIFICATIONS_ACTIONS } from '@/app/store/modules/notifications';
import { PAYOUT_ACTIONS } from '@/app/store/modules/payout';
import { useStore } from '@/app/utils/composables';

import SNOContentTitle from '@/app/components/SNOContentTitle.vue';
import SNOContentFilling from '@/app/components/SNOContentFilling.vue';

const store = useStore();

onMounted(async () => {
    await store.dispatch(APPSTATE_ACTIONS.SET_LOADING, true);

    try {
        await store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, null);
    } catch (error) {
        console.error(error);
    }

    try {
        await store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, 1);
    } catch (error) {
        console.error(error);
    }

    try {
        await store.dispatch(PAYOUT_ACTIONS.GET_TOTAL);
    } catch (error) {
        console.error(error);
    }

    try {
        await store.dispatch(PAYOUT_ACTIONS.GET_ESTIMATION);
    } catch (error) {
        console.error(error);
    }

    await store.dispatch(APPSTATE_ACTIONS.SET_LOADING, false);
});
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

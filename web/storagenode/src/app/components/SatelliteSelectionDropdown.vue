// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-click-outside="closePopup" class="satellite-selection-choice-container">
        <div class="satellite-selection-overflow-container">
            <div class="satellite-selection-choice-container__all-satellites">
                <button name="Choose All satellite" class="satellite-selection-overflow-container__satellite-choice" type="button" @click.stop="onAllSatellitesClick">
                    <p class="satellite-selection-overflow-container__satellite-choice__name" :class="{selected: !selectedSatellite}">All Satellites</p>
                </button>
            </div>
            <!-- loop for rendering satellites -->
            <SatelliteSelectionDropdownItem
                v-for="satellite in satellites"
                :key="satellite.id"
                :satellite="satellite"
                @onSatelliteClick="onSatelliteClick"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { NODE_ACTIONS } from '@/app/store/modules/node';
import { PAYOUT_ACTIONS } from '@/app/store/modules/payout';
import { PayoutInfoRange } from '@/app/types/payout';
import { PayoutPeriod } from '@/storagenode/payouts/payouts';
import { SatelliteInfo } from '@/storagenode/sno/sno';
import { useStore } from '@/app/utils/composables';

import SatelliteSelectionDropdownItem from '@/app/components/SatelliteSelectionDropdownItem.vue';

const store = useStore();

const now = ref<Date>(new Date());

const satellites = computed<SatelliteInfo[]>(() => {
    return store.state.node.satellites;
});

const selectedSatellite = computed<string>(() => {
    return store.state.node.selectedSatellite.id;
});

const isCurrentPeriod = computed<boolean>(() => {
    const end = store.state.payoutModule.periodRange.end;
    const isCurrentMonthSelected = end.year === now.value.getUTCFullYear() && end.month === now.value.getUTCMonth();

    return !store.state.payoutModule.periodRange.start && isCurrentMonthSelected;
});

async function onSatelliteClick(id: string): Promise<void> {
    await store.dispatch(APPSTATE_ACTIONS.SET_LOADING, true);

    try {
        await store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
        await store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, id);
        await fetchPayoutInfo(id);
    } catch (error) {
        console.error(error);
    }

    await store.dispatch(APPSTATE_ACTIONS.SET_LOADING, false);
}

async function onAllSatellitesClick(): Promise<void> {
    await store.dispatch(APPSTATE_ACTIONS.SET_LOADING, true);

    try {
        await store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
        await store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, null);
        await fetchPayoutInfo();
    } catch (error) {
        console.error(error);
    }

    await store.dispatch(APPSTATE_ACTIONS.SET_LOADING, false);
}

async function fetchPayoutInfo(id = ''): Promise<void> {
    await store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_CALENDAR, false);
    await store.dispatch(APPSTATE_ACTIONS.SET_NO_PAYOUT_DATA, false);

    if (!isCurrentPeriod.value) {
        try {
            await store.dispatch(PAYOUT_ACTIONS.SET_PERIODS_RANGE, new PayoutInfoRange(null, new PayoutPeriod()));
        } catch (error) {
            console.error(error);
        }
    }

    try {
        await store.dispatch(PAYOUT_ACTIONS.GET_ESTIMATION, id);
    } catch (error) {
        console.error(error);
    }

    try {
        await store.dispatch(PAYOUT_ACTIONS.GET_PRICING_MODEL, id);
    } catch (error) {
        console.error(error);
    }

    try {
        await store.dispatch(PAYOUT_ACTIONS.GET_TOTAL, id);
    } catch (error) {
        console.error(error);
    }

    try {
        await store.dispatch(PAYOUT_ACTIONS.GET_PERIODS, id);
    } catch (error) {
        console.error(error);
    }
}

function closePopup(): void {
    store.dispatch(APPSTATE_ACTIONS.CLOSE_ALL_POPUPS);
}
</script>

<style scoped lang="scss">
    .satellite-selection-choice-container {
        position: absolute;
        top: 50px;
        left: 0;
        width: 100%;
        border-radius: 8px;
        padding: 7px 0;
        box-shadow: 0 4px 4px rgb(0 0 0 / 25%);
        background-color: var(--block-background-color);
        z-index: 103;
    }

    .satellite-selection-overflow-container {
        overflow-y: auto;
        overflow-x: hidden;
        height: auto;

        &__satellite-choice {
            position: relative;
            display: flex;
            width: calc(100% - 36px);
            align-items: center;
            justify-content: flex-start;
            margin-left: 8px;
            border-radius: 12px;
            padding: 10px;

            &__image {
                position: absolute;
                top: 10px;
                left: 10px;
            }

            &__name {
                font-size: 14px;
                line-height: 21px;
            }

            &:hover {
                background-color: var(--satellite-selection-hover-background-color);
                cursor: pointer;
                color: var(--regular-text-color);
            }
        }
    }

    .disqualified,
    .suspended {
        margin-left: 20px;
    }

    /* width */

    ::-webkit-scrollbar {
        width: 4px;
    }

    /* Track */

    ::-webkit-scrollbar-track {
        box-shadow: inset 0 0 5px #fff;
    }

    /* Handle */

    ::-webkit-scrollbar-thumb {
        background: #afb7c1;
        border-radius: 6px;
        height: 5px;
    }
</style>

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
                @on-satellite-click="onSatelliteClick"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { PayoutInfoRange } from '@/app/types/payout';
import { PayoutPeriod } from '@/storagenode/payouts/payouts';
import { SatelliteInfo } from '@/storagenode/sno/sno';
import { useAppStore } from '@/app/store/modules/appStore';
import { usePayoutStore } from '@/app/store/modules/payoutStore';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import SatelliteSelectionDropdownItem from '@/app/components/SatelliteSelectionDropdownItem.vue';

const appStore = useAppStore();
const payoutStore = usePayoutStore();
const nodeStore = useNodeStore();

const now = ref<Date>(new Date());

const satellites = computed<SatelliteInfo[]>(() => {
    return nodeStore.state.satellites;
});

const selectedSatellite = computed<string>(() => {
    return nodeStore.state.selectedSatellite.id;
});

const isCurrentPeriod = computed<boolean>(() => {
    const end = payoutStore.state.periodRange.end;
    const isCurrentMonthSelected = end.year === now.value.getUTCFullYear() && end.month === now.value.getUTCMonth();

    return !payoutStore.state.periodRange.start && isCurrentMonthSelected;
});

async function onSatelliteClick(id: string): Promise<void> {
    appStore.setLoading(true);

    try {
        appStore.toggleSatelliteSelection();
        await nodeStore.selectSatellite(id);
        await fetchPayoutInfo(id);
    } catch (error) {
        console.error(error);
    }

    appStore.setLoading(false);
}

async function onAllSatellitesClick(): Promise<void> {
    appStore.setLoading(true);

    try {
        appStore.toggleSatelliteSelection();
        await nodeStore.selectSatellite();
        await fetchPayoutInfo();
    } catch (error) {
        console.error(error);
    }

    appStore.setLoading(false);
}

async function fetchPayoutInfo(id = ''): Promise<void> {
    appStore.togglePayoutCalendar(false);
    appStore.setNoPayoutData(false);

    if (!isCurrentPeriod.value) {
        payoutStore.setPeriodsRange(new PayoutInfoRange(null, new PayoutPeriod()));
    }

    try {
        await payoutStore.fetchEstimation(id);
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchPricingModel(id);
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchTotalPayments(id);
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.getPeriods(id);
    } catch (error) {
        console.error(error);
    }
}

function closePopup(): void {
    appStore.closeAllPopups();
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
        overflow: hidden auto;
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

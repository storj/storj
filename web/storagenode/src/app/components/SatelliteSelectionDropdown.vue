// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="satellite-selection-choice-container" v-click-outside="closePopup">
        <div class="satellite-selection-overflow-container">
            <div class="satellite-selection-choice-container__all-satellites">
                <div class="satellite-selection-overflow-container__satellite-choice" @click.stop="onAllSatellitesClick">
                    <p class="satellite-selection-overflow-container__satellite-choice__name" :class="{selected: !selectedSatellite}">All Satellites</p>
                </div>
            </div>
            <!-- loop for rendering satellites -->
            <div class="satellite-selection-overflow-container__satellite-choice"
                v-for="satellite in satellites" :key="satellite.id"
                @click.stop="onSatelliteClick(satellite.id)">
                <DisqualificationIcon
                    class="satellite-selection-overflow-container__satellite-choice__image"
                    v-if="satellite.disqualified"
                    alt="disqualified image"
                />
                <SuspensionIcon
                    class="satellite-selection-overflow-container__satellite-choice__image"
                    v-if="satellite.suspended && !satellite.disqualified"
                    alt="suspended image"
                />
                <p class="satellite-selection-overflow-container__satellite-choice__name" :class="{disqualified: satellite.disqualified, suspended: satellite.suspended}">{{satellite.url}}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import DisqualificationIcon from '@/../static/images/disqualify.svg';
import SuspensionIcon from '@/../static/images/suspend.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { NODE_ACTIONS } from '@/app/store/modules/node';
import { PAYOUT_ACTIONS } from '@/app/store/modules/payout';
import { PayoutInfoRange, PayoutPeriod } from '@/app/types/payout';
import { SatelliteInfo } from '@/storagenode/dashboard';

@Component({
    components: {
        DisqualificationIcon,
        SuspensionIcon,
    },
})
export default class SatelliteSelectionDropdown extends Vue {
    private now: Date = new Date();

    /**
     * Returns node satellites list from store.
     */
    public get satellites(): SatelliteInfo[] {
        return this.$store.state.node.satellites;
    }

    /**
     * Returns selected satellite id from store.
     */
    public get selectedSatellite(): string {
        return this.$store.state.node.selectedSatellite.id;
    }

    /**
     * Indicates if current month selected.
     */
    public get isCurrentPeriod(): boolean {
        const end = this.$store.state.payoutModule.periodRange.end;
        const isCurrentMonthSelected = end.year === this.now.getUTCFullYear() && end.month === this.now.getUTCMonth();

        return !this.$store.state.payoutModule.periodRange.start && isCurrentMonthSelected;
    }

    /**
     * Fires on satellite click and selects it.
     */
    public async onSatelliteClick(id: string): Promise<void> {
        try {
            await this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
            await this.$store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, id);
            this.fetchPayoutInfo(id);
        } catch (error) {
            console.error(error.message);
        }
    }

    /**
     * Fires on all satellites click and sets selected satellite id to null.
     */
    public async onAllSatellitesClick(): Promise<void> {
        try {
            await this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
            await this.$store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, null);
            this.fetchPayoutInfo();
        } catch (error) {
            console.error(error.message);
        }
    }

    /**
     * Closes dropdown.
     */
    public closePopup(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.CLOSE_ALL_POPUPS);
    }

    /**
     * Fetches payout information depends on selected satellite.
     */
    private async fetchPayoutInfo(id: string = ''): Promise<void> {
        await this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_PAYOUT_CALENDAR, false);

        if (!this.isCurrentPeriod) {
            try {
                await this.$store.dispatch(PAYOUT_ACTIONS.SET_PERIODS_RANGE, new PayoutInfoRange(null, new PayoutPeriod()));
            } catch (error) {
                console.error(error.message);
            }
        }

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_TOTAL, id);
        } catch (error) {
            console.error(error.message);
        }
    }
}
</script>

<style scoped lang="scss">
    .satellite-selection-choice-container {
        position: absolute;
        top: 50px;
        left: 0;
        width: 100%;
        border-radius: 8px;
        padding: 7px 0 7px 0;
        box-shadow: 0 4px 4px rgba(0, 0, 0, 0.25);
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

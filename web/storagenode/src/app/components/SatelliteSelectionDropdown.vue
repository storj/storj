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
                <p class="satellite-selection-overflow-container__satellite-choice__name" :class="{disqualified: satellite.disqualified}">{{satellite.id}}</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import DisqualificationIcon from '@/../static/images/disqualify.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { NODE_ACTIONS } from '@/app/store/modules/node';
import { SatelliteInfo } from '@/storagenode/dashboard';

@Component({
    components: {
        DisqualificationIcon,
    },
})
export default class SatelliteSelectionDropdown extends Vue {
    public async onSatelliteClick(id: string): Promise<void> {
        try {
            await this.$store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, id);
            await this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
        } catch (error) {
            console.error(`${error.message} satellite data.`);
        }
    }

    public async onAllSatellitesClick(): Promise<void> {
        try {
            await this.$store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, null);
            await this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
        } catch (error) {
            console.error(`${error.message} satellite data.`);
        }
    }

    public closePopup(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.CLOSE_ALL_POPUPS);
    }

    public get satellites(): SatelliteInfo[] {
        return this.$store.state.node.satellites;
    }

    public get selectedSatellite(): string {
        return this.$store.state.node.selectedSatellite.id;
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
        background-color: #fff;
        z-index: 1120;
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
                background-color: #ebecf0;
                cursor: pointer;
            }
        }
    }

    .disqualified {
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

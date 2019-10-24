// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="satellite-selection-choice-container" id="satelliteDropdown">
        <div class="satellite-selection-overflow-container">
            <!-- loop for rendering satellites -->
            <div class="satellite-selection-overflow-container__satellite-choice"
                v-for="satellite in satellites" :key="satellite.id"
                @click.stop="onSatelliteClick(satellite.id)">
                <svg class="satellite-selection-overflow-container__satellite-choice__image" v-if="satellite.disqualified" width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg" alt="disqualified image">
                    <path d="M16.6625 13.9324C16.441 13.5232 16.2181 13.1133 15.996 12.702C15.5825 11.9391 15.1698 11.1762 14.7571 10.4133C14.2614 9.4992 13.7671 8.58373 13.2721 7.66969C12.7939 6.78728 12.3172 5.90625 11.8391 5.02459C11.4875 4.37631 11.1374 3.72733 10.7858 3.07843C10.6909 2.90265 10.596 2.72688 10.501 2.55249C10.4026 2.36968 10.2985 2.1939 10.1495 2.04484C9.62918 1.521 8.76574 1.42045 8.13997 1.81633C7.87137 1.98648 7.67732 2.22766 7.52826 2.50398C7.29975 2.92587 7.07122 3.34773 6.84271 3.77102C6.42646 4.54093 6.00951 5.31087 5.59326 6.08078C5.09192 6.99977 4.59482 7.92092 4.0956 8.84198C3.6231 9.71527 3.1499 10.5871 2.6767 11.4612C2.33076 12.101 1.98411 12.7394 1.63749 13.3792C1.54608 13.548 1.45468 13.7167 1.36328 13.8855C1.23531 14.1231 1.13477 14.3636 1.10312 14.6378C1.01383 15.4127 1.55663 16.1763 2.30687 16.3661C2.50516 16.4167 2.70062 16.4189 2.90102 16.4189H15.276H15.2957C15.7035 16.4104 16.0902 16.2571 16.3891 15.9794C16.6773 15.7122 16.8461 15.3466 16.8939 14.9599C16.9396 14.5957 16.8341 14.2511 16.6626 13.9326L16.6625 13.9324ZM8.29666 6.27882C8.29666 5.88507 8.6187 5.59327 8.99978 5.5757C9.37947 5.55812 9.70289 5.9118 9.70289 6.27882V11.2303C9.70289 11.624 9.38085 11.9158 8.99978 11.9334C8.62008 11.951 8.29666 11.5973 8.29666 11.2303V6.27882ZM8.99978 14.3282C8.60462 14.3282 8.28399 14.0083 8.28399 13.6124C8.28399 13.2173 8.6039 12.8967 8.99978 12.8967C9.39493 12.8967 9.71556 13.2166 9.71556 13.6124C9.71556 14.0076 9.39495 14.3282 8.99978 14.3282Z" fill="#F4D638"/>
                </svg>
                <p class="satellite-selection-overflow-container__satellite-choice__name" :class="{disqualified: satellite.disqualified}">{{satellite.id}}</p>
            </div>
            <div class="satellite-selection-choice-container__all-satellites">
                <div class="satellite-selection-overflow-container__satellite-choice" @click.stop="onSatelliteClick(null)">
                    <p class="satellite-selection-overflow-container__satellite-choice__name" :class="{selected: !selectedSatellite}">All Satellites</p>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { NODE_ACTIONS } from '@/app/store/modules/node';
import { SatelliteInfo } from '@/storagenode/dashboard';

@Component
export default class SatelliteSelectionDropdown extends Vue {
    public async onSatelliteClick(id: string): Promise<void> {
        try {
            await this.$store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, id);
            await this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
        } catch (error) {
            console.error(`${error.message} satellite data.`);
        }
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
                top: 13px;
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

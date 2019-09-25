// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="satellite-selection-toggle-container" v-if="satellites" @click="toggleDropDown">
        <p><b class="satellite-selection-toggle-container__bold-text">Choose your satellite: </b>{{selectedSatellite ? selectedSatellite : 'All satellites'}}</p>
        <svg class="satellite-selection-toggle-container__image" width="8" height="4" viewBox="0 0 8 4" fill="none" xmlns="http://www.w3.org/2000/svg" alt="arrow image">
            <path d="M3.33657 3.73107C3.70296 4.09114 4.29941 4.08814 4.66237 3.73107L7.79796 0.650836C8.16435 0.291517 8.01864 0 7.47247 0L0.526407 0C-0.0197628 0 -0.16292 0.294525 0.200917 0.650836L3.33657 3.73107Z" fill="#535F77"/>
        </svg>
        <SatelliteSelectionDropdown v-if="isPopupShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { SatelliteInfo } from '@/storagenode/dashboard';

import SatelliteSelectionDropdown from './SatelliteSelectionDropdown.vue';

@Component({
    components: {
        SatelliteSelectionDropdown,
    },
})
export default class SatelliteSelectionContainer extends Vue {
    @Prop({default: ''})
    private readonly label: string;

    public toggleDropDown(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
    }

    public get satellites(): SatelliteInfo[] {
        return this.$store.state.node.satellites;
    }

    public get selectedSatellite(): string {
        return this.$store.state.node.selectedSatellite.id;
    }

    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.isSatelliteSelectionShown;
    }
}
</script>

<style lang="scss">
    .satellite-selection-toggle-container {
        width: calc(100%-28px);
        height: 44px;
        display: flex;
        justify-content: flex-start;
        align-items: center;
        background-color: #FFFFFF;
        border: 1px solid #E8E8E8;
        border-radius: 12px;
        padding: 0 14px 0 14px;
        position: relative;
        font-size: 14px;
        cursor: pointer;
        color: #535F77;

        &__bold-text {
            margin-right: 3px;
        }

        &__image {
            position: absolute;
            right: 14px;
        }
    }
</style>

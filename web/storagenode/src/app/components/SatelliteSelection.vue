// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="satellite-selection-toggle-container" v-if="satellites" @click.stop="toggleDropDown">
        <p><b class="satellite-selection-toggle-container__bold-text">Choose your satellite: </b>{{selectedSatellite ? selectedSatellite : 'All Satellites'}}</p>
        <DropdownArrowIcon
            class="satellite-selection-toggle-container__image"
            alt="Arrow down"
        />
        <SatelliteSelectionDropdown v-if="isPopupShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import DropdownArrowIcon from '@/../static/images/dropdownArrow.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { SatelliteInfo } from '@/storagenode/dashboard';

import SatelliteSelectionDropdown from './SatelliteSelectionDropdown.vue';

@Component({
    components: {
        SatelliteSelectionDropdown,
        DropdownArrowIcon,
    },
})
export default class SatelliteSelection extends Vue {
    @Prop({default: ''})
    private readonly label: string;

    public toggleDropDown(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
    }

    public get satellites(): SatelliteInfo[] {
        return this.$store.state.node.satellites;
    }

    public get selectedSatellite(): string {
        return this.$store.state.node.selectedSatellite.url;
    }

    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.isSatelliteSelectionShown;
    }
}
</script>

<style scoped lang="scss">
    .satellite-selection-toggle-container {
        width: calc(100% - 26px);
        height: 44px;
        display: flex;
        justify-content: flex-start;
        align-items: center;
        background-color: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        border-radius: 12px;
        padding: 0 12px;
        position: relative;
        font-size: 14px;
        cursor: pointer;
        color: var(--regular-text-color);

        &__bold-text {
            margin-right: 3px;
        }

        &__image {
            position: absolute;
            right: 14px;
        }
    }
</style>

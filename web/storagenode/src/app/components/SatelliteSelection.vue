// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <button
        v-if="satellites"
        name="Choose your satellite"
        class="satellite-selection-toggle-container"
        type="button"
        @click.stop="toggleDropDown"
    >
        <p
            class="satellite-selection-toggle-container__text"
            :class="{'with-id-button': selectedSatellite.id && isNameShown, 'with-copy-button': selectedSatellite.id && !isNameShown}"
        >
            <b class="satellite-selection-toggle-container__bold-text">Choose your satellite: </b>{{ label }}
        </p>
        <div v-if="selectedSatellite.id" class="satellite-selection__right-area">
            <div
                v-if="isNameShown"
                class="satellite-selection-toggle-container__right-area__button"
                @click.stop.prevent="toggleSatelliteView"
            >
                <EyeIcon />
                <p class="satellite-selection-toggle-container__right-area__button__text">ID</p>
            </div>
            <div v-else class="row">
                <div
                    v-clipboard:copy="selectedSatellite.id"
                    class="satellite-selection-toggle-container__right-area__button copy-button"
                    @click.stop="() => {}"
                >
                    <CopyIcon />
                </div>
                <div class="satellite-selection-toggle-container__right-area__button" @click.stop.prevent="toggleSatelliteView">
                    <EyeIcon />
                    <p class="satellite-selection-toggle-container__right-area__button__text">Name</p>
                </div>
            </div>
        </div>
        <DropdownArrowIcon
            class="satellite-selection-toggle-container__image"
            alt="Arrow down"
        />
        <SatelliteSelectionDropdown v-if="isPopupShown" />
    </button>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CopyIcon from '@/../static/images/Copy.svg';
import DropdownArrowIcon from '@/../static/images/dropdownArrow.svg';
import EyeIcon from '@/../static/images/Eye.svg';

import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { SatelliteInfo } from '@/storagenode/sno/sno';

import SatelliteSelectionDropdown from './SatelliteSelectionDropdown.vue';

// @vue/component
@Component({
    components: {
        SatelliteSelectionDropdown,
        DropdownArrowIcon,
        CopyIcon,
        EyeIcon,
    },
})
export default class SatelliteSelection extends Vue {
    /**
     * Indicates if name or id should be shown.
     */
    public isNameShown = true;

    /**
     * Returns label depends on which satellite is selected.
     */
    public get label(): string {
        if (!this.selectedSatellite.id) {
            return 'All Satellites';
        }

        return this.isNameShown ? this.selectedSatellite.url : this.selectedSatellite.id;
    }

    /**
     * Toggles between name and id view.
     */
    public toggleSatelliteView(): void {
        this.isNameShown = !this.isNameShown;
    }

    public toggleDropDown(): void {
        this.$store.dispatch(APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION);
    }

    public get satellites(): SatelliteInfo[] {
        return this.$store.state.node.satellites;
    }

    public get selectedSatellite(): SatelliteInfo {
        return this.$store.state.node.selectedSatellite;
    }

    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.isSatelliteSelectionShown;
    }
}
</script>

<style scoped lang="scss">
    .satellite-selection-toggle-container {
        width: calc(100% - 67px);
        height: 44px;
        display: flex;
        justify-content: space-between;
        align-items: center;
        background-color: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        border-radius: 12px;
        padding: 0 55px 0 12px;
        position: relative;
        font-size: 14px;
        cursor: pointer;
        color: var(--regular-text-color);

        &__text {
            width: calc(100% - 10px);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        &__bold-text {
            margin-right: 3px;
        }

        &__right-area {

            &__button {
                display: flex;
                align-items: center;
                justify-content: center;
                background: var(--button-background-color);
                border-radius: 5px;
                height: 30px;
                padding: 0 10px;
                cursor: pointer;
                font-family: 'font_medium', sans-serif;
                font-size: 13px;
                color: #9cabbe;
                border: transparent;

                &__text {
                    margin-left: 6.75px;
                }

                &:hover {
                    background-color: #e4ebfc;
                    cursor: pointer;
                    color: #133e9c;

                    .svg ::v-deep path {
                        fill: #133e9c !important;
                    }
                }
            }
        }

        &__image {
            position: absolute;
            right: 14px;
        }
    }

    .copy-button {
        margin-right: 8px;
    }

    .row {
        display: flex;
    }

    .with-id-button {
        width: calc(100% - 90px);
    }

    .with-copy-button {
        width: calc(100% - 155px);
    }
</style>

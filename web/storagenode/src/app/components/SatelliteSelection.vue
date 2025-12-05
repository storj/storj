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
                    class="satellite-selection-toggle-container__right-area__button copy-button"
                    @click.stop="onCopy"
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

<script setup lang="ts">
import { computed, ref } from 'vue';

import SatelliteSelectionDropdown from './SatelliteSelectionDropdown.vue';

import { SatelliteInfo } from '@/storagenode/sno/sno';
import { useAppStore } from '@/app/store/modules/appStore';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import CopyIcon from '@/../static/images/Copy.svg';
import DropdownArrowIcon from '@/../static/images/dropdownArrow.svg';
import EyeIcon from '@/../static/images/Eye.svg';

const appStore = useAppStore();
const nodeStore = useNodeStore();

const isNameShown = ref<boolean>(true);

const label = computed<string>(() => {
    if (!selectedSatellite.value.id) {
        return 'All Satellites';
    }

    return isNameShown.value ? selectedSatellite.value.url : selectedSatellite.value.id;
});

const satellites = computed<SatelliteInfo[]>(() => {
    return nodeStore.state.satellites;
});

const selectedSatellite = computed<SatelliteInfo>(() => {
    return nodeStore.state.selectedSatellite;
});

const isPopupShown = computed<boolean>(() => {
    return appStore.state.isSatelliteSelectionShown;
});

function onCopy(): void {
    navigator.clipboard.writeText(selectedSatellite.value.id);
}

function toggleSatelliteView(): void {
    isNameShown.value = !isNameShown.value;
}

function toggleDropDown(): void {
    appStore.toggleSatelliteSelection();
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

                    .svg :deep(path) {
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

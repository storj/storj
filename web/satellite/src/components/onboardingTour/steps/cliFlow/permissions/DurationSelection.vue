// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="duration-selection">
        <div
            class="duration-selection__toggle-container"
            aria-roledescription="select-duration"
            @click.stop="togglePicker"
        >
            <h1 class="duration-selection__toggle-container__name">{{ dateRangeLabel }}</h1>
            <ExpandIcon
                class="duration-selection__toggle-container__expand-icon"
                alt="Arrow down (expand)"
            />
        </div>
        <DurationPicker
            v-if="isDurationPickerVisible"
            @setLabel="setDateRangeLabel"
        />
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';

import DurationPicker from '@/components/onboardingTour/steps/cliFlow/permissions/DurationPicker.vue';

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

const appStore = useAppStore();
const agStore = useAccessGrantsStore();

const dateRangeLabel = ref<string>('Forever');

/**
 * Indicates if date picker is shown.
 */
const isDurationPickerVisible = computed((): boolean => {
    return appStore.state.viewsState.activeDropdown === APP_STATE_DROPDOWNS.AG_DATE_PICKER;
});

/**
 * Returns not before date permission from store.
 */
const notBeforePermission = computed((): Date | null => {
    return agStore.state.permissionNotBefore;
});

/**
 * Returns not after date permission from store.
 */
const notAfterPermission = computed((): Date | null => {
    return agStore.state.permissionNotAfter;
});

/**
 * Toggles duration picker.
 */
function togglePicker(): void {
    appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.AG_DATE_PICKER);
}

/**
 * Sets date range label.
 */
function setDateRangeLabel(label: string): void {
    dateRangeLabel.value = label;
}

/**
 * Mounted hook after initial render.
 * Sets previously selected date range if exists.
 */
onMounted(() => {
    if (notBeforePermission.value && notAfterPermission.value) {
        const fromFormattedString = notBeforePermission.value.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: '2-digit' });
        const toFormattedString = notAfterPermission.value.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: '2-digit' });
        dateRangeLabel.value = `${fromFormattedString} - ${toFormattedString}`;
    }
});
</script>

<style scoped lang="scss">
    .duration-selection {
        background-color: #fff;
        cursor: pointer;
        margin-left: 15px;
        border-radius: 6px;
        border: 1px solid rgb(56 75 101 / 40%);
        font-family: 'font_regular', sans-serif;
        width: 235px;
        position: relative;

        &__toggle-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 15px 20px;
            width: calc(100% - 40px);

            &__name {
                font-style: normal;
                font-weight: normal;
                font-size: 16px;
                line-height: 21px;
                color: #384b65;
                margin: 0;
            }
        }
    }

    .access-date-container {
        margin-left: 0;
        height: 40px;
        border: 1px solid var(--c-grey-4);
    }

    .access-date-text {
        padding: 10px 20px;
    }
</style>

// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="date-select">
        <div
            class="date-select__toggle-container"
            tabindex="0"
            aria-roledescription="select-date"
            @click.stop="togglePicker"
            @keyup.space="togglePicker"
        >
            <h1 class="date-select__toggle-container__label">{{ notAfterLabel }}</h1>
            <ExpandIcon />
        </div>
        <EndDatePicker
            v-if="isDatePickerVisible"
            :set-label="setNotAfterLabel"
            :set-not-after="setNotAfter"
        />
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import EndDatePicker from './EndDatePicker.vue';

import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';

import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

const props = defineProps<{
    setNotAfter: (date: Date | undefined) => void;
    setNotAfterLabel: (label: string) => void;
    notAfterLabel: string;
}>();

const appStore = useAppStore();

/**
 * Indicates if date picker is shown.
 */
const isDatePickerVisible = computed((): boolean => {
    return appStore.state.viewsState.activeDropdown === APP_STATE_DROPDOWNS.AG_DATE_PICKER;
});

/**
 * Toggles date picker.
 */
function togglePicker(): void {
    appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.AG_DATE_PICKER);
}
</script>

<style scoped lang="scss">
    .date-select {
        background-color: var(--c-white);
        cursor: pointer;
        border-radius: 6px;
        border: 1px solid rgb(56 75 101 / 40%);
        font-family: 'font_regular', sans-serif;
        position: relative;
        box-sizing: border-box;
        width: 100%;

        &__toggle-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 10px 16px;
            width: calc(100% - 32px);

            &__label {
                font-size: 16px;
                line-height: 21px;
                color: var(--c-grey-7);
                margin: 0;
            }
        }
    }
</style>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="selector">
        <div tabindex="0" class="selector__content" @keyup.enter="toggleSelector" @click.stop="toggleSelector">
            <span v-if="selected" class="selector__content__label">{{ selected?.shortString }}</span>
            <span v-else class="selector__content__label">Select duration</span>
            <arrow-down-icon class="selector__content__arrow" :class="{ open: isOpen }" />
        </div>
        <div
            v-if="isOpen"
            v-click-outside="closeSelector"
            tabindex="0"
            class="selector__dropdown"
        >
            <div
                v-for="(option, index) in options"
                :key="index"
                tabindex="0"
                class="selector__dropdown__item"
                :class="{ selected: isSelected(option) }"
                @click.stop="() => select(option)"
                @keyup.enter="() => select(option)"
            >
                <span class="selector__dropdown__item__label">{{ option.shortString }}</span>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { Duration } from '@/utils/time';
import { useAppStore } from '@/store/modules/appStore';

import ArrowDownIcon from '@/../static/images/common/dropIcon.svg';

const appStore = useAppStore();

const props = defineProps<{
    selected: Duration | null;
}>();

const emit = defineEmits<{
    // select is emitted when the selection changes.
    (e: 'select', duration: Duration): void
}>();

const options = [
    Duration.MINUTES_15,
    Duration.MINUTES_30,
    Duration.HOUR_1,
    Duration.DAY_1,
    Duration.WEEK_1,
    Duration.DAY_30,
];

/**
 * whether the selector drop down is open
 * */
const isOpen = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.TIMEOUT_SELECTOR;
});

/**
 * whether an option is currently selected.
 * @param option
 * */
function isSelected(option: Duration): boolean {
    if (!props.selected) {
        return false;
    }
    return props.selected.isEqualTo(option);
}

/**
 * select sends the new selection to a parent component.
 * @param option the new selection
 * */
function select(option: Duration) {
    emit('select', option);
    closeSelector();
}

/**
 * closeSelector closes the selector dropdown.
 * */
function closeSelector() {
    appStore.closeDropdowns();
}

/**
 * toggleSelector closes or opens the selector dropdown
 * */
function toggleSelector() {
    if (isOpen.value) {
        appStore.closeDropdowns();
    } else {
        appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.TIMEOUT_SELECTOR);
    }
}
</script>

<style scoped lang="scss">
.selector {
    border: 1px solid var(--c-grey-3);
    border-radius: 6px;
    max-width: 170px;
    position: relative;
    box-sizing: border-box;

    &__content {
        display: flex;
        align-items: center;
        justify-content: space-between;
        position: relative;
        padding: 10px 14px;

        &__label {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-grey-6);
            cursor: default;
        }

        &__arrow {
            transition-duration: 0.5s;

            &.open {
                transform: rotate(180deg) scaleX(-1);
            }
        }
    }

    &__dropdown {
        position: absolute;
        top: 50px;
        background: var(--c-white);
        z-index: 999;
        box-sizing: border-box;
        box-shadow: 0 -2px 16px rgb(0 0 0 / 10%);
        border-radius: 8px;
        border: 1px solid var(--c-grey-2);
        width: 100%;

        &__item {
            padding: 10px;

            &__label {
                cursor: default;
            }

            &.selected {
                background: var(--c-grey-1);
            }

            &:first-of-type {
                border-top-right-radius: 8px;
                border-top-left-radius: 8px;
            }

            &:last-of-type {
                border-bottom-right-radius: 8px;
                border-bottom-left-radius: 8px;
            }

            &:hover {
                background: var(--c-grey-2);
            }
        }
    }
}
</style>

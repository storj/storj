// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="size-changer">
        <span class="size-changer__label">Show rows</span>
        <div class="size-changer__selector">
            <div tabindex="0" class="size-changer__selector__content" @keyup.enter="toggleSelector" @click.stop="toggleSelector">
                <span v-if="selected" class="size-changer__selector__content__label">{{ selected }}</span>
                <span v-else class="size-changer__selector__content__label">Size</span>
                <arrow-down-icon class="size-changer__selector__content__arrow" :class="{ open: isOpen }" />
            </div>
            <div v-if="isOpen" v-click-outside="closeSelector" class="size-changer__selector__dropdown">
                <div
                    v-for="(option, index) in options"
                    :key="index"
                    tabindex="0"
                    class="size-changer__selector__dropdown__item"
                    :class="{ selected: isSelected(option.value) }"
                    @click.stop="() => select(option.value)"
                    @keyup.enter="() => select(option.value)"
                >
                    <span class="size-changer__selector__dropdown__item__label">{{ option.label }}</span>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';

import ArrowDownIcon from '@/../static/images/common/dropIcon.svg';

const appStore = useAppStore();

const props = defineProps<{
  selected: number | null;
  itemCount: number;
}>();

const emit = defineEmits<{
  (e: 'change', size: number): void
}>();

const options = computed((): {label:string, value:number}[] => {
    const opts = [
        { label: '10', value: 10 },
        { label: '25', value: 25 },
        { label: '50', value: 50 },
        { label: '100', value: 100 },
    ];
    if (props.itemCount < 1000) {
        return [{ label: 'All', value: props.itemCount }, ...opts];
    }
    return opts;
});

/**
 * whether the selector drop down is open
 * */
const isOpen = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.PAGE_SIZE_SELECTOR;
});

/**
 * whether a size is currently selected.
 * @param size
 * */
function isSelected(size: number): boolean {
    if (!props.selected) {
        return false;
    }
    return props.selected === size;
}

/**
 * select sends the new selection to a parent component.
 * @param size the new selection
 * */
function select(size: number) {
    emit('change', size);
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
        appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.PAGE_SIZE_SELECTOR);
    }
}
</script>

<style scoped lang="scss">
.size-changer {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    font-family: 'font-medium', sans-serif;
    color: var(--c-grey-6);

    &__label {
        margin-right: 10px;

        @media only screen and (width <= 768px) {
            display: none;
        }
    }

    &__selector {
        border: 1px solid var(--c-grey-3);
        width: 65px;
        border-radius: 6px;
        box-sizing: border-box;

        &__content {
            display: flex;
            align-items: center;
            justify-content: space-between;
            position: relative;
            padding: 5px 14px;

            &__label {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                line-height: 20px;
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
            top: -190px;
            background: var(--c-white);
            z-index: 999;
            box-sizing: border-box;
            box-shadow: 0 -2px 16px rgb(0 0 0 / 10%);
            border-radius: 8px;
            border: 1px solid var(--c-grey-2);
            width: 60px;

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
}
</style>

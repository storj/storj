// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="toggle">
        <label class="toggle__input-container">
            <input :id="`checkbox${label}`" :checked="checked" type="checkbox" @change="onCheck">
            <span />
        </label>
        <label class="toggle__label" :for="`checkbox${label}`">{{ label }}</label>
        <VInfo v-if="slots.infoMessage" class="toggle__info">
            <template #icon>
                <InfoIcon class="toggle__info__icon" />
            </template>
            <template #message>
                <slot name="infoMessage" />
            </template>
        </VInfo>
    </div>
</template>

<script setup lang="ts">
import { useSlots } from 'vue';

import VInfo from '@/components/common/VInfo.vue';

import InfoIcon from '@/../static/images/accessGrants/newCreateFlow/info.svg';

const slots = useSlots();

const props = withDefaults(defineProps<{
    checked: boolean;
    label: string;
    onCheck: () => void;
}>(), {
    checked: false,
    label: '',
    onCheck: () => {},
});
</script>

<style scoped lang="scss">
.toggle {
    display: flex;
    align-items: center;
    font-family: 'font_regular', sans-serif;

    &__input-container {
        display: block;
        position: relative;
        height: 16px;
        width: 16px;
        cursor: pointer;

        input {
            position: absolute;
            opacity: 0;
            cursor: pointer;
            height: 0;
            width: 0;
        }

        span {
            position: absolute;
            top: 0;
            left: 0;
            height: 16px;
            width: 16px;
            border: 1px solid #c8d3de;
            border-radius: 4px;
            box-sizing: border-box;

            &:after {
                content: '';
                position: absolute;
                display: none;
                left: 3.5px;
                top: 1px;
                width: 3px;
                height: 7px;
                border: solid white;
                border-width: 0 2px 2px 0;
                transform: rotate(45deg);
            }
        }

        input:checked ~ span:after {
            display: block;
        }

        input:checked ~ span {
            border: 2px solid #376fff;
            background-color: #376fff;
        }

        &:hover {

            input:checked ~ span {
                border: 2px solid #376fff;
                background-color: #376fff;
            }
        }
    }

    &__label {
        margin-left: 8px;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);
        cursor: pointer;
    }

    &__info {
        margin-left: 8px;
        max-height: 16px;

        &__icon {
            cursor: pointer;
        }
    }
}

:deep(.info__box) {
    width: 270px;
    left: calc(50% - 135px);
    top: unset;
    bottom: 15px;
    cursor: default;
    filter: none;
    transform: rotate(-180deg);
}

:deep(.info__box__message) {
    background: var(--c-grey-6);
    border-radius: 4px;
    padding: 10px 8px;
    transform: rotate(-180deg);
}

:deep(.info__box__arrow) {
    background: var(--c-grey-6);
    width: 10px;
    height: 10px;
    margin-bottom: -3px;
}
</style>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="toggle">
        <label class="toggle__input-container">
            <input :id="id || label" :checked="checked" :disabled="disabled" type="checkbox" @change="onCheck">
            <span />
        </label>
        <label class="toggle__label" :for="id || label">{{ label }}</label>
        <template v-if="onShowHideAll">
            <ChevronIcon
                tabindex="0"
                class="toggle__chevron"
                :class="{'toggle__chevron--up': allShown}"
                @click="onShowHideAll"
                @keyup.space="onShowHideAll"
            />
        </template>
        <VInfo v-if="slots.infoMessage" class="toggle__info">
            <template #icon="{onSpace}">
                <InfoIcon tabindex="0" class="toggle__info__icon" @keyup.space="onSpace" />
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
import ChevronIcon from '@/../static/images/accessGrants/newCreateFlow/chevron.svg';

const slots = useSlots();

const props = withDefaults(defineProps<{
    checked: boolean;
    label: string;
    onCheck: () => void;
    id?: string;
    onShowHideAll?: () => void;
    allShown?: boolean;
    disabled?: boolean;
}>(), {
    checked: false,
    label: '',
    id: '',
    onCheck: () => {},
    onShowHideAll: undefined,
    disabled: false,
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

            &:focus + span {
                outline: 2px solid #376fff;
            }
        }

        span {
            position: absolute;
            top: 0;
            left: 0;
            height: 16px;
            width: 16px;
            border: 1px solid var(--c-grey-4);
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
                border: solid var(--c-white);
                border-width: 0 2px 2px 0;
                transform: rotate(45deg);
            }
        }

        input:checked ~ span:after {
            display: block;
        }

        input:checked ~ span {
            border: 2px solid var(--c-light-blue-5);
            background-color: var(--c-light-blue-5);
        }

        &:hover {

            input:checked ~ span {
                border: 2px solid var(--c-light-blue-5);
                background-color: var(--c-light-blue-5);
            }
        }
    }

    &__label {
        margin-left: 8px;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);
        cursor: pointer;
        text-align: left;
    }

    &__info {
        margin-left: 8px;
        max-height: 16px;

        &__icon {
            cursor: pointer;
        }
    }

    &__chevron {
        transition: transform 0.3s;
        margin-left: 8px;
        cursor: pointer;

        &--up {
            transform: rotate(180deg);
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

    @media screen and (max-width: 460px) {
        left: unset;
        right: -83px;
    }
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

    @media screen and (max-width: 460px) {
        margin-right: 88px;
    }
}
</style>

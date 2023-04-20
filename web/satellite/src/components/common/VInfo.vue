// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info" @mouseenter="toggleVisibility" @mouseleave="toggleVisibility">
        <slot name="icon" :on-space="toggleVisibility" />
        <div v-if="isVisible" class="info__box">
            <div class="info__box__arrow" />
            <div class="info__box__message">
                <h1 v-if="title" class="info__box__message__title">{{ title }}</h1>
                <slot name="message" />
                <VButton
                    v-if="buttonLabel"
                    class="info__box__message__button"
                    :label="buttonLabel"
                    height="42px"
                    border-radius="52px"
                    :is-uppercase="true"
                    :on-press="onClick"
                />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import VButton from '@/components/common/VButton.vue';

const props = withDefaults(defineProps<{
    title?: string;
    buttonLabel?: string;
    onButtonClick?: () => unknown;
}>(), {
    title: '',
    buttonLabel: '',
    onButtonClick: () => () => false,
});

const isVisible = ref<boolean>(false);

/**
 * Toggles bubble visibility.
 */
function toggleVisibility(): void {
    isVisible.value = !isVisible.value;
}

/**
 * Holds on button click logic.
 */
function onClick(): void {
    props.onButtonClick();
    toggleVisibility();
}
</script>

<style scoped lang="scss">
    .info {
        position: relative;

        &__box {
            position: absolute;
            top: calc(100% + 10px);
            left: calc(50% + 1px);
            transform: translate(-50%);
            display: flex;
            flex-direction: column;
            align-items: center;
            filter: drop-shadow(0 0 34px #0a1b2c47);
            z-index: 1;

            &__arrow {
                background-color: white;
                width: 40px;
                height: 40px;
                border-radius: 4px 0 0;
                transform: scale(1, 0.85) translate(0, 20%) rotate(45deg);
                margin-bottom: -15px;
            }

            &__message {
                box-sizing: border-box;
                background-color: white;
                padding: 24px;
                border-radius: 20px;

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 14px;
                    line-height: 32px;
                    color: #000;
                    margin-bottom: 10px;
                }

                &__button {
                    margin-top: 20px;
                }
            }
        }
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info" @mouseenter="toggleVisibility" @mouseleave="toggleVisibility">
        <slot />
        <div v-if="isVisible" class="info__message-box">
            <div class="info__message-box__text">
                <p class="info__message-box__text__regular-text">{{ text }}</p>
                <p class="info__message-box__text__bold-text">{{ boldText }}</p>
                <p class="info__message-box__text__bold-text">{{ extraBoldText }}</p>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

withDefaults(defineProps<{
    text?: string;
    boldText?: string;
    extraBoldText?: string;
}>(), {
    text: '',
    boldText: '',
    extraBoldText: '',
});

const isVisible = ref<boolean>(false);

function toggleVisibility(): void {
    isVisible.value = !isVisible.value;
}
</script>

<style scoped lang="scss">
    p {
        margin-block: 0;
    }

    .info {
        position: relative;

        &__message-box {
            position: absolute;
            left: 50%;
            transform: translate(-50%);
            height: auto;
            width: auto;
            white-space: nowrap;
            display: flex;
            justify-content: space-between;
            align-items: center;
            text-align: center;
            background-image: var(--info-image-arrow-middle-path);
            background-size: 100% 100%;
            z-index: 101;
            padding: 11px 18px 20px;

            &__text {
                display: flex;
                flex-direction: column;
                align-items: center;
                justify-content: center;

                &__bold-text {
                    color: var(--regular-text-color);
                    font-size: 12px;
                    line-height: 16px;
                    font-family: 'font_bold', sans-serif;
                }

                &__regular-text {
                    color: var(--regular-text-color);
                    font-size: 12px;
                    line-height: 16px;
                }
            }
        }
    }

    @media screen and (width <= 500px) {

        .info__message-box {
            display: none;
        }
    }
</style>

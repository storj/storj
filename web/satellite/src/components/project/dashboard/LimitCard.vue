// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="card">
        <div class="card__header">
            <component :is="icon" />
            <h2 class="card__header__title">{{ title }}</h2>
        </div>
        <VLoader v-if="isLoading" />
        <template v-else>
            <div class="card__track">
                <div class="card__track__fill" :style="style" />
            </div>
            <div class="card__data">
                <div>
                    <h2 class="card__data__title">{{ usedTitle }}</h2>
                    <p class="card__data__info">{{ usedInfo }}</p>
                </div>
                <div>
                    <h2 class="card__data__title">{{ availableTitle }}</h2>
                    <p v-if="useAction" class="card__data__action" @click="onAction">{{ actionTitle }}</p>
                    <a
                        v-else
                        class="card__data__action"
                        target="_blank"
                        rel="noopener noreferrer"
                        :href="link"
                    >
                        {{ actionTitle }}
                    </a>
                </div>
            </div>
        </template>
    </div>
</template>

<script setup lang="ts">
import { computed, VueConstructor } from 'vue';

import VLoader from '@/components/common/VLoader.vue';

const props = withDefaults(defineProps<{
    icon: VueConstructor
    title: string
    color: string
    usedValue: number
    usedTitle: string
    usedInfo: string
    availableTitle: string
    actionTitle: string
    onAction: () => void
    isLoading: boolean
    isDark?: boolean
    useAction?: boolean
    link?: string
}>(), {
    isDark: false,
    useAction: false,
    link: '',
});

/**
 * Returns progress bar styling which depends on provided prop values.
 */
const style = computed((): Record<string, string> => {
    let color = '';
    switch (true) {
    case props.isDark:
        color = '#091c45';
        break;
    case props.usedValue >= 80 && props.usedValue < 100:
        color = '#ff8a00';
        break;
    case props.usedValue >= 100:
        color = '#ff458b';
        break;
    default:
        color = props.color;
    }

    return {
        width: `${props.usedValue}%`,
        'background-color': color,
    };
});
</script>

<style scoped lang="scss">
.card {
    font-family: 'font_regular', sans-serif;
    padding: 24px;
    background-color: var(--c-white);
    box-shadow: 0 0 20px rgb(0 0 0 / 4%);
    border-radius: 10px;

    &__header {
        display: flex;
        align-items: center;
        margin-bottom: 16px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 18px;
            line-height: 27px;
            color: var(--c-black);
            margin-left: 8px;
        }
    }

    &__track {
        width: 100%;
        height: 6px;
        background: var(--c-grey-3);
        border-radius: 100px;
        position: relative;
        margin-bottom: 16px;

        &__fill {
            max-width: 100%;
            position: absolute;
            border-radius: 100px;
            top: 0;
            bottom: 0;
            left: 0;
        }
    }

    &__data {
        display: flex;
        align-items: center;
        justify-content: space-between;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 18px;
            line-height: 27px;
            color: var(--c-black);
        }

        &__info {
            font-size: 14px;
            line-height: 22px;
            color: var(--c-grey-6);
        }

        &__action {
            display: block;
            font-size: 14px;
            line-height: 22px;
            color: var(--c-grey-6);
            text-align: right;
            text-decoration: underline !important;
            cursor: pointer;
        }
    }
}
</style>

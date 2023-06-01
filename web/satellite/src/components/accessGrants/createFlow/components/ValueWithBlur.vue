// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="blured-container">
        <div v-if="!isMnemonic" class="blured-container__header">
            <h2 class="blured-container__header__title">{{ title }}</h2>
            <VInfo v-if="info" class="blured-container__header__info">
                <template #icon>
                    <InfoIcon class="blured-container__header__info__icon" />
                </template>
                <template #message>
                    <p class="blured-container__header__info__text">{{ info }}</p>
                </template>
            </VInfo>
        </div>
        <div class="blured-container__wrap" :class="{justify: !isMnemonic}">
            <p v-if="isMnemonic" tabindex="0" class="blured-container__wrap__mnemonic" @keyup.space="onCopy">{{ value }}</p>
            <p v-else tabindex="0" class="blured-container__wrap__text" @keyup.space="onCopy">{{ value }}</p>
            <div
                v-if="!isMnemonic"
                tabindex="0"
                class="blured-container__wrap__copy"
                @click="onCopy"
                @keyup.space="onCopy"
            >
                <CopyIcon />
            </div>
            <div v-if="!isValueShown" class="blured-container__wrap__blur">
                <VButton
                    :label="buttonLabel"
                    icon="lock"
                    width="159px"
                    height="40px"
                    font-size="12px"
                    :is-white="true"
                    :on-press="showValue"
                />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useNotify } from '@/utils/hooks';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';

import VButton from '@/components/common/VButton.vue';
import VInfo from '@/components/common/VInfo.vue';

import InfoIcon from '@/../static/images/accessGrants/newCreateFlow/info.svg';
import CopyIcon from '@/../static/images/accessGrants/newCreateFlow/copy.svg';

const props = withDefaults(defineProps<{
    isMnemonic: boolean;
    value: string;
    buttonLabel: string;
    title?: string;
    info?: string;
}>(), {
    title: '',
    info: '',
});

const notify = useNotify();

const isValueShown = ref<boolean>(false);

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Makes blurred value to be shown.
 */
function showValue(): void {
    isValueShown.value = true;
}

/**
 * Holds on copy click logic.
 */
function onCopy(): void {
    navigator.clipboard.writeText(props.value);
    analytics.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);
    notify.success(`${props.title} was copied successfully`);
}
</script>

<style scoped lang="scss">
.blured-container {
    font-family: 'font_regular', sans-serif;

    &__header {
        display: flex;
        align-items: center;
        margin-bottom: 16px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-grey-6);
        }

        &__info {
            margin-left: 8px;
            max-height: 16px;

            &__icon {
                cursor: pointer;
            }

            &__text {
                color: var(--c-white);
            }
        }
    }

    &__wrap {
        padding: 16px;
        background: var(--c-grey-2);
        border: 1px solid var(--c-grey-2);
        border-radius: 10px;
        position: relative;
        display: flex;
        align-items: center;

        &__mnemonic {
            font-size: 14px;
            line-height: 26px;
            color: var(--c-black);
        }

        &__text {
            font-size: 14px;
            line-height: 20px;
            color: var(--c-grey-7);
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
            margin-right: 16px;
        }

        &__copy {
            min-width: 16px;
            cursor: pointer;
        }

        &__blur {
            position: absolute;
            inset: 0;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 10px;
            backdrop-filter: blur(10px);
        }
    }
}

.justify {
    justify-content: space-between;
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

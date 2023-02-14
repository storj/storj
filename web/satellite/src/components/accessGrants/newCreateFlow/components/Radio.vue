// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="radio">
        <input :id="`checkbox${label}`" :checked="checked" type="radio" @change="onCheck">
        <label class="radio__label" :for="`checkbox${label}`">{{ label }}</label>
        <VInfo class="radio__info">
            <template #icon>
                <InfoIcon class="radio__info__icon" />
            </template>
            <template #message>
                <p class="radio__info__message">{{ info }}</p>
            </template>
        </VInfo>
    </div>
</template>

<script setup lang="ts">
import VInfo from '@/components/common/VInfo.vue';

import InfoIcon from '@/../static/images/accessGrants/newCreateFlow/info.svg';

const props = defineProps<{
    checked: boolean;
    label: string;
    onCheck: () => void;
    info: string;
}>();
</script>

<style scoped lang="scss">
.radio {
    display: flex;
    align-items: center;
    font-family: 'font_regular', sans-serif;

    input {
        cursor: pointer;
        height: 16px;
        width: 16px;
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

        &__message {
            color: #fff;
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

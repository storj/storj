// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="manage-options">
        <div class="manage-options__option" @click="setCreate">
            <PlusIcon class="manage-options__option__icon" />
            <div class="manage-options__option__info">
                <h2 class="manage-options__option__info__title">Create a new passphrase</h2>
                <p class="manage-options__option__info__msg">Allows you to upload data with a different passphrase.</p>
            </div>
            <ChevronIcon class="manage-options__option__arrow" />
        </div>
        <div class="manage-options__option middle" @click="setSwitch">
            <SwitchIcon class="manage-options__option__icon" />
            <div class="manage-options__option__info">
                <h2 class="manage-options__option__info__title">Switch active passphrase</h2>
                <p class="manage-options__option__info__msg">View and upload data using another passphrase. </p>
            </div>
            <ChevronIcon class="manage-options__option__arrow" />
        </div>
        <div class="manage-options__option" @click="setClear">
            <ClearIcon class="manage-options__option__icon" />
            <div class="manage-options__option__info">
                <h2 class="manage-options__option__info__title">Clear saved passphrase</h2>
                <p class="manage-options__option__info__msg">Lock your data and clear passphrase from this session.</p>
            </div>
            <ChevronIcon class="manage-options__option__arrow" />
        </div>
        <div class="manage-options__divider" />
        <VButton
            label="Cancel"
            width="100%"
            height="48px"
            font-size="14px"
            border-radius="10px"
            :on-press="onCancel"
            :is-white="true"
        />
    </div>
</template>

<script setup lang="ts">
import VButton from '@/components/common/VButton.vue';

import PlusIcon from '@/../static/images/projectPassphrase/plus.svg';
import SwitchIcon from '@/../static/images/projectPassphrase/switch.svg';
import ClearIcon from '@/../static/images/projectPassphrase/clear.svg';
import ChevronIcon from '@/../static/images/projectPassphrase/chevron.svg';

const props = withDefaults(defineProps<{
    setCreate?: () => void,
    setSwitch?: () => void,
    setClear?: () => void,
    onCancel?: () => void,
}>(), {
    setCreate: () => () => {},
    setSwitch: () => () => {},
    setClear: () => () => {},
    onCancel: () => () => {},
});
</script>

<style scoped lang="scss">
.manage-options {
    display: flex;
    flex-direction: column;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    max-width: 350px;

    &__option {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 14px 20px;
        width: calc(100% - 40px);
        background: #fafafb;
        border: 1px solid #d8dee3;
        border-radius: 10px;
        cursor: pointer;

        &__icon {
            min-width: 32px;
            min-height: 32px;
        }

        &__info {
            margin: 0 8px 0 16px;
            width: 100%;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: #000;
                margin-bottom: 8px;
                text-align: left;
            }

            &__msg {
                font-size: 12px;
                line-height: 18px;
                color: #000;
                text-align: left;
            }
        }

        &__arrow {
            min-height: 20px;
            min-width: 20px;
        }

        &:hover {
            background-color: #fff;
            border-color: #0149ff;

            h2 {
                color: #0d6efd;
            }

            :deep(path) {
                fill: #0149ff;
            }
        }
    }

    &__divider {
        width: 100%;
        height: 1px;
        margin: 16px 0;
        background-color: var(--c-grey-2);
    }
}

.middle {
    margin: 18px 0;
}
</style>

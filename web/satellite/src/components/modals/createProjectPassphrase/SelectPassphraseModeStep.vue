// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="passphrase-mode">
        <LockIcon />
        <h1 class="passphrase-mode__title">Encryption Passphrase</h1>
        <p class="passphrase-mode__info">
            The encryption passphrase will be used to encrypt all the files you upload in this project. We encourage
            you to generate the encryption passphrase. You can also enter your own passphrase.
        </p>
        <div
            class="passphrase-mode__option"
            :class="{selected: isGenerate}"
            @click="setGenerate"
        >
            <div
                class="passphrase-mode__option__check"
                :class="{'selected-check': isGenerate}"
            >
                <CheckIcon />
            </div>
            <div class="passphrase-mode__option__info">
                <h2 class="passphrase-mode__option__info__title">
                    Generate passphrase
                </h2>
                <p class="passphrase-mode__option__info__msg">
                    Automatically generate 12-word passphrase.
                </p>
            </div>
        </div>
        <div
            class="passphrase-mode__option"
            :class="{selected: !isGenerate}"
            @click="setEnter"
        >
            <div
                class="passphrase-mode__option__check"
                :class="{'selected-check': !isGenerate}"
            >
                <CheckIcon />
            </div>
            <div class="passphrase-mode__option__info">
                <h2 class="passphrase-mode__option__info__title">
                    Enter passphrase
                </h2>
                <p class="passphrase-mode__option__info__msg">
                    You can also enter your own passphrase.
                </p>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import LockIcon from '@/../static/images/projectPassphrase/lock.svg';
import CheckIcon from '@/../static/images/projectPassphrase/check.svg';

const props = withDefaults(defineProps<{
    isGenerate?: boolean,
    setGenerate?: () => void
    setEnter?: () => void
}>(), {
    isGenerate: true,
    setGenerate: () => () => {},
    setEnter: () => () => {},
});
</script>

<style scoped lang="scss">
.passphrase-mode {
    display: flex;
    flex-direction: column;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    max-width: 433px;

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 32px;
        line-height: 39px;
        color: #1b2533;
        margin: 14px 0;
    }

    &__info {
        font-size: 14px;
        line-height: 19px;
        color: #354049;
        margin-bottom: 24px;
    }

    &__option {
        padding: 14px 20px;
        display: flex;
        align-items: center;
        border: 1px solid #d8dee3;
        background-color: #fafafb;
        border-radius: 10px;
        margin-bottom: 9px;
        width: calc(100% - 40px);
        cursor: pointer;

        &__check {
            min-width: 32px;
            min-height: 32px;
            background-color: #fff;
            border: 1px solid #c8d3de;
            border-radius: 16px;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        &__info {
            margin-left: 16px;
            align-items: flex-start;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: #091c45;
                text-align: left;
                margin-bottom: 8px;
            }

            &__msg {
                font-size: 12px;
                line-height: 18px;
                color: #091c45;
                text-align: left;
            }
        }

        &:hover {
            background-color: #fff;
            border-color: #0149ff;
        }
    }
}

.selected {
    background-color: #fff;
    border-color: #929fb1;

    &:hover {
        border-color: #929fb1;
    }
}

.selected-check {
    border-color: #00ac26;
    background-color: #00ac26;
}
</style>

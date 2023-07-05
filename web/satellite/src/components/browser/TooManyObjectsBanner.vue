// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="banner">
        <div class="banner__left">
            <LockedIcon class="banner__left__icon" />
            <p class="banner__left__title">
                Due to the number of objects you have uploaded, some files may not be displayed.
                To list all objects you can use
                <a
                    class="banner__left__title__link"
                    href="https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/prerequisites"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Uplink CLI
                </a>
                tool.
            </p>
        </div>
        <CloseIcon class="banner__close" @click="onClose" />
    </div>
</template>

<script setup lang="ts">
import LockedIcon from '@/../static/images/browser/locked.svg';
import CloseIcon from '@/../static/images/browser/close.svg';

const props = defineProps<{
    onClose: () => void
}>();
</script>

<style scoped lang="scss">
.banner {
    padding: 16px;
    background: #fec;
    border: 1px solid var(--c-yellow-2);
    box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-family: 'font_regular', sans-serif;
    margin-bottom: 21px;

    &__left {
        display: flex;
        align-items: center;
        margin-right: 15px;

        &__icon {
            margin-right: 16px;
            min-width: 32px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-black);

            &__link {
                color: var(--c-blue-3);

                &:visited {
                    color: var(--c-blue-3);
                }
            }
        }
    }

    &__close {
        min-width: 12px;
        cursor: pointer;
    }
}
</style>

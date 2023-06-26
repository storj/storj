// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="banner">
        <div class="banner__left">
            <LockedIcon class="banner__left__icon" />
            <div class="banner__left__labels">
                <h2 class="banner__left__labels__title">
                    You have at least {{ lockedFilesCount }} object{{ lockedFilesCount > 1 ? 's' : '' }} locked with a
                    different passphrase.
                </h2>
                <p class="banner__left__labels__subtitle">Enter your other passphrase to access these files.</p>
            </div>
        </div>
        <div class="banner__right">
            <p class="banner__right__unlock" @click="openManageModal">
                Unlock now
            </p>
            <CloseIcon class="banner__right__close" @click="onClose" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';

import LockedIcon from '@/../static/images/browser/locked.svg';
import CloseIcon from '@/../static/images/browser/close.svg';

const props = defineProps<{
    lockedFilesCount: number
    onClose: () => void
}>();

const appStore = useAppStore();

/**
 * Opens switch passphrase modal.
 */
function openManageModal(): void {
    appStore.setManagePassphraseStep(ManageProjectPassphraseStep.Switch);
    appStore.updateActiveModal(MODALS.manageProjectPassphrase);
}
</script>

<style scoped lang="scss">
.banner {
    padding: 16px;
    background: #fec;
    border: 1px solid #ffd78a;
    box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-family: 'font_regular', sans-serif;
    margin-bottom: 21px;

    @media screen and (width <= 400px) {
        flex-direction: column-reverse;
    }

    &__left {
        display: flex;
        align-items: center;
        margin-right: 15px;

        @media screen and (width <= 600px) {
            flex-direction: column;
            align-items: flex-start;
        }

        &__icon {
            min-width: 32px;
        }

        &__labels {
            margin-left: 16px;

            @media screen and (width <= 600px) {
                margin: 10px 0 0;
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: #000;
            }

            &__subtitle {
                font-size: 14px;
                line-height: 20px;
                color: #000;
            }
        }
    }

    &__right {
        display: flex;
        align-items: center;

        @media screen and (width <= 400px) {
            width: 100%;
            justify-content: space-between;
            margin-bottom: 10px;
        }

        &__unlock {
            font-size: 14px;
            line-height: 22px;
            color: #000;
            text-decoration: underline;
            text-underline-position: under;
            cursor: pointer;
            margin-right: 16px;
            white-space: nowrap;
        }

        &__close {
            min-width: 12px;
            cursor: pointer;
        }
    }
}
</style>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="banner">
        <div class="banner__left">
            <LockedIcon class="banner__left__icon" />
            <div class="banner__left__labels">
                <template v-if="objectsCount <= NUMBER_OF_DISPLAYED_OBJECTS">
                    <h2 class="banner__left__labels__title">
                        You have at least {{ lockedFilesNumber }} object{{ lockedFilesNumber > 1 ? 's' : '' }} locked with a
                        different passphrase.
                    </h2>
                    <p class="banner__left__labels__subtitle">Enter your other passphrase to access these files.</p>
                </template>
                <template v-else>
                    <h2 class="banner__left__labels__title">
                        Due to the number of objects you have uploaded to this bucket, {{ lockedFilesNumber }} files are
                        not displayed.
                    </h2>
                </template>
            </div>
        </div>
        <div class="banner__right">
            <p v-if="objectsCount <= NUMBER_OF_DISPLAYED_OBJECTS" class="banner__right__unlock" @click="openManageModal">
                Unlock now
            </p>
            <CloseIcon class="banner__right__close" @click="onClose" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { Bucket } from '@/types/buckets';
import { useStore } from '@/utils/hooks';
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';

import LockedIcon from '@/../static/images/browser/locked.svg';
import CloseIcon from '@/../static/images/browser/close.svg';

const props = withDefaults(defineProps<{
    onClose?: () => void;
}>(), {
    onClose: () => {},
});

const appStore = useAppStore();
const store = useStore();

const NUMBER_OF_DISPLAYED_OBJECTS = 1000;

/**
 * Returns locked files number.
 */
const lockedFilesNumber = computed((): number => {
    const ownObjectsCount = store.state.files.objectsCount;

    return objectsCount.value - ownObjectsCount;
});

/**
 * Returns bucket objects count from store.
 */
const objectsCount = computed((): number => {
    const name: string = store.state.files.bucket;
    const data: Bucket | undefined = store.state.bucketUsageModule.page.buckets.find((bucket: Bucket) => bucket.name === name);

    return data?.objectCount || 0;
});

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

    @media screen and (max-width: 400px) {
        flex-direction: column-reverse;
    }

    &__left {
        display: flex;
        align-items: center;
        margin-right: 15px;

        @media screen and (max-width: 600px) {
            flex-direction: column;
            align-items: flex-start;
        }

        &__icon {
            min-width: 32px;
        }

        &__labels {
            margin-left: 16px;

            @media screen and (max-width: 600px) {
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

        @media screen and (max-width: 400px) {
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

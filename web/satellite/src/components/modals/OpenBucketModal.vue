// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <OpenBucketIcon />
                <h1 class="modal__title">Enter your encryption passphrase</h1>
                <p class="modal__info">
                    To open a bucket and view your encrypted files, <br>please enter your encryption passphrase.
                </p>
                <VInput
                    :class="{'orange-border': isWarningState}"
                    label="Encryption Passphrase"
                    placeholder="Enter a passphrase here"
                    :error="enterError"
                    role-description="passphrase"
                    is-password
                    :disabled="isLoading"
                    @setData="setPassphrase"
                />
                <div v-if="isWarningState" class="modal__warning">
                    <OpenWarningIcon class="modal__warning__icon" />
                    <div class="modal__warning__info">
                        <p class="modal__warning__info__title">
                            This bucket includes files that are uploaded using a different encryption passphrase from
                            the one you entered.
                        </p>
                    </div>
                </div>
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        height="48px"
                        :is-transparent="true"
                        :on-press="closeModal"
                        :is-disabled="isLoading"
                    />
                    <VButton
                        :label="isWarningState ? 'Continue Anyway ->' : 'Continue ->'"
                        height="48px"
                        :on-press="onContinue"
                        :is-disabled="isLoading"
                        :is-orange="isWarningState"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Bucket } from '@/types/buckets';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify, useRouter } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import VModal from '@/components/common/VModal.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import OpenBucketIcon from '@/../static/images/buckets/openBucket.svg';
import OpenWarningIcon from '@/../static/images/objects/openWarning.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const notify = useNotify();

const NUMBER_OF_DISPLAYED_OBJECTS = 1000;
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const enterError = ref<string>('');
const passphrase = ref<string>('');
const isLoading = ref<boolean>(false);
const isWarningState = ref<boolean>(false);

/**
 * Returns chosen bucket name from store.
 */
const bucketName = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Returns selected bucket name object count.
 */
const bucketObjectCount = computed((): number => {
    const data: Bucket | undefined = bucketsStore.state.page.buckets.find(
        (bucket: Bucket) => bucket.name === bucketName.value,
    );

    return data?.objectCount || 0;
});

/**
 * Sets access and navigates to object browser.
 */
async function onContinue(): Promise<void> {
    if (isLoading.value) return;

    if (isWarningState.value) {
        bucketsStore.setPromptForPassphrase(false);

        closeModal();
        analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
        await router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);

        return;
    }

    if (!passphrase.value) {
        enterError.value = 'Passphrase can\'t be empty';
        analytics.errorEventTriggered(AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);

        return;
    }

    isLoading.value = true;

    try {
        bucketsStore.setPassphrase(passphrase.value);
        await bucketsStore.setS3Client(projectsStore.state.selectedProject.id);
        const count: number = await bucketsStore.getObjectsCount(bucketName.value);
        if (bucketObjectCount.value > count && bucketObjectCount.value <= NUMBER_OF_DISPLAYED_OBJECTS) {
            isWarningState.value = true;
            isLoading.value = false;
            return;
        }
        bucketsStore.setPromptForPassphrase(false);
        isLoading.value = false;

        closeModal();
        analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
        await router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);
        isLoading.value = false;
    }
}

/**
 * Closes open bucket modal.
 */
function closeModal(): void {
    if (isLoading.value) return;

    appStore.updateActiveModal(MODALS.openBucket);
}

/**
 * Sets passphrase from child component.
 */
function setPassphrase(value: string): void {
    if (enterError.value) enterError.value = '';
    if (isWarningState.value) isWarningState.value = false;

    passphrase.value = value;
}
</script>

<style scoped lang="scss">
    .modal {
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 62px 62px 54px;
        max-width: 500px;

        @media screen and (max-width: 600px) {
            padding: 62px 24px 54px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 26px;
            line-height: 31px;
            color: #131621;
            margin: 30px 0 15px;
        }

        &__info {
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #354049;
            margin-bottom: 32px;
        }

        &__warning {
            max-width: 405px;
            padding: 16px;
            display: flex;
            align-items: flex-start;
            background: #fec;
            border: 1px solid #ffd78a;
            box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
            border-radius: 10px;
            margin-top: 22px;

            &__icon {
                min-width: 32px;
            }

            &__info {
                margin-left: 16px;

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                    color: #000;
                    text-align: left;
                }
            }
        }

        &__buttons {
            display: flex;
            column-gap: 20px;
            margin-top: 31px;
            width: 100%;

            @media screen and (max-width: 500px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 20px;
            }
        }
    }

    .orange-border {

        :deep(input) {
            border-color: #ff8a00;
        }
    }
</style>

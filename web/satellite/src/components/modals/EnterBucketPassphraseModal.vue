// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <AccessEncryptionIcon />
                    <h1 class="modal__header__title">Enter passphrase</h1>
                </div>
                <p class="modal__info">
                    Enter your encryption passphrase to view and manage your data in the browser. This passphrase will
                    be used to unlock all buckets in this project.
                </p>
                <VInput
                    :class="{'orange-border': isWarningState}"
                    label="Encryption Passphrase"
                    placeholder="Enter a passphrase here"
                    :error="enterError"
                    role-description="passphrase"
                    is-password
                    :autocomplete="autocompleteValue"
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
                        height="52px"
                        font-size="14px"
                        border-radius="10px"
                        :is-transparent="true"
                        :on-press="closeModal"
                        :is-disabled="isLoading"
                    />
                    <VButton
                        label="Continue ->"
                        height="52px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="onContinue"
                        :is-disabled="isLoading || !passphrase"
                        :is-transparent="isWarningState"
                    />
                </div>
                <div v-if="isLoading" class="modal__loading-wrap">
                    <VLoader width="50px" height="50px" is-white />
                    <p class="modal__loading-wrap__label">Counting objects...</p>
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Bucket } from '@/types/buckets';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VModal from '@/components/common/VModal.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';

import AccessEncryptionIcon from '@/../static/images/accessGrants/newCreateFlow/accessEncryption.svg';
import OpenWarningIcon from '@/../static/images/objects/openWarning.svg';

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const notify = useNotify();

const NUMBER_OF_DISPLAYED_OBJECTS = 1000;

const enterError = ref<string>('');
const passphrase = ref<string>('');
const isLoading = ref<boolean>(false);
const isWarningState = ref<boolean>(false);

/**
 * Returns formatted autocomplete value.
 */
const autocompleteValue = computed((): string => {
    return `section-${selectedProjectID.value.toLowerCase()} new-password`;
});

/**
 * Returns selected project ID from store.
 */
const selectedProjectID = computed((): string => {
    return projectsStore.state.selectedProject.id;
});

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

    const callback = bucketsStore.state.enterPassphraseCallback || ((): void => {
        analyticsStore.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
        router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
    });

    if (isWarningState.value) {
        bucketsStore.setPromptForPassphrase(false);

        closeModal();
        callback();

        return;
    }

    if (!passphrase.value) {
        enterError.value = 'Passphrase can\'t be empty';
        analyticsStore.errorEventTriggered(AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);

        return;
    }

    isLoading.value = true;

    try {
        bucketsStore.setPassphrase(passphrase.value);
        await bucketsStore.setS3Client(selectedProjectID.value);
        const count: number = await bucketsStore.getObjectsCount(bucketName.value);
        if (bucketObjectCount.value > count && bucketObjectCount.value <= NUMBER_OF_DISPLAYED_OBJECTS) {
            isWarningState.value = true;
            isLoading.value = false;
            return;
        }
        bucketsStore.setPromptForPassphrase(false);
        isLoading.value = false;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);
        isLoading.value = false;
        return;
    }

    closeModal();
    callback();
}

/**
 * Closes open bucket modal.
 */
function closeModal(): void {
    if (isLoading.value) return;

    appStore.removeActiveModal();
}

/**
 * Sets passphrase from child component.
 */
function setPassphrase(value: string): void {
    if (enterError.value) enterError.value = '';
    if (isWarningState.value) isWarningState.value = false;

    passphrase.value = value;
}

onUnmounted((): void => {
    bucketsStore.setEnterPassphraseCallback(null);
});
</script>

<style scoped lang="scss">
    .modal {
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        padding: 32px;
        max-width: 350px;
        position: relative;

        &__header {
            display: flex;
            align-items: center;
            padding-bottom: 16px;
            margin-bottom: 16px;
            border-bottom: 1px solid var(--c-grey-2);

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 31px;
                color: var(--c-grey-8);
                margin-left: 16px;
                text-align: left;
            }
        }

        &__info {
            font-size: 14px;
            line-height: 19px;
            color: #354049;
            padding-bottom: 16px;
            margin-bottom: 16px;
            border-bottom: 1px solid var(--c-grey-2);
            text-align: left;
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
                    font-weight: 500;
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

            @media screen and (width <= 500px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 20px;
            }
        }

        &__loading-wrap {
            position: absolute;
            inset: 0;
            background: rgb(27 37 51 / 40%);
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 18px;
                margin-top: 16px;
                color: var(--c-white);
            }
        }
    }

    .orange-border {

        :deep(h3) {
            color: var(--c-yellow-5);
        }

        :deep(input) {
            border-color: var(--c-yellow-5);
        }
    }
</style>

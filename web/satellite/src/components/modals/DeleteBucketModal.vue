// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <h1 class="modal__title">Are you sure?</h1>
                <p class="modal__subtitle">
                    The following bucket will be deleted and all of its data.<br>This action cannot be undone.
                </p>
                <div class="modal__chip">
                    <DeleteBucketIcon />
                    <p class="modal__chip__label">{{ bucketToDelete }}</p>
                </div>
                <VInput
                    class="modal__input"
                    label="Type the name of the bucket to confirm"
                    placeholder="Bucket Name"
                    :is-loading="isLoading"
                    @setData="onChangeName"
                />
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="44px"
                        :is-white="true"
                        :on-press="closeModal"
                    />
                    <VButton
                        label="Delete Bucket"
                        width="100%"
                        height="48px"
                        :on-press="onDelete"
                        :is-disabled="isLoading || !name"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore, FILE_BROWSER_AG_NAME } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

import DeleteBucketIcon from '@/../static/images/buckets/deleteBucket.svg';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const worker = ref<Worker| null>(null);
const name = ref<string>('');
const isLoading = ref<boolean>(false);

/**
 * Returns apiKey from store.
 */
const apiKey = computed<string>(() => bucketsStore.state.apiKey);
const bucketToDelete = computed<string>(() => bucketsStore.state.bucketToDelete);

/**
 * Holds on delete button click logic.
 * Creates unrestricted access grant and deletes bucket.
 */
async function onDelete(): Promise<void> {
    if (!worker.value) {
        return;
    }

    if (isLoading.value) return;

    isLoading.value = true;

    const projectID = projectsStore.state.selectedProject.id;

    try {
        if (!apiKey.value) {
            await agStore.deleteAccessGrantByNameAndProjectID(FILE_BROWSER_AG_NAME, projectID);
            const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(FILE_BROWSER_AG_NAME, projectID);
            bucketsStore.setApiKey(cleanAPIKey.secret);
        }

        const now = new Date();
        const inOneHour = new Date(now.setHours(now.getHours() + 1));

        worker.value.postMessage({
            'type': 'SetPermission',
            'isDownload': false,
            'isUpload': false,
            'isList': true,
            'isDelete': true,
            'notAfter': inOneHour.toISOString(),
            'buckets': JSON.stringify([name.value]),
            'apiKey': apiKey.value,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => {
            if (worker.value) {
                worker.value.onmessage = resolve;
            }
        });
        if (grantEvent.data.error) {
            notify.error(grantEvent.data.error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
            return;
        }

        const salt = await projectsStore.getProjectSalt(projectsStore.state.selectedProject.id);
        const satelliteNodeURL: string = configStore.state.config.satelliteNodeURL;

        worker.value.postMessage({
            'type': 'GenerateAccess',
            'apiKey': grantEvent.data.value,
            'passphrase': '',
            'salt': salt,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const accessGrantEvent: MessageEvent = await new Promise(resolve => {
            if (worker.value) {
                worker.value.onmessage = resolve;
            }
        });
        if (accessGrantEvent.data.error) {
            notify.error(accessGrantEvent.data.error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
            return;
        }

        const accessGrant = accessGrantEvent.data.value;

        const edgeCredentials: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
        bucketsStore.setEdgeCredentialsForDelete(edgeCredentials);
        await bucketsStore.deleteBucket(name.value);
        analyticsStore.eventTriggered(AnalyticsEvent.BUCKET_DELETED);
        await fetchBuckets();
        bucketsStore.setBucketToDelete('');
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
        return;
    } finally {
        isLoading.value = false;
    }

    closeModal();
}

/**
 * Fetches bucket using api.
 */
async function fetchBuckets(page = 1): Promise<void> {
    try {
        await bucketsStore.getBuckets(page, projectsStore.state.selectedProject.id);
    } catch (error) {
        notify.error(`Unable to fetch buckets. ${error.message}`, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
    }
}

/**
 * Sets local worker with worker instantiated in store.
 */
function setWorker(): void {
    worker.value = agStore.state.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
        };
    }
}

/**
 * Sets name from input.
 */
function onChangeName(value: string): void {
    name.value = value;
}

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Lifecycle hook after initial render.
 * Sets local worker.
 */
onMounted(() => {
    setWorker();
});
</script>

<style scoped lang="scss">
.modal {
    padding: 35px;
    border-radius: 10px;
    font-family: 'font_regular', sans-serif;
    display: flex;
    flex-direction: column;
    align-items: center;
    background-color: var(--c-white);
    max-width: 480px;

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 22px;
        line-height: 30px;
        color: var(--c-grey-8);
        margin: 0 0 14px;
    }

    &__subtitle {
        font-size: 16px;
        line-height: 21px;
        text-align: center;
        color: var(--c-grey-8);
        margin: 0 0 20px;
    }

    &__chip {
        max-width: 100%;
        display: flex;
        align-items: center;
        padding: 7px 30px;
        border-radius: 999px;
        margin-bottom: 24px;
        background-color: var(--c-grey-2);

        svg {
            min-width: 18px;
        }

        &__label {
            margin-left: 7px;
            font-family: 'font_bold', sans-serif;
            color: var(--c-blue-6);
            font-size: 14px;
            line-height: 18px;
            word-break: break-word;
        }
    }

    &__buttons {
        display: flex;
        align-items: center;
        width: 100%;
        margin-top: 30px;
        column-gap: 15px;

        @media screen and (width <= 550px) {
            flex-direction: column-reverse;
            column-gap: unset;
            row-gap: 10px;
            margin-top: 15px;
        }
    }
}
</style>

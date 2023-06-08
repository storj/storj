// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <CreateBucketIcon />
                    <h1 class="modal__header__title">Create a Bucket</h1>
                </div>
                <p class="modal__info">
                    Buckets are used to store and organize your files. Enter lowercase alphanumeric characters only,
                    no spaces.
                </p>
                <div class="modal__input-container">
                    <VLoader v-if="bucketNamesLoading" width="100px" height="100px" />
                    <VInput
                        v-else
                        :init-value="bucketName"
                        label="Bucket Name"
                        additional-label="Required"
                        placeholder="Enter bucket name"
                        :error="nameError"
                        @setData="setBucketName"
                    />
                </div>
                <div class="modal__button-container">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="closeModal"
                        :is-transparent="true"
                    />
                    <VButton
                        label="Create bucket ->"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="onCreate"
                        :is-disabled="!bucketName"
                    />
                </div>
                <div v-if="isLoading" class="modal__blur">
                    <VLoader class="modal__blur__loader" width="50px" height="50px" />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { Validator } from '@/utils/validation';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { LocalData } from '@/utils/localData';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore, FILE_BROWSER_AG_NAME } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';

import VLoader from '@/components/common/VLoader.vue';
import VInput from '@/components/common/VInput.vue';
import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';

import CreateBucketIcon from '@/../static/images/buckets/createBucket.svg';

const configStore = useConfigStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const bucketName = ref<string>('');
const nameError = ref<string>('');
const bucketNamesLoading = ref<boolean>(true);
const isLoading = ref<boolean>(false);
const worker = ref<Worker | null>(null);

/**
 * Returns all bucket names from store.
 */
const allBucketNames = computed((): string[] => {
    return bucketsStore.state.allBucketNames;
});

/**
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
});

/**
 * Returns object browser api key from store.
 */
const apiKey = computed((): string => {
    return bucketsStore.state.apiKey;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentials;
});

/**
 * Returns edge credentials for bucket creation from store.
 */
const edgeCredentialsForCreate = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentialsForCreate;
});

/**
 * Indicates if bucket was created.
 */
const bucketWasCreated = computed((): boolean => {
    const status = LocalData.getBucketWasCreatedStatus();
    if (status !== null) {
        return status;
    }

    return false;
});

/**
 * Validates provided bucket's name and creates a bucket.
 */
async function onCreate(): Promise<void> {
    if (isLoading.value) return;

    if (!worker.value) {
        notify.error('Worker is not defined', AnalyticsErrorEventSource.BUCKET_CREATION_NAME_STEP);
        return;
    }

    if (!isBucketNameValid(bucketName.value)) {
        analytics.errorEventTriggered(AnalyticsErrorEventSource.BUCKET_CREATION_NAME_STEP);
        return;
    }

    if (allBucketNames.value.includes(bucketName.value)) {
        notify.error('Bucket with this name already exists', AnalyticsErrorEventSource.BUCKET_CREATION_NAME_STEP);
        return;
    }

    isLoading.value = true;

    try {
        const projectID = projectsStore.state.selectedProject.id;

        if (!promptForPassphrase.value) {
            if (!edgeCredentials.value.accessKeyId) {
                await bucketsStore.setS3Client(projectID);
            }
            await bucketsStore.createBucket(bucketName.value);
            await bucketsStore.getBuckets(1, projectID);
            bucketsStore.setFileComponentBucketName(bucketName.value);

            analytics.eventTriggered(AnalyticsEvent.BUCKET_CREATED);
            analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
            await router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
            closeModal();

            if (!bucketWasCreated.value) {
                LocalData.setBucketWasCreatedStatus();
            }

            return;
        }

        if (edgeCredentialsForCreate.value.accessKeyId) {
            await bucketsStore.createBucketWithNoPassphrase(bucketName.value);
            await bucketsStore.getBuckets(1, projectID);
            analytics.eventTriggered(AnalyticsEvent.BUCKET_CREATED);
            closeModal();

            if (!bucketWasCreated.value) {
                LocalData.setBucketWasCreatedStatus();
            }

            return ;
        }

        if (!apiKey.value) {
            await agStore.deleteAccessGrantByNameAndProjectID(FILE_BROWSER_AG_NAME, projectID);
            const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(FILE_BROWSER_AG_NAME, projectID);
            bucketsStore.setApiKey(cleanAPIKey.secret);
        }

        const now = new Date();
        const inOneHour = new Date(now.setHours(now.getHours() + 1));

        await worker.value.postMessage({
            'type': 'SetPermission',
            'isDownload': false,
            'isUpload': true,
            'isList': false,
            'isDelete': false,
            'notAfter': inOneHour.toISOString(),
            'buckets': JSON.stringify([]),
            'apiKey': apiKey.value,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => {
            if (worker.value) {
                worker.value.onmessage = resolve;
            }
        });
        if (grantEvent.data.error) {
            await notify.error(grantEvent.data.error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
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
            await notify.error(accessGrantEvent.data.error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
            return;
        }

        const accessGrant = accessGrantEvent.data.value;

        const creds: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
        bucketsStore.setEdgeCredentialsForCreate(creds);
        await bucketsStore.createBucketWithNoPassphrase(bucketName.value);
        await bucketsStore.getBuckets(1, projectID);
        analytics.eventTriggered(AnalyticsEvent.BUCKET_CREATED);

        closeModal();

        if (!bucketWasCreated.value) {
            LocalData.setBucketWasCreatedStatus();
        }
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.BUCKET_CREATION_FLOW);
    } finally {
        isLoading.value = false;
    }
}

/**
 * Sets bucket name value from input to local variable.
 */
function setBucketName(name: string): void {
    bucketName.value = name;
}

/**
 * Closes create bucket modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
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
 * Returns validation status of a bucket name.
 */
function isBucketNameValid(name: string): boolean {
    switch (true) {
    case name.length < 3 || name.length > 63:
        nameError.value = 'Name must be not less than 3 and not more than 63 characters length';
        return false;
    case !Validator.bucketName(name):
        nameError.value = 'Name must contain only lowercase latin characters, numbers, a hyphen or a period';
        return false;
    default:
        return true;
    }
}

onMounted(async (): Promise<void> => {
    setWorker();

    try {
        await bucketsStore.getAllBucketsNames(projectsStore.state.selectedProject.id);
        bucketName.value = allBucketNames.value.length > 0 ? '' : 'demo-bucket';
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.BUCKET_CREATION_NAME_STEP);
    } finally {
        bucketNamesLoading.value = false;
    }
});
</script>

<style scoped lang="scss">
.modal {
    padding: 32px;
    font-family: 'font_regular', sans-serif;
    display: flex;
    flex-direction: column;
    max-width: 350px;

    @media screen and (width <= 615px) {
        padding: 30px 20px;
    }

    &__blur {
        position: absolute;
        top: 0;
        left: 0;
        height: 100%;
        width: 100%;
        background-color: rgb(229 229 229 / 20%);
        border-radius: 8px;
        z-index: 100;

        &__loader {
            width: 25px;
            height: 25px;
            position: absolute;
            right: 40px;
            top: 40px;
        }
    }

    :deep(.label-container) {
        margin-bottom: 8px;
    }

    :deep(.label-container__main__label) {
        font-family: 'font_bold', sans-serif;
        font-size: 14px;
        color: #56606d;
    }

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
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
        line-height: 20px;
        color: var(--c-blue-6);
        padding: 16px 0;
        border-bottom: 1px solid var(--c-grey-2);
        text-align: left;
    }

    &__input-container {
        padding-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);
    }

    &__button-container {
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: 16px;
        column-gap: 20px;

        @media screen and (width <= 600px) {
            margin-top: 20px;
            column-gap: unset;
            row-gap: 8px;
            flex-direction: column-reverse;
        }
    }
}

</style>

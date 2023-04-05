// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <CreateBucketIcon class="modal__icon" />
                <h1 class="modal__title" aria-roledescription="modal-title">
                    Create a Bucket
                </h1>
                <p class="modal__info">
                    Buckets are used to store and organize your files. Enter lowercase alphanumeric characters only,
                    no spaces.
                </p>
                <VLoader v-if="bucketNamesLoading" width="100px" height="100px" />
                <VInput
                    v-else
                    :init-value="bucketName"
                    label="Bucket Name"
                    placeholder="Enter bucket name"
                    class="full-input"
                    :error="nameError"
                    @setData="setBucketName"
                />
                <div class="modal__button-container">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="48px"
                        font-size="14px"
                        :on-press="closeModal"
                        :is-transparent="true"
                    />
                    <VButton
                        label="Create bucket"
                        width="100%"
                        height="48px"
                        font-size="14px"
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

import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { Validator } from '@/utils/validation';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { LocalData } from '@/utils/localData';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { FILE_BROWSER_AG_NAME } from '@/store/modules/bucketsStore';

import VLoader from '@/components/common/VLoader.vue';
import VInput from '@/components/common/VInput.vue';
import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';

import CreateBucketIcon from '@/../static/images/buckets/createBucket.svg';

const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const store = useStore();
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
    return store.state.bucketUsageModule.allBucketNames;
});

/**
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return store.state.objectsModule.promptForPassphrase;
});

/**
 * Returns object browser api key from store.
 */
const apiKey = computed((): string => {
    return store.state.objectsModule.apiKey;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return store.state.objectsModule.gatewayCredentials;
});

/**
 * Returns edge credentials for bucket creation from store.
 */
const gatewayCredentialsForCreate = computed((): EdgeCredentials => {
    return store.state.objectsModule.gatewayCredentialsForCreate;
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
        const projectID = store.getters.selectedProject.id;

        if (!promptForPassphrase.value) {
            if (!edgeCredentials.value.accessKeyId) {
                await store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
            }
            await store.dispatch(OBJECTS_ACTIONS.CREATE_BUCKET, bucketName.value);
            await store.dispatch(BUCKET_ACTIONS.FETCH, 1);
            await store.dispatch(OBJECTS_ACTIONS.SET_FILE_COMPONENT_BUCKET_NAME, bucketName.value);
            analytics.eventTriggered(AnalyticsEvent.BUCKET_CREATED);
            analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
            await router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
            closeModal();

            if (!bucketWasCreated.value) {
                LocalData.setBucketWasCreatedStatus();
            }

            return;
        }

        if (gatewayCredentialsForCreate.value.accessKeyId) {
            await store.dispatch(OBJECTS_ACTIONS.CREATE_BUCKET_WITH_NO_PASSPHRASE, bucketName.value);
            await store.dispatch(BUCKET_ACTIONS.FETCH, 1);
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
            await store.dispatch(OBJECTS_ACTIONS.SET_API_KEY, cleanAPIKey.secret);
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
            'buckets': [],
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

        const salt = await store.dispatch(PROJECTS_ACTIONS.GET_SALT, store.getters.selectedProject.id);
        const satelliteNodeURL: string = appStore.state.config.satelliteNodeURL;

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

        const gatewayCredentials: EdgeCredentials = await agStore.getEdgeCredentials(accessGrant);
        await store.dispatch(OBJECTS_ACTIONS.SET_GATEWAY_CREDENTIALS_FOR_CREATE, gatewayCredentials);
        await store.dispatch(OBJECTS_ACTIONS.CREATE_BUCKET_WITH_NO_PASSPHRASE, bucketName.value);
        await store.dispatch(BUCKET_ACTIONS.FETCH, 1);
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
    appStore.updateActiveModal(MODALS.createBucket);
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
        await store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
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
        width: 430px;
        padding: 43px 60px 66px;
        display: flex;
        align-items: center;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;

        @media screen and (max-width: 600px) {
            width: calc(100% - 48px);
            padding: 54px 24px 32px;
        }

        &__icon {
            max-height: 154px;
            max-width: 118px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            color: #1b2533;
            margin-top: 20px;
            text-align: center;

            @media screen and (max-width: 600px) {
                margin-top: 16px;
                font-size: 24px;
                line-height: 31px;
            }
        }

        &__info {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #354049;
            margin: 20px 0 0;
        }

        &__button-container {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 30px;
            column-gap: 20px;

            @media screen and (max-width: 600px) {
                margin-top: 20px;
                column-gap: unset;
                row-gap: 8px;
                flex-direction: column-reverse;
            }
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
    }

    .full-input {
        margin-top: 20px;
    }

    :deep(.label-container) {
        margin-bottom: 8px;
    }

    :deep(.label-container__main__label) {
        font-family: 'font_bold', sans-serif;
        font-size: 14px;
        color: #56606d;
    }
</style>

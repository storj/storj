// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <h1 class="modal__title">Are you sure?</h1>
                <p class="modal__subtitle">
                    Deleting bucket will delete all metadata related to this bucket.
                </p>
                <VInput
                    class="modal__input"
                    label="Bucket Name"
                    placeholder="Enter bucket name"
                    :is-loading="isLoading"
                    @setData="onChangeName"
                />
                <VButton
                    label="Confirm Delete Bucket"
                    width="100%"
                    height="48px"
                    :on-press="onDelete"
                    :is-disabled="isLoading || !name"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { MetaUtils } from '@/utils/meta';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useNotify, useStore } from '@/utils/hooks';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

const FILE_BROWSER_AG_NAME = 'Web file browser API key';
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const store = useStore();
const notify = useNotify();

const worker = ref<Worker| null>(null);
const name = ref<string>('');
const isLoading = ref<boolean>(false);

/**
 * Returns apiKey from store.
 */
const apiKey = computed((): string => {
    return store.state.objectsModule.apiKey;
});

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

    try {
        if (!apiKey.value) {
            await store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE_BY_NAME_AND_PROJECT_ID, FILE_BROWSER_AG_NAME);
            const cleanAPIKey: AccessGrant = await store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, FILE_BROWSER_AG_NAME);
            await store.dispatch(OBJECTS_ACTIONS.SET_API_KEY, cleanAPIKey.secret);
        }

        const now = new Date();
        const inOneHour = new Date(now.setHours(now.getHours() + 1));

        await worker.value.postMessage({
            'type': 'SetPermission',
            'isDownload': false,
            'isUpload': false,
            'isList': true,
            'isDelete': true,
            'notAfter': inOneHour.toISOString(),
            'buckets': [name.value],
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
        const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');

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

        const gatewayCredentials: EdgeCredentials = await store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant });
        await store.dispatch(OBJECTS_ACTIONS.SET_GATEWAY_CREDENTIALS_FOR_DELETE, gatewayCredentials);
        await store.dispatch(OBJECTS_ACTIONS.DELETE_BUCKET, name.value);
        analytics.eventTriggered(AnalyticsEvent.BUCKET_DELETED);
        await fetchBuckets();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
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
        await store.dispatch(BUCKET_ACTIONS.FETCH, page);
    } catch (error) {
        await notify.error(`Unable to fetch buckets. ${error.message}`, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
    }
}

/**
 * Sets local worker with worker instantiated in store.
 */
function setWorker(): void {
    worker.value = store.state.accessGrantsModule.accessGrantsWebWorker;
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
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.deleteBucket);
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
    padding: 45px 70px;
    border-radius: 10px;
    font-family: 'font_regular', sans-serif;
    font-style: normal;
    display: flex;
    flex-direction: column;
    align-items: center;
    background-color: #fff;
    max-width: 480px;

    @media screen and (max-width: 700px) {
        padding: 45px;
    }

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 22px;
        line-height: 27px;
        color: #000;
        margin: 0 0 18px;
    }

    &__subtitle {
        font-size: 18px;
        line-height: 30px;
        text-align: center;
        letter-spacing: -0.1007px;
        color: rgb(37 37 37 / 70%);
        margin: 0 0 24px;
    }

    &__input {
        margin-bottom: 18px;
    }
}
</style>

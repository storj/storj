// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="switch-step">
        <h1 class="switch-step__title">Switch passphrase</h1>
        <p class="switch-step__info">
            Switch passphrases to view existing data that is uploaded with a different passphrase, or upload new data.
            Please note that you won’t see the previous data once you switch passphrases.
        </p>
        <VInput
            label="Encryption Passphrase"
            :is-password="true"
            width="100%"
            height="56px"
            placeholder="Enter Encryption Passphrase"
            :error="enterError"
            @setData="setPassphrase"
        />
        <div class="switch-step__buttons">
            <VButton
                label="Cancel"
                width="100%"
                height="48px"
                :is-white="true"
                :on-press="onCancel"
            />
            <VButton
                label="Switch Passphrase"
                width="100%"
                height="48px"
                :on-press="onSwitch"
                :is-disabled="isLoading"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useNotify, useStore } from '@/utils/hooks';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { OBJECTS_ACTIONS, OBJECTS_MUTATIONS } from '@/store/modules/objects';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

const FILE_BROWSER_AG_NAME = 'Web file browser API key';

const props = withDefaults(defineProps<{
    onCancel?: () => void,
}>(), {
    onCancel: () => () => {},
});

const notify = useNotify();
const store = useStore();

const passphrase = ref<string>('');
const enterError = ref<string>('');
const isLoading = ref<boolean>(false);
const worker = ref<Worker | null>(null);

/**
 * Returns web file browser api key from vuex state.
 */
const apiKey = computed((): string => {
    return store.state.objectsModule.apiKey;
});

/**
 * Lifecycle hook after initial render.
 * Sets local worker.
 */
onMounted(() => {
    setWorker();
});

/**
 * Sets passphrase input value to local variable.
 * Resets error is present.
 * @param value
 */
function setPassphrase(value: string): void {
    if (enterError.value) {
        enterError.value = '';
    }

    passphrase.value = value;
}

/**
 * Sets local worker with worker instantiated in store.
 */
function setWorker(): void {
    worker.value = store.state.accessGrantsModule.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.SWITCH_PROJECT_LEVEL_PASSPHRASE_MODAL);
        };
    }
}

/**
 * Generates s3 credentials from provided passphrase and stores it in vuex state to be reused.
 */
async function setAccess(): Promise<void> {
    if (!worker.value) {
        throw new Error('Worker is not defined');
    }

    if (!apiKey.value) {
        await store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE_BY_NAME_AND_PROJECT_ID, FILE_BROWSER_AG_NAME);
        const cleanAPIKey: AccessGrant = await store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, FILE_BROWSER_AG_NAME);
        await store.dispatch(OBJECTS_ACTIONS.SET_API_KEY, cleanAPIKey.secret);
    }

    const now = new Date();
    const inThreeDays = new Date(now.setDate(now.getDate() + 3));

    await worker.value.postMessage({
        'type': 'SetPermission',
        'isDownload': true,
        'isUpload': true,
        'isList': true,
        'isDelete': true,
        'notAfter': inThreeDays.toISOString(),
        'buckets': [],
        'apiKey': apiKey.value,
    });

    const grantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    if (grantEvent.data.error) {
        throw new Error(grantEvent.data.error);
    }

    const salt = await store.dispatch(PROJECTS_ACTIONS.GET_SALT, store.getters.selectedProject.id);
    const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');

    worker.value.postMessage({
        'type': 'GenerateAccess',
        'apiKey': grantEvent.data.value,
        'passphrase': passphrase.value,
        'salt': salt,
        'satelliteNodeURL': satelliteNodeURL,
    });

    const accessGrantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    if (accessGrantEvent.data.error) {
        throw new Error(accessGrantEvent.data.error);
    }

    const accessGrant = accessGrantEvent.data.value;

    const gatewayCredentials: EdgeCredentials = await store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant });
    await store.dispatch(OBJECTS_ACTIONS.SET_GATEWAY_CREDENTIALS, gatewayCredentials);
    await store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
    store.commit(OBJECTS_MUTATIONS.SET_PROMPT_FOR_PASSPHRASE, false);
}

/**
 * Sets new passphrase and generates new edge credentials.
 */
async function onSwitch(): Promise<void> {
    if (isLoading.value) return;

    if (!passphrase.value) {
        enterError.value = 'Passphrase can\'t be empty';

        return;
    }

    isLoading.value = true;

    try {
        await setAccess();
        store.dispatch(OBJECTS_ACTIONS.SET_PASSPHRASE, passphrase.value);

        notify.success('Passphrase was switched successfully');
        store.commit(APP_STATE_MUTATIONS.TOGGLE_MANAGE_PROJECT_PASSPHRASE_MODAL_SHOWN);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.SWITCH_PROJECT_LEVEL_PASSPHRASE_MODAL);
    } finally {
        isLoading.value = false;
    }
}
</script>

<style scoped lang="scss">
.switch-step {
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

    &__buttons {
        display: flex;
        align-items: center;
        justify-content: center;
        column-gap: 33px;
        margin-top: 20px;
        width: 100%;

        @media screen and (max-width: 530px) {
            column-gap: unset;
            flex-direction: column-reverse;
            row-gap: 15px;
        }
    }
}
</style>

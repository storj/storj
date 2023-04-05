// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <h1 class="modal__title">Share Bucket</h1>
                <ShareContainer :link="link" />
                <p class="modal__label">
                    Or copy link:
                </p>
                <VLoader v-if="isLoading" width="20px" height="20px" />
                <div v-if="!isLoading" class="modal__input-group">
                    <input
                        id="url"
                        class="modal__input"
                        type="url"
                        :value="link"
                        aria-describedby="btn-copy-link"
                        readonly
                    >
                    <VButton
                        :label="copyButtonState === ButtonStates.Copy ? 'Copy' : 'Copied'"
                        width="114px"
                        height="40px"
                        :on-press="onCopy"
                        :is-disabled="isLoading"
                        :is-green="copyButtonState === ButtonStates.Copied"
                        :icon="copyButtonState === ButtonStates.Copied ? 'none' : 'copy'"
                    >
                        <template v-if="copyButtonState === ButtonStates.Copied" #icon>
                            <check-icon />
                        </template>
                    </VButton>
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify, useStore } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';

import VModal from '@/components/common/VModal.vue';
import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';
import ShareContainer from '@/components/common/share/ShareContainer.vue';

import CheckIcon from '@/../static/images/common/check.svg';

enum ButtonStates {
    Copy,
    Copied,
}

const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const store = useStore();
const notify = useNotify();

const worker = ref<Worker | null>(null);
const isLoading = ref<boolean>(true);
const link = ref<string>('');
const copyButtonState = ref<ButtonStates>(ButtonStates.Copy);

/**
 * Returns chosen bucket name from store.
 */
const bucketName = computed((): string => {
    return store.state.objectsModule.fileComponentBucketName;
});

/**
 * Returns passphrase from store.
 */
const passphrase = computed((): string => {
    return store.state.objectsModule.passphrase;
});

/**
 * Copies link to users clipboard.
 */
async function onCopy(): Promise<void> {
    await navigator.clipboard.writeText(link.value);
    copyButtonState.value = ButtonStates.Copied;

    setTimeout(() => {
        copyButtonState.value = ButtonStates.Copy;
    }, 2000);

    await notify.success('Link copied successfully.');
}

/**
 * Sets share bucket link.
 */
async function setShareLink(): Promise<void> {
    if (!worker.value) {
        return;
    }

    try {
        let path = `${bucketName.value}`;
        const now = new Date();
        const LINK_SHARING_AG_NAME = `${path}_shared-bucket_${now.toISOString()}`;
        const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(LINK_SHARING_AG_NAME, store.getters.selectedProject.id);

        const satelliteNodeURL = appStore.state.config.satelliteNodeURL;
        const salt = await store.dispatch(PROJECTS_ACTIONS.GET_SALT, store.getters.selectedProject.id);

        worker.value.postMessage({
            'type': 'GenerateAccess',
            'apiKey': cleanAPIKey.secret,
            'passphrase': passphrase.value,
            'salt': salt,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => {
            if (worker.value) {
                worker.value.onmessage = resolve;
            }
        });
        const grantData = grantEvent.data;
        if (grantData.error) {
            await notify.error(grantData.error, AnalyticsErrorEventSource.SHARE_BUCKET_MODAL);
            return;
        }

        worker.value.postMessage({
            'type': 'RestrictGrant',
            'isDownload': true,
            'isUpload': false,
            'isList': true,
            'isDelete': false,
            'paths': [path],
            'grant': grantData.value,
        });

        const event: MessageEvent = await new Promise(resolve => {
            if (worker.value) {
                worker.value.onmessage = resolve;
            }
        });
        const data = event.data;
        if (data.error) {
            await notify.error(data.error, AnalyticsErrorEventSource.SHARE_BUCKET_MODAL);
            return;
        }

        const credentials: EdgeCredentials = await agStore.getEdgeCredentials(data.value, undefined, true);

        path = encodeURIComponent(path.trim());

        const linksharingURL = appStore.state.config.linksharingURL;

        link.value = `${linksharingURL}/${credentials.accessKeyId}/${path}`;
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.SHARE_BUCKET_MODAL);
    } finally {
        isLoading.value = false;
    }
}

/**
 * Sets local worker with worker instantiated in store.
 */
function setWorker(): void {
    worker.value = agStore.state.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.SHARE_BUCKET_MODAL);
        };
    }
}

/**
 * Closes open bucket modal.
 */
function closeModal(): void {
    if (isLoading.value) return;

    appStore.updateActiveModal(MODALS.shareBucket);
}

onMounted(async () => {
    setWorker();
    await setShareLink();
});
</script>

<style scoped lang="scss">
    .modal {
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 50px;
        max-width: 470px;

        @media screen and (max-width: 430px) {
            padding: 20px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 22px;
            line-height: 29px;
            color: #1b2533;
            margin: 10px 0 35px;
        }

        &__label {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 21px;
            color: #354049;
            align-self: center;
            margin: 20px 0 10px;
        }

        &__link {
            font-size: 16px;
            line-height: 21px;
            color: #384b65;
            align-self: flex-start;
            word-break: break-all;
            text-align: left;
        }

        &__buttons {
            display: flex;
            column-gap: 20px;
            margin-top: 32px;
            width: 100%;

            @media screen and (max-width: 430px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 15px;
            }
        }

        &__input-group {
            border: 1px solid var(--c-grey-4);
            background: var(--c-grey-1);
            padding: 10px;
            display: flex;
            justify-content: space-between;
            border-radius: 8px;
            width: 100%;
            height: 42px;
        }

        &__input {
            background: none;
            border: none;
            font-size: 14px;
            color: var(--c-grey-6);
            outline: none;
            max-width: 340px;
            width: 100%;

            @media screen and (max-width: 430px) {
                max-width: 210px;
            }
        }
    }
</style>

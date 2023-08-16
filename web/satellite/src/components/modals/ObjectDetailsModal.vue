// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__preview">
                    <img
                        v-if="previewAndMapFailed"
                        class="failed-preview"
                        :src="ErrorNoticeIcon"
                        alt="failed preview"
                    >
                    <template v-else>
                        <img
                            v-if="previewIsImage && !isLoading"
                            class="preview img-fluid"
                            :src="objectPreviewUrl"
                            aria-roledescription="image-preview"
                            alt="preview"
                        >

                        <video
                            v-if="previewIsVideo && !isLoading"
                            class="preview"
                            controls
                            :src="objectPreviewUrl"
                            aria-roledescription="video-preview"
                        />

                        <audio
                            v-if="previewIsAudio && !isLoading"
                            class="preview"
                            controls
                            :src="objectPreviewUrl"
                            aria-roledescription="audio-preview"
                        />
                        <PlaceholderImage v-if="placeHolderDisplayable" />
                    </template>
                </div>
                <div class="modal__info">
                    <p class="modal__info__title">
                        {{ filePath }}
                    </p>
                    <p class="modal__info__size">
                        <span class="modal__info__size__label text-lighter">Size:</span>
                        {{ size }}
                    </p>
                    <p class="modal__info__size">
                        <span class="modal__info__size__label text-lighter">Created:</span>
                        {{ uploadDate }}
                    </p>
                    <VButton
                        class="modal__info__download-btn"
                        label="Download"
                        width="100%"
                        height="34px"
                        :on-press="download"
                    />
                    <div
                        v-if="objectLink"
                        class="modal__info__input-group"
                    >
                        <input
                            id="url"
                            class="form-control"
                            type="url"
                            :value="objectLink"
                            aria-describedby="generateShareLink"
                            readonly
                        >
                        <VButton
                            class="modal__info__input-group__copy"
                            :label="copyText"
                            :is-transparent="true"
                            font-size="14px"
                            width="unset"
                            height="34px"
                            :on-press="copy"
                        />
                    </div>
                    <VButton
                        v-else
                        label="Share"
                        width="100%"
                        height="34px"
                        :is-transparent="true"
                        :on-press="getSharedLink"
                    />
                    <VLoader v-if="isLoading" class="modal__info__loader" />
                    <div
                        v-if="objectMapUrl && !previewAndMapFailed"
                        class="modal__info__map"
                    >
                        <div class="storage-nodes">
                            Nodes storing this file
                        </div>
                        <img
                            class="object-map"
                            :src="objectMapUrl"
                            alt="object map"
                        >
                    </div>
                    <p v-if="!placeHolderDisplayable && !previewAndMapFailed && !isLoading" class="modal__info__note text-lighter">
                        Note: If you would like to share this object with others, please use the 'Share'
                        button rather than copying the path from the object preview. This object preview
                        link will expire within 24 hours.
                    </p>
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref, watch } from 'vue';
import prettyBytes from 'pretty-bytes';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useAppStore } from '@/store/modules/appStore';
import { useLinksharing } from '@/composables/useLinksharing';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';

import ErrorNoticeIcon from '@/../static/images/common/errorNotice.svg?url';
import PlaceholderImage from '@/../static/images/browser/placeholder.svg';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();

const notify = useNotify();
const { generateFileOrFolderShareURL, generateObjectPreviewAndMapURL } = useLinksharing();

const isLoading = ref<boolean>(false);
const previewAndMapFailed = ref<boolean>(false);
const objectMapUrl = ref<string>('');
const objectPreviewUrl = ref<string>('');
const objectLink = ref<string>('');
const copyText = ref<string>('Copy Link');

/**
 * Retrieve the file object that the modal is set to from the store.
 */
const file = computed((): BrowserObject | undefined => {
    return obStore.state.files.find(
        (file) => file.Key === filePath.value.split('/').slice(-1)[0],
    );
});

/**
 * Retrieve the filepath of the modal from the store.
 */
const filePath = computed((): string => {
    return obStore.state.objectPathForModal;
});

/**
 * Format the file size to be displayed.
 */
const size = computed((): string => {
    return prettyBytes(
        obStore.state.files.find((f) => f.Key === file.value?.Key)?.Size || 0,
    );
});

/**
 * Format the upload date of the current file.
 */
const uploadDate = computed((): string | undefined => {
    return file.value?.LastModified.toLocaleString().split(',')[0];
});

/**
 * Get the extension of the current file.
 */
const extension = computed((): string | undefined => {
    return filePath.value.split('.').pop();
});

/**
 * Check to see if the current file is an image file.
 */
const previewIsImage = computed((): boolean => {
    if (typeof extension.value !== 'string') {
        return false;
    }

    return ['bmp', 'svg', 'jpg', 'jpeg', 'png', 'ico', 'gif'].includes(
        extension.value.toLowerCase(),
    );
});

/**
 * Check to see if the current file is a video file.
 */
const previewIsVideo = computed((): boolean => {
    if (typeof extension.value !== 'string') {
        return false;
    }

    return ['m4v', 'mp4', 'webm', 'mov', 'mkv'].includes(
        extension.value.toLowerCase(),
    );
});

/**
 * Check to see if the current file is an audio file.
 */
const previewIsAudio = computed((): boolean => {
    if (typeof extension.value !== 'string') {
        return false;
    }

    return ['mp3', 'wav', 'ogg'].includes(extension.value.toLowerCase());
});

/**
 * Check to see if the current file is neither an image file, video file, or audio file.
 */
const placeHolderDisplayable = computed((): boolean => {
    return [
        previewIsImage.value,
        previewIsVideo.value,
        previewIsAudio.value,
    ].every((value) => !value);
});

/**
 * Get the object map url for the file being displayed.
 */
async function fetchPreviewAndMapUrl(): Promise<void> {
    isLoading.value = true;

    let url = '';
    try {
        url = await generateObjectPreviewAndMapURL(bucketsStore.state.fileComponentBucketName, filePath.value);
    } catch (error) {
        error.message = `Unable to get file preview and map URL. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.ACCESS_GRANTS_PAGE);
    }

    if (!url) {
        previewAndMapFailed.value = true;
        isLoading.value = false;

        return;
    }

    const mapURL = `${url}?map=1`;
    const previewURL = `${url}?view=1`;

    await new Promise((resolve) => {
        const preload = new Image();
        preload.onload = resolve;
        preload.src = mapURL;
    });

    objectMapUrl.value = mapURL;
    objectPreviewUrl.value = previewURL;
    isLoading.value = false;
}

/**
 * Download the current opened file.
 */
async function download(): Promise<void> {
    try {
        await obStore.download(file.value);
        notify.warning('Do not share download link with other people. If you want to share this data better use "Share" option.');
    } catch (error) {
        notify.error('Can not download your file', AnalyticsErrorEventSource.OBJECT_DETAILS_MODAL);
    }
}

/**
 * Close the current opened file details modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Copy the current opened file.
 */
async function copy(): Promise<void> {
    await navigator.clipboard.writeText(objectLink.value);
    notify.success('Link copied successfully.');

    copyText.value = 'Copied!';
    setTimeout(() => {
        copyText.value = 'Copy Link';
    }, 2000);
}

/**
 * Get the share link of the current opened file.
 */
async function getSharedLink(): Promise<void> {
    analyticsStore.eventTriggered(AnalyticsEvent.LINK_SHARED);
    try {
        objectLink.value = await generateFileOrFolderShareURL(
            bucketsStore.state.fileComponentBucketName, filePath.value);
    } catch (error) {
        error.message = `Unable to get sharing URL. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.OBJECT_DETAILS_MODAL);
    }
}

/**
 * Call `fetchPreviewAndMapUrl` on before mount lifecycle method.
 */
onBeforeMount((): void => {
    fetchPreviewAndMapUrl();
});

/**
 * Watch for changes on the filepath and call `fetchObjectMapUrl` the moment it updates.
 */
watch(filePath, () => {
    fetchPreviewAndMapUrl();
});
</script>

<style scoped lang="scss">
    .modal {
        box-sizing: border-box;
        display: flex;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        width: 1140px;
        max-width: 1140px;

        @media screen and (width <= 1200px) {
            width: 800px;
            max-width: 800px;
        }

        @media screen and (width <= 900px) {
            width: 600px;
            max-width: 600px;
        }

        @media screen and (width <= 660px) {
            flex-direction: column-reverse;
            width: calc(100vw - 50px);
            min-width: calc(100vw - 50px);
            max-width: calc(100vw - 50px);
        }

        @media screen and (width <= 400px) {
            width: calc(100vw - 20px);
            min-width: calc(100vw - 20px);
            max-width: calc(100vw - 20px);
        }

        &__preview {
            width: 67%;
            max-width: 67%;
            min-height: 75vh;
            display: flex;
            align-items: center;
            justify-content: center;

            @media screen and (width <= 900px) {
                width: 60%;
                max-width: 60%;
            }

            @media screen and (width <= 660px) {
                width: 100%;
                min-width: 100%;
                max-width: 100%;
                min-height: 50vh;
                margin-top: 10px;
                border-radius: 0 0 10px 10px;
            }

            @media screen and (width <= 500px) {
                min-height: 30vh;
            }
        }

        &__info {
            padding: 64px 32px;
            box-sizing: border-box;
            width: 33%;
            max-width: 33%;
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            align-self: flex-start;

            @media screen and (width <= 900px) {
                width: 40%;
                max-width: 40%;
            }

            @media screen and (width <= 660px) {
                width: 100%;
                min-width: 100%;
                max-width: 100%;
                padding-bottom: 0;
            }

            @media screen and (width <= 400px) {
                padding: 64px 20px 0;
            }

            &__title {
                display: inline-block;
                font-weight: bold;
                max-width: 100%;
                position: relative;
                font-size: 18px;
                text-overflow: ellipsis;
                white-space: nowrap;
                overflow: hidden;
                margin-bottom: 14px;
            }

            &__size {
                margin-bottom: 14px;
                font-size: 12px;

                &__label {
                    margin-right: 7px;
                }
            }

            &__download-btn {
                margin-bottom: 14px;
            }

            &__input-group {
                display: flex;
                align-items: center;
                width: 100%;

                @media screen and (width <= 1200px) {
                    flex-direction: column;
                    align-items: flex-start;
                }

                @media screen and (width <= 660px) {
                    flex-direction: row;
                    align-items: center;
                }

                @media screen and (width <= 400px) {
                    flex-direction: column;
                    align-items: flex-start;
                }

                input {
                    box-sizing: border-box;
                    width: 100%;
                    padding: 0.375rem 0.75rem;
                    font-size: 1rem;
                    font-weight: 400;
                    line-height: 1.5;
                    color: #495057;
                    border: 1px solid #ced4da;
                    border-radius: 0.25rem;
                    background-color: #e9ecef;
                }

                &__copy {
                    padding: 0 10px;
                }
            }

            &__loader {
                margin: 20px 0;
            }

            &__map {
                margin-top: 36px;
            }

            &__note {
                margin-top: 16px;
                text-align: left;
            }
        }
    }

    .preview {
        width: 100%;
    }

    .object-map {
        width: 100%;
    }

    .storage-nodes {
        padding: 5px;
        background: rgb(0 0 0 / 80%);
        font-weight: normal;
        color: white;
        font-size: 0.8rem;
    }

    .text-lighter {
        color: #768394;
    }

    .failed-preview {
        width: 50%;
    }
</style>

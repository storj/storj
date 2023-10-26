// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <Teleport to="#app">
        <div ref="viewContainer" class="gallery" tabindex="0" @keydown.esc="closeModal" @keydown.right="onNext" @keydown.left="onPrevious">
            <div class="gallery__header">
                <LogoIcon class="gallery__header__logo" />
                <SmallLogoIcon class="gallery__header__small-logo" />
                <div class="gallery__header__name">
                    <ImageIcon v-if="previewType === PreviewType.Image" />
                    <VideoIcon v-else-if="previewType === PreviewType.Audio || previewType === PreviewType.Video" />
                    <EmptyIcon v-else />
                    <p class="gallery__header__name__label" :title="file?.Key || ''">{{ file?.Key || '' }}</p>
                </div>
                <div class="gallery__header__functional">
                    <ButtonIcon
                        v-click-outside="closeDropdown"
                        :icon="DotsIcon"
                        :on-press="toggleDropdown"
                        :is-active="isOptionsDropdown === true"
                        info="More"
                    />
                    <ButtonIcon
                        class="gallery__header__functional__item"
                        :icon="MapIcon"
                        :on-press="() => setActiveModal(DistributionModal)"
                        info="Geographic Distribution"
                    />
                    <ButtonIcon
                        :icon="DownloadIcon"
                        :on-press="download"
                        info="Download"
                    />
                    <ButtonIcon
                        class="gallery__header__functional__item"
                        :icon="ShareIcon"
                        :on-press="showShareModal"
                        info="Share"
                    />
                    <ButtonIcon
                        :icon="CloseIcon"
                        :on-press="closeModal"
                        info="Close"
                    />
                    <OptionsDropdown
                        v-if="isOptionsDropdown"
                        :on-distribution="() => setActiveModal(DistributionModal)"
                        :on-view-details="() => setActiveModal(DetailsModal)"
                        :on-download="download"
                        :on-share="showShareModal"
                        :on-delete="() => setActiveModal(DeleteModal)"
                    />
                </div>
            </div>
            <div class="gallery__main">
                <ArrowIcon class="gallery__main__left-arrow" @click="onPrevious" />
                <VLoader v-if="isLoading" class="gallery__main__loader" width="100px" height="100px" is-white />
                <div v-else class="gallery__main__preview">
                    <text-file-preview v-if="previewType === PreviewType.Text" :src="objectPreviewUrl">
                        <file-preview-placeholder :file="file" @download="download" />
                    </text-file-preview>
                    <c-s-v-file-preview v-else-if="previewType === PreviewType.CSV" :src="objectPreviewUrl">
                        <file-preview-placeholder :file="file" @download="download" />
                    </c-s-v-file-preview>
                    <img
                        v-else-if="previewType === PreviewType.Image"
                        :src="objectPreviewUrl"
                        class="gallery__main__preview__item"
                        aria-roledescription="image-preview"
                        alt="preview"
                    >
                    <video
                        v-else-if="previewType === PreviewType.Video"
                        controls
                        :src="objectPreviewUrl"
                        class="gallery__main__preview__item"
                        aria-roledescription="video-preview"
                    />
                    <audio
                        v-else-if="previewType === PreviewType.Audio"
                        controls
                        :src="objectPreviewUrl"
                        class="gallery__main__preview__item"
                        aria-roledescription="audio-preview"
                    />
                    <file-preview-placeholder v-else :file="file" @download="download" />
                    <div class="gallery__main__preview__buttons">
                        <ArrowIcon class="gallery__main__preview__buttons__left-arrow" @click="onPrevious" />
                        <ArrowIcon @click="onNext" />
                    </div>
                </div>
                <ArrowIcon class="gallery__main__right-arrow" @click="onNext" />
            </div>
        </div>
        <div v-if="activeModal">
            <component
                :is="activeModal"
                :on-close="() => setActiveModal(undefined)"
                :object="file"
                :map-url="objectMapUrl"
                :on-delete="onDelete"
            />
        </div>
    </Teleport>
</template>

<script setup lang="ts">
import { Component, computed, h, onBeforeMount, onMounted, ref, Teleport, watch } from 'vue';
import { useRoute } from 'vue-router';

import { BrowserObject, PreviewCache, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useAppStore } from '@/store/modules/appStore';
import { useNotify } from '@/utils/hooks';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useLinksharing } from '@/composables/useLinksharing';
import { RouteConfig } from '@/types/router';
import { EXTENSION_PREVIEW_TYPES, PreviewType, ShareType } from '@/types/browser';

import ButtonIcon from '@/components/browser/galleryView/ButtonIcon.vue';
import OptionsDropdown from '@/components/browser/galleryView/OptionsDropdown.vue';
import DeleteModal from '@/components/browser/galleryView/modals/Delete.vue';
import ShareModal from '@/components/modals/ShareModal.vue';
import DetailsModal from '@/components/browser/galleryView/modals/Details.vue';
import DistributionModal from '@/components/browser/galleryView/modals/Distribution.vue';
import VLoader from '@/components/common/VLoader.vue';
import TextFilePreview from '@/components/browser/galleryView/TextFilePreview.vue';
import CSVFilePreview from '@/components/browser/galleryView/CSVFilePreview.vue';
import FilePreviewPlaceholder from '@/components/browser/galleryView/FilePreviewPlaceholder.vue';

import LogoIcon from '@/../static/images/logo.svg';
import SmallLogoIcon from '@/../static/images/smallLogo.svg';
import ImageIcon from '@/../static/images/browser/galleryView/image.svg';
import VideoIcon from '@/../static/images/browser/galleryView/video.svg';
import EmptyIcon from '@/../static/images/browser/galleryView/empty.svg';
import DotsIcon from '@/../static/images/browser/galleryView/dots.svg';
import MapIcon from '@/../static/images/browser/galleryView/map.svg';
import DownloadIcon from '@/../static/images/browser/galleryView/download.svg';
import ShareIcon from '@/../static/images/browser/galleryView/share.svg';
import CloseIcon from '@/../static/images/browser/galleryView/close.svg';
import ArrowIcon from '@/../static/images/browser/galleryView/arrow.svg';

const appStore = useAppStore();
const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const notify = useNotify();
const { generateObjectPreviewAndMapURL } = useLinksharing();

const route = useRoute();

const viewContainer = ref<HTMLElement>();
const isLoading = ref<boolean>(false);
const previewAndMapFailed = ref<boolean>(false);
const isOptionsDropdown = ref<boolean>(false);
const activeModal = ref<Component>();
const objectMapUrl = ref<string>('');
const objectPreviewUrl = ref<string>('');

const folderType = 'folder';

/**
 * Returns object preview URLs cache from store.
 */
const cachedObjectPreviewURLs = computed((): Map<string, PreviewCache> => {
    return obStore.state.cachedObjectPreviewURLs;
});

/**
 * Retrieve the file object that the modal is set to from the store.
 */
const file = computed((): BrowserObject => {
    return obStore.sortedFiles[fileIndex.value];
});

/**
 * Retrieve the file index that the modal is set to from the store.
 */
const fileIndex = computed((): number => {
    return obStore.sortedFiles.findIndex(f => f.Key === filePath.value.split('/').pop());
});

/**
 * Retrieve the filepath of the modal from the store.
 */
const filePath = computed((): string => {
    return obStore.state.objectPathForModal;
});

/**
 * Returns bucket name from store.
 */
const bucket = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Returns current path without object key.
 */
const currentPath = computed((): string => {
    return decodeURIComponent(route.path.replace(RouteConfig.Buckets.with(RouteConfig.UploadFile).path, ''));
});

/**
 * Returns the type of object being previewed.
 */
const previewType = computed<PreviewType>(() => {
    if (previewAndMapFailed.value) return PreviewType.None;

    const dotIdx = file.value.Key.lastIndexOf('.');
    if (dotIdx === -1) return PreviewType.None;

    const ext = file.value.Key.toLowerCase().slice(dotIdx + 1);
    for (const [exts, previewType] of EXTENSION_PREVIEW_TYPES) {
        if (exts.includes(ext)) return previewType;
    }

    return PreviewType.None;
});

/**
 * Get the object map url for the file being displayed.
 */
async function fetchPreviewAndMapUrl(): Promise<void> {
    isLoading.value = true;

    let url = '';
    try {
        url = await generateObjectPreviewAndMapURL(
            bucketsStore.state.fileComponentBucketName, filePath.value);
    } catch (error) {
        error.message = `Unable to get file preview and map URL. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.GALLERY_VIEW);
    }

    if (!url) {
        previewAndMapFailed.value = true;
        isLoading.value = false;

        return;
    }

    const encodedPath = encodeURIComponent(`${bucket.value}/${filePath.value.trim()}`);
    obStore.cacheObjectPreviewURL(encodedPath, { url, lastModified: file.value.LastModified.getTime() });

    objectMapUrl.value = `${url}?map=1`;
    objectPreviewUrl.value = `${url}?view=1`;
    isLoading.value = false;
}

/**
 * Deletes active object.
 */
async function onDelete(): Promise<void> {
    try {
        let newFile: BrowserObject | undefined = obStore.sortedFiles[fileIndex.value + 1];
        if (!newFile || newFile.type === folderType) {
            newFile = obStore.sortedFiles.find(f => f.type !== folderType && f.Key !== file.value.Key);
        }

        await obStore.deleteObject(
            currentPath.value,
            file.value,
        );
        setActiveModal(undefined);

        if (!newFile) {
            closeModal();
            return;
        }

        setNewObjectPath(newFile.Key);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.GALLERY_VIEW);
    }
}

/**
 * Download the current opened file.
 */
async function download(): Promise<void> {
    try {
        await obStore.download(file.value);
        notify.success(() => [
            h('p', { class: 'message-title' }, 'Downloading...'),
            h('p', { class: 'message-info' }, [
                'Keep this download link private.', h('br'), 'If you want to share, use the Share option.',
            ]),
        ]);
    } catch (error) {
        notify.error('Can not download your file', AnalyticsErrorEventSource.OBJECT_DETAILS_MODAL);
    }
}

/**
 * Close the current opened file details modal.
 */
function closeModal(): void {
    appStore.setGalleryView(false);
}

/**
 * Toggles options dropdown.
 */
function toggleDropdown(): void {
    isOptionsDropdown.value = !isOptionsDropdown.value;
}

/**
 * Closes options dropdown.
 */
function closeDropdown(): void {
    isOptionsDropdown.value = false;
}

/**
 * Sets active modal.
 */
function setActiveModal(value: Component | undefined): void {
    activeModal.value = value;

    if (!value) {
        viewContainer.value?.focus();
    }
}

/**
 * Handles on previous click logic.
 */
function onPrevious(): void {
    const currentIndex = fileIndex.value;
    const sortedFilesLength = obStore.sortedFiles.length;

    let newFile: BrowserObject;
    if (currentIndex <= 0) {
        newFile = obStore.sortedFiles[sortedFilesLength - 1];
    } else {
        newFile = obStore.sortedFiles[currentIndex - 1];
        if (newFile.type === folderType) {
            newFile = obStore.sortedFiles[sortedFilesLength - 1];
        }
    }

    setNewObjectPath(newFile.Key);
}

/**
 * Handles on next click logic.
 */
function onNext(): void {
    let newFile: BrowserObject | undefined = obStore.sortedFiles[fileIndex.value + 1];
    if (!newFile || newFile.type === folderType) {
        newFile = obStore.sortedFiles.find(f => f.type !== folderType);

        if (!newFile) return;
    }

    setNewObjectPath(newFile.Key);
}

/**
 * Sets new object path.
 */
function setNewObjectPath(objectKey: string): void {
    obStore.setObjectPathForModal(`${currentPath.value}${objectKey}`);
}

/**
 * Loads object URL from cache or generates new URL.
 */
function processFilePath(): void {
    const url = findCachedURL();
    if (!url) {
        fetchPreviewAndMapUrl();
        return;
    }

    objectMapUrl.value = `${url}?map=1`;
    objectPreviewUrl.value = `${url}?view=1`;
}

/**
 * Try to find current object path in cache.
 */
function findCachedURL(): string | undefined {
    const encodedPath = encodeURIComponent(`${bucket.value}/${filePath.value.trim()}`);
    const cache = cachedObjectPreviewURLs.value.get(encodedPath);

    if (!cache) return undefined;
    if (cache.lastModified !== file.value.LastModified.getTime()) {
        obStore.removeFromObjectPreviewCache(encodedPath);
        return undefined;
    }

    return cache.url;
}

/**
 * Displays the Share modal.
 */
function showShareModal(): void {
    appStore.setShareModalType(ShareType.File);
    appStore.updateActiveModal(ShareModal);
}

/**
 * Call `fetchPreviewAndMapUrl` on before mount lifecycle method.
 */
onBeforeMount((): void => {
    processFilePath();
});

onMounted((): void => {
    viewContainer.value?.focus();
});

/**
 * Watch for changes on the filepath and call `fetchObjectMapUrl` the moment it updates.
 */
watch(filePath, () => {
    if (!filePath.value) return;

    processFilePath();
});
</script>

<style scoped lang="scss">
.gallery {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background-color: var(--c-black);
    font-family: 'font_regular', sans-serif;

    &__header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 30px 24px 0;
        max-width: 100%;

        &__logo {
            min-width: 207px;

            @media screen and (width <= 1100px) {
                display: none;
            }
        }

        &__small-logo {
            min-width: 40px;
            display: none;

            @media screen and (width <= 1100px) and (width > 500px) {
                display: block;
            }
        }

        svg {

            :deep(path) {
                fill: var(--c-white);
            }
        }

        &__name,
        &__functional {
            display: flex;
            align-items: center;
        }

        &__name {
            max-width: 50%;

            @media screen and (width <= 370px) {
                max-width: 45%;
            }

            svg {
                min-width: 32px;

                @media screen and (width <= 600px) {
                    display: none;
                }
            }

            &__label {
                font-size: 16px;
                line-height: 24px;
                color: var(--c-white);
                margin-left: 8px;
                overflow-x: hidden;
                white-space: nowrap;
                text-overflow: ellipsis;
            }
        }

        &__functional {
            column-gap: 16px;
            position: relative;

            @media screen and (width <= 1100px) {
                column-gap: 8px;

                &__item {
                    display: none;
                }
            }
        }
    }

    &__main {
        display: flex;
        justify-content: space-between;
        padding: 48px 32px 0;
        width: 100%;
        box-sizing: border-box;
        height: calc(100vh - 120px);

        @media screen and (width <= 1100px) {
            padding: 24px 12px 0;
        }

        @media screen and (width <= 600px) {
            padding: 24px 0 0;
            height: calc(100vh - 140px);
        }

        &__loader {
            align-self: center;

            @media screen and (width <= 600px) {
                align-self: flex-start;
            }
        }

        &__left-arrow,
        &__right-arrow {
            align-self: center;
            cursor: pointer;
            min-width: 46px;

            &:hover {

                :deep(rect)  {

                    &:first-of-type {
                        fill: rgb(255 255 255 / 10%);
                    }
                }
            }

            @media screen and (width <= 600px) {
                display: none;
            }
        }

        &__left-arrow {
            transform: rotate(180deg);
        }

        &__preview {
            box-sizing: border-box;
            width: 100%;
            padding: 0 42px;
            display: flex;

            @media screen and (width <= 1100px) {
                padding: 0 12px;
            }

            @media screen and (width <= 600px) {
                flex-direction: column;
            }

            &__item {
                margin: 0 auto;
                max-height: 100%;
                max-width: 100%;
                align-self: center;

                @media screen and (width <= 600px) {
                    align-self: flex-start;
                }
            }

            &__buttons {
                display: none;

                svg {
                    width: 30px;
                    height: 30px;

                    &:hover {

                        :deep(rect)  {

                            &:first-of-type {
                                fill: rgb(255 255 255 / 10%);
                            }
                        }
                    }
                }

                @media screen and (width <= 600px) {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    column-gap: 16px;
                    margin-top: 20px;

                    &__left-arrow {
                        transform: rotate(180deg);
                    }
                }
            }
        }
    }
}
</style>

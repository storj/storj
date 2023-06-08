// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <Teleport to="#app">
        <div ref="viewContainer" class="gallery" tabindex="0" @keydown.esc="closeModal">
            <div class="gallery__header">
                <LogoIcon class="gallery__header__logo" />
                <SmallLogoIcon class="gallery__header__small-logo" />
                <div class="gallery__header__name">
                    <ImageIcon v-if="previewIsImage" />
                    <VideoIcon v-else-if="previewIsAudio || previewIsVideo" />
                    <EmptyIcon v-else />
                    <p class="gallery__header__name__label" :title="file?.Key || ''">{{ file?.Key || '' }}</p>
                </div>
                <div class="gallery__header__functional">
                    <ButtonIcon
                        v-click-outside="closeDropdown"
                        :icon="DotsIcon"
                        :on-press="toggleDropdown"
                        :is-active="isOptionsDropdown === true"
                    />
                    <ButtonIcon :icon="MapIcon" :on-press="() => setActiveModal(DistributionModal)" />
                    <ButtonIcon class="gallery__header__functional__item" :icon="DownloadIcon" :on-press="download" />
                    <ButtonIcon class="gallery__header__functional__item" :icon="ShareIcon" :on-press="() => setActiveModal(ShareModal)" />
                    <ButtonIcon :icon="CloseIcon" :on-press="closeModal" />
                    <OptionsDropdown
                        v-if="isOptionsDropdown"
                        :on-view-details="() => setActiveModal(DetailsModal)"
                        :on-download="download"
                        :on-share="() => setActiveModal(ShareModal)"
                        :on-delete="() => setActiveModal(DeleteModal)"
                    />
                </div>
            </div>
            <div class="gallery__main">
                <ArrowIcon class="gallery__main__left-arrow" @click="onPrevious" />
                <VLoader v-if="isLoading" class="gallery__main__loader" width="100px" height="100px" is-white />
                <div v-else class="gallery__main__preview">
                    <img
                        v-if="previewIsImage && !isLoading"
                        :src="objectPreviewUrl"
                        class="gallery__main__preview__item"
                        aria-roledescription="image-preview"
                        alt="preview"
                    >
                    <video
                        v-if="previewIsVideo && !isLoading"
                        controls
                        :src="objectPreviewUrl"
                        class="gallery__main__preview__item"
                        aria-roledescription="video-preview"
                    />
                    <audio
                        v-if="previewIsAudio && !isLoading"
                        controls
                        :src="objectPreviewUrl"
                        class="gallery__main__preview__item"
                        aria-roledescription="audio-preview"
                    />
                    <div v-if="placeHolderDisplayable || previewAndMapFailed" class="gallery__main__preview__empty">
                        <p class="gallery__main__preview__empty__key">{{ file?.Key || '' }}</p>
                        <p class="gallery__main__preview__empty__label">No preview available</p>
                        <VButton
                            icon="download"
                            :label="`Download (${prettyBytes(file?.Size || 0)})`"
                            :on-press="download"
                            width="188px"
                            height="52px"
                            border-radius="10px"
                            font-size="14px"
                        />
                    </div>
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
import { Component, computed, onBeforeMount, onMounted, ref, Teleport, watch } from 'vue';
import { useRoute } from 'vue-router';
import prettyBytes from 'pretty-bytes';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useAppStore } from '@/store/modules/appStore';
import { useNotify } from '@/utils/hooks';
import { RouteConfig } from '@/types/router';

import ButtonIcon from '@/components/browser/galleryView/ButtonIcon.vue';
import OptionsDropdown from '@/components/browser/galleryView/OptionsDropdown.vue';
import DeleteModal from '@/components/browser/galleryView/modals/Delete.vue';
import ShareModal from '@/components/browser/galleryView/modals/Share.vue';
import DetailsModal from '@/components/browser/galleryView/modals/Details.vue';
import DistributionModal from '@/components/browser/galleryView/modals/Distribution.vue';
import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';

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
const notify = useNotify();

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
 * Format the file size to be displayed.
 */
const size = computed((): string => {
    return prettyBytes(obStore.sortedFiles.find(f => f.Key === file.value.Key)?.Size || 0);
});

/**
 * Retrieve the filepath of the modal from the store.
 */
const filePath = computed((): string => {
    return obStore.state.objectPathForModal;
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
    if (!extension.value) {
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
    if (!extension.value) {
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
    if (!extension.value) {
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
 * Returns current path without object key.
 */
const currentPath = computed((): string => {
    return route.path.replace(RouteConfig.Buckets.with(RouteConfig.UploadFile).path, '');
});

/**
 * Get the object map url for the file being displayed.
 */
async function fetchPreviewAndMapUrl(): Promise<void> {
    isLoading.value = true;

    const url: string = await obStore.state.fetchPreviewAndMapUrl(filePath.value);
    if (!url) {
        previewAndMapFailed.value = true;
        isLoading.value = false;

        return;
    }

    objectMapUrl.value = `${url}?map=1`;
    objectPreviewUrl.value = `${url}?view=1`;
    isLoading.value = false;
}

/**
 * Deletes active object.
 */
async function onDelete(): Promise<void> {
    try {
        const objectsCount = obStore.sortedFiles.length;
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
        notify.warning('Do not share download link with other people. If you want to share this data better use "Share" option.');
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
 * Call `fetchPreviewAndMapUrl` on before mount lifecycle method.
 */
onBeforeMount((): void => {
    fetchPreviewAndMapUrl();
});

onMounted((): void => {
    viewContainer.value?.focus();
});

/**
 * Watch for changes on the filepath and call `fetchObjectMapUrl` the moment it updates.
 */
watch(filePath, () => {
    if (!filePath.value) return;

    fetchPreviewAndMapUrl();
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

            &__empty {
                width: 100%;
                align-self: center;
                display: flex;
                align-items: center;
                flex-direction: column;

                &__key {
                    font-size: 16px;
                    line-height: 24px;
                    color: var(--c-white);
                    margin-bottom: 14px;
                }

                &__label {
                    font-family: 'font_bold', sans-serif;
                    font-size: 28px;
                    line-height: 36px;
                    letter-spacing: -0.02em;
                    color: var(--c-white);
                    margin-bottom: 17px;
                }
            }

            &__buttons {
                display: none;

                svg {
                    width: 30px;
                    height: 30px;
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

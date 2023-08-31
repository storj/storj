// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container v-if="isLoading" class="fill-height flex-column justify-center align-center">
        <v-progress-circular indeterminate />
    </v-container>
    <v-container v-else-if="previewIsVideo" class="fill-height flex-column justify-center align-center">
        <video
            controls
            :src="objectPreviewUrl"
            style="max-width: 100%; max-height: 90%;"
            aria-roledescription="video-preview"
        />
    </v-container>
    <v-container v-else-if="previewIsAudio" class="fill-height flex-column justify-center align-center">
        <audio
            controls
            :src="objectPreviewUrl"
            aria-roledescription="audio-preview"
        />
    </v-container>
    <v-container v-else-if="previewIsImage" class="fill-height flex-column justify-center align-center">
        <img
            v-if="objectPreviewUrl"
            :src="objectPreviewUrl"
            class="v-img__img v-img__img--contain"
            aria-roledescription="image-preview"
            alt="preview"
        >
    </v-container>
    <v-container v-else-if="placeHolderDisplayable || previewAndMapFailed" class="fill-height flex-column justify-center align-center">
        <p class="mb-5">{{ file?.Key || '' }}</p>
        <p class="text-h5 mb-5 font-weight-bold">No preview available</p>
        <v-btn
            @click="() => emits('download')"
        >
            <template #prepend>
                <img src="@poc/assets/icon-download.svg" width="22" alt="Download">
            </template>
            {{ `Download (${prettyBytes(file?.Size || 0)})` }}
        </v-btn>
    </v-container>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VContainer, VProgressCircular } from 'vuetify/components';
import { useRoute } from 'vue-router';
import prettyBytes from 'pretty-bytes';

import { useAppStore } from '@/store/modules/appStore';
import { BrowserObject, PreviewCache, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLinksharing } from '@/composables/useLinksharing';

const appStore = useAppStore();
const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const notify = useNotify();
const { generateObjectPreviewAndMapURL } = useLinksharing();

const route = useRoute();

const isLoading = ref<boolean>(false);
const previewAndMapFailed = ref<boolean>(false);

const imgExts = ['bmp', 'svg', 'jpg', 'jpeg', 'png', 'ico', 'gif'];
const videoExts = ['m4v', 'mp4', 'webm', 'mov', 'mkv'];
const audioExts = ['m4a', 'mp3', 'wav', 'ogg'];

const props = defineProps<{
  file: BrowserObject,
  active: boolean, // whether this item is visible
}>();

const emits = defineEmits(['download']);

/**
 * Returns object preview URLs cache from store.
 */
const cachedObjectPreviewURLs = computed((): Map<string, PreviewCache> => {
    return obStore.state.cachedObjectPreviewURLs;
});

/**
 * Returns object preview URL from cache.
 */
const objectPreviewUrl = computed((): string => {
    const cache = cachedObjectPreviewURLs.value.get(encodedFilePath.value);
    const url = cache?.url || '';
    return `${url}?view=1`;
});

/**
 * Returns bucket name from store.
 */
const bucket = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Retrieve the current filepath.
 */
const filePath = computed((): string => {
    return obStore.state.objectPathForModal;
});

/**
 * Retrieve the encoded filepath.
 */
const encodedFilePath = computed((): string => {
    return encodeURIComponent(`${bucket.value}/${filePath.value.trim()}`);
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

    return imgExts.includes(extension.value.toLowerCase());
});

/**
 * Check to see if the current file is a video file.
 */
const previewIsVideo = computed((): boolean => {
    if (!extension.value) {
        return false;
    }

    return videoExts.includes(extension.value.toLowerCase());
});

/**
 * Check to see if the current file is an audio file.
 */
const previewIsAudio = computed((): boolean => {
    if (!extension.value) {
        return false;
    }

    return audioExts.includes(extension.value.toLowerCase());
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
    previewAndMapFailed.value = false;

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
    obStore.cacheObjectPreviewURL(encodedFilePath.value, { url, lastModified: props.file.LastModified.getTime() });

    isLoading.value = false;
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
}

/**
 * Try to find current object path in cache.
 */
function findCachedURL(): string | undefined {
    const cache = cachedObjectPreviewURLs.value.get(encodedFilePath.value);

    if (!cache) return undefined;

    if (cache.lastModified !== props.file.LastModified.getTime()) {
        obStore.removeFromObjectPreviewCache(encodedFilePath.value);
        return undefined;
    }

    return cache.url;
}

watch(() => props.active, active => {
    if (active) {
        processFilePath();
    }
}, { immediate: true });
</script>

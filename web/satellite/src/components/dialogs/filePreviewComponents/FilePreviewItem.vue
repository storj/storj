// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container v-if="isLoading" class="fill-height flex-column justify-center align-center">
        <v-progress-circular indeterminate />
    </v-container>
    <text-file-preview v-else-if="!loadError && previewType === PreviewType.Text" :src="objectPreviewUrl">
        <file-preview-placeholder :file="file" @download="emit('download')" />
    </text-file-preview>
    <c-s-v-file-preview v-else-if="!loadError && previewType === PreviewType.CSV" :src="objectPreviewUrl">
        <file-preview-placeholder :file="file" @download="emit('download')" />
    </c-s-v-file-preview>
    <v-container v-else-if="!loadError && previewType === PreviewType.Video" class="fill-height flex-column justify-center align-center">
        <video
            controls
            :src="objectPreviewUrl"
            class="video"
            aria-roledescription="video-preview"
            :autoplay="videoAutoplay"
            :muted="videoAutoplay"
            @error="loadError = true"
        />
    </v-container>
    <v-container v-else-if="!loadError && previewType === PreviewType.Audio" class="fill-height flex-column justify-center align-center">
        <audio
            controls
            :src="objectPreviewUrl"
            aria-roledescription="audio-preview"
            @error="loadError = true"
        />
    </v-container>
    <v-container v-else-if="!loadError && previewType === PreviewType.Image" class="fill-height flex-column justify-center align-center">
        <img
            v-if="objectPreviewUrl && !loadError"
            :src="objectPreviewUrl"
            class="v-img__img v-img__img--contain"
            aria-roledescription="image-preview"
            alt="preview"
            @error="loadError = true"
        >
        <file-preview-placeholder v-else :file="file" @download="emit('download')" />
    </v-container>
    <v-container v-else-if="!loadError && previewType === PreviewType.PDF" class="fill-height flex-column justify-center align-center">
        <object
            :data="objectPreviewUrl"
            type="application/pdf"
            aria-roledescription="pdf-preview"
            class="h-100 w-100"
        >
            <file-preview-placeholder :file="file" @download="emit('download')" />
        </object>
    </v-container>
    <file-preview-placeholder v-else :file="file" @download="emit('download')" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VContainer, VProgressCircular } from 'vuetify/components';

import { BrowserObject, PreviewCache, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { EXTENSION_PREVIEW_TYPES, PreviewType } from '@/types/browser';

import FilePreviewPlaceholder from '@/components/dialogs/filePreviewComponents/FilePreviewPlaceholder.vue';
import TextFilePreview from '@/components/dialogs/filePreviewComponents/TextFilePreview.vue';
import CSVFilePreview from '@/components/dialogs/filePreviewComponents/CSVFilePreview.vue';

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const notify = useNotify();

const isLoading = ref<boolean>(false);
const loadError = ref<boolean>(false);
const previewAndMapFailed = ref<boolean>(false);

const props = withDefaults(defineProps<{
    file: BrowserObject,
    active: boolean, // whether this item is visible
    videoAutoplay?: boolean
    showingVersion?: boolean
}>(), {
    videoAutoplay: false,
    showingVersion: false,
});

const emit = defineEmits<{
    download: [];
}>();

/**
 * Returns object preview URLs cache from store.
 */
const cachedObjectPreviewURLs = computed((): Map<string, PreviewCache> => {
    return obStore.state.cachedObjectPreviewURLs;
});

const cacheKey = computed(() => props.showingVersion ? props.file.VersionId ?? '' : encodedFilePath.value);

/**
 * Returns object preview URL from cache.
 */
const objectPreviewUrl = computed((): string => {
    const cache = cachedObjectPreviewURLs.value.get(cacheKey.value);
    return cache?.url ?? '';
});

/**
 * Returns bucket name from store.
 */
const bucket = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Retrieve the encoded filepath.
 */
const encodedFilePath = computed((): string => {
    return encodeURIComponent(`${bucket.value}/${props.file.path}${props.file.Key}`);
});

/**
 * Returns the type of object being previewed.
 */
const previewType = computed<PreviewType>(() => {
    if (previewAndMapFailed.value) return PreviewType.None;

    const dotIdx = props.file.Key.lastIndexOf('.');
    if (dotIdx === -1) return PreviewType.None;

    const ext = props.file.Key.toLowerCase().slice(dotIdx + 1);
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
    previewAndMapFailed.value = false;

    let url = '';
    try {
        url = await obStore.getDownloadLink(props.file);
    } catch (error) {
        error.message = `Unable to get file preview and map URL. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.GALLERY_VIEW);
    }

    if (!url) {
        previewAndMapFailed.value = true;
        isLoading.value = false;
        return;
    }
    obStore.cacheObjectPreviewURL(cacheKey.value, { url, lastModified: props.file.LastModified.getTime() });

    isLoading.value = false;
}

/**
 * Loads object URL from cache or generates new URL.
 */
function processFilePath(): void {
    const url = findCachedURL();
    if (!url) fetchPreviewAndMapUrl();
}

/**
 * Try to find current object path in cache.
 */
function findCachedURL(): string | undefined {
    const cache = cachedObjectPreviewURLs.value.get(cacheKey.value);

    if (!cache) return undefined;

    if (cache.lastModified !== props.file.LastModified.getTime()) {
        obStore.removeFromObjectPreviewCache(cacheKey.value);
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

<style scoped lang="scss">
.video {
    max-width: 100%;
    max-height: 90%;
}
</style>

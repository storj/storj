// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" rounded="md">
        <div class="h-100 d-flex flex-column justify-space-between">
            <a role="button" class="h-100" @click="previewClicked">
                <template v-if="previewType === PreviewType.Image">
                    <v-img
                        :src="objectPreviewUrl"
                        class="bg-light card-preview-img h-100"
                        alt="preview"
                        :aspect-ratio="1"
                        cover
                        :draggable="false"
                    >
                        <template #placeholder>
                            <v-progress-linear indeterminate />
                        </template>
                        <template #error>
                            <div
                                class="d-flex h-100 bg-light flex-column justify-center align-center file-icon-container card-preview-icon"
                            >
                                <img
                                    :src="item.typeInfo.icon"
                                    :alt="item.typeInfo.title + 'icon'"
                                    :aria-roledescription="item.typeInfo.title + 'icon'"
                                    height="70"
                                >
                            </div>
                        </template>
                    </v-img>
                </template>
                <template v-else-if="previewType === PreviewType.Video">
                    <div class="pos-relative">
                        <video
                            ref="videoEl"
                            :src="objectPreviewUrl"
                            class="bg-light h-100 custom-video"
                            muted
                            @loadedmetadata="captureVideoFrame"
                        />
                        <div class="absolute w-100 h-100">
                            <div
                                class="d-flex h-100 bg-light flex-column justify-center align-center file-icon-container card-preview-icon"
                            ><img
                                :src="item.typeInfo.icon"
                                :alt="item.typeInfo.title + 'icon'"
                                :aria-roledescription="item.typeInfo.title + 'icon'"
                                class="bg-blue6 rounded-xxlg"
                                height="70"
                            >
                            </div>
                        </div>
                    </div>
                </template>
                <div
                    v-else
                    class="d-flex h-100 bg-light flex-column justify-center align-center file-icon-container card-preview-icon"
                >
                    <img
                        :src="item.typeInfo.icon"
                        :alt="item.typeInfo.title + 'icon'"
                        :aria-roledescription="item.typeInfo.title + 'icon'"
                        height="70"
                    >
                </div>
            </a>

            <browser-row-actions
                class="pl-3 pt-3"
                :file="item.browserObject"
                :deleting="filesBeingDeleted.has(item.browserObject.path + item.browserObject.Key)"
                align="left"
                @preview-click="emit('previewClick', item.browserObject)"
                @delete-file-click="emit('deleteFileClick', item.browserObject)"
                @share-click="emit('shareClick', item.browserObject)"
                @lock-object-click="emit('lockObjectClick', item.browserObject)"
                @legal-hold-click="emit('legalHoldClick', item.browserObject)"
                @locked-object-delete="(fullObject) => emit('lockedObjectDelete', fullObject)"
                @download-folder-click="emit('downloadFolderClick', item.browserObject)"
            />
            <v-card-item class="pt-0">
                <v-card-title>
                    <small
                        class="link text-decoration-none"
                        :title="item.browserObject.Key"
                        @click="() => emit('previewClick', item.browserObject)"
                    >
                        {{ item.browserObject.Key }}
                    </small>
                </v-card-title>
                <v-card-subtitle class="text-caption">
                    {{ item.browserObject.type === 'folder' ? '&nbsp;': getFormattedDate(item.browserObject) }}
                </v-card-subtitle>
            </v-card-item>
        </div>
    </v-card>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VCard, VCardItem, VCardSubtitle, VCardTitle, VImg, VProgressLinear } from 'vuetify/components';

import {
    BrowserObject,
    FullBrowserObject,
    PreviewCache,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { EXTENSION_PREVIEW_TYPES, PreviewType } from '@/types/browser';
import { Time } from '@/utils/time';

import BrowserRowActions from '@/components/BrowserRowActions.vue';

type BrowserObjectTypeInfo = {
    title: string;
    icon: string;
};

type BrowserObjectWrapper = {
    browserObject: BrowserObject;
    typeInfo: BrowserObjectTypeInfo;
    lowerName: string;
    ext: string;
};

const bucketsStore = useBucketsStore();
const obStore = useObjectBrowserStore();

const props = defineProps<{
    item: BrowserObjectWrapper,
}>();

const emit = defineEmits<{
    previewClick: [BrowserObject];
    deleteFileClick: [BrowserObject];
    shareClick: [BrowserObject];
    lockObjectClick: [BrowserObject];
    legalHoldClick: [BrowserObject];
    lockedObjectDelete: [FullBrowserObject];
    downloadFolderClick: [BrowserObject];
}>();

const videoEl = ref<HTMLVideoElement>();

/**
 * Returns object preview URL from cache.
 */
const objectPreviewUrl = computed((): string => {
    const cache = cachedObjectPreviewURLs.value.get(encodedFilePath.value);
    return cache?.url ?? '';
});

/**
 * Returns object preview URLs cache from store.
 */
const cachedObjectPreviewURLs = computed((): Map<string, PreviewCache> => {
    return obStore.state.cachedObjectPreviewURLs;
});

/**
 * Retrieve the encoded objectpath.
 */
const encodedFilePath = computed((): string => {
    return encodeURIComponent(`${bucket.value}/${props.item.browserObject.path}${props.item.browserObject.Key}`);
});

/**
 * Returns bucket name from store.
 */
const bucket = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Returns the type of object being previewed.
 */
const previewType = computed<PreviewType>(() => {
    const dotIdx = props.item.browserObject.Key.lastIndexOf('.');
    if (dotIdx === -1) return PreviewType.None;

    const ext = props.item.browserObject.Key.toLowerCase().slice(dotIdx + 1);
    for (const [exts, previewType] of EXTENSION_PREVIEW_TYPES) {
        if (exts.includes(ext)) return previewType;
    }

    return PreviewType.None;
});

/**
 * Returns objects being deleted from store.
 */
const filesBeingDeleted = computed((): Set<string> => obStore.state.filesToBeDeleted);

/**
 * Updates current video time to show video thumbnail.
 */
function captureVideoFrame(): void {
    if (videoEl.value) videoEl.value.currentTime = 0;
}

/**
 * Returns the string form of the file's last modified date.
 */
function getFormattedDate(file: BrowserObject): string {
    return Time.formattedDate(file.LastModified);
}

function previewClicked() {
    if (filesBeingDeleted.value.has(props.item.browserObject.path + props.item.browserObject.Key)) {
        return;
    }
    emit('previewClick', props.item.browserObject);
}
</script>

<style scoped lang="scss">
.custom-video {
    max-width: 100%;
    max-height: 100%;
    aspect-ratio: 1;
    object-fit: contain;
}

.absolute {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    z-index: 1;
}
</style>

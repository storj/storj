// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <div class="h-100 d-flex flex-column justify-space-between">
            <v-img
                v-if="objectPreviewUrl && previewType === PreviewType.Image"
                :src="objectPreviewUrl"
                alt="preview"
                height="200px"
                cover
                @click="emit('previewClick', item.browserObject)"
            />
            <div
                v-else
                class="d-flex flex-column justify-center align-center file-icon-container"
                @click="emit('previewClick', item.browserObject)"
            >
                <img
                    :src="item.typeInfo.icon"
                    :alt="item.typeInfo.title + 'icon'"
                    :aria-roledescription="item.typeInfo.title + 'icon'"
                    height="100"
                >
            </div>

            <v-card-item>
                <v-card-title>
                    <a class="link" @click="emit('previewClick', item.browserObject)">
                        {{ item.browserObject.Key }}
                    </a>
                </v-card-title>
                <v-card-subtitle>
                    {{ item.browserObject.type === 'folder' ? '&nbsp;': getFormattedSize(item.browserObject) }}
                </v-card-subtitle>
                <v-card-subtitle>
                    {{ item.browserObject.type === 'folder' ? '&nbsp;': getFormattedDate(item.browserObject) }}
                </v-card-subtitle>
            </v-card-item>
            <v-card-text class="flex-grow-0">
                <v-divider class="mt-1 mb-4" />
                <browser-row-actions
                    :file="item.browserObject"
                    align="left"
                    @preview-click="emit('previewClick', item.browserObject)"
                    @delete-file-click="emit('deleteFileClick', item.browserObject)"
                    @share-click="emit('shareClick', item.browserObject)"
                />
            </v-card-text>
        </div>
    </v-card>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
import { VCard, VCardItem, VCardSubtitle, VCardText, VCardTitle, VDivider, VImg } from 'vuetify/components';
import { computed } from 'vue';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';
import { BrowserObject, PreviewCache, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { Size } from '@/utils/bytesSize';
import { EXTENSION_PREVIEW_TYPES, PreviewType } from '@/types/browser';

import BrowserRowActions from '@poc/components/BrowserRowActions.vue';

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

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const obStore = useObjectBrowserStore();

const router = useRouter();
const notify = useNotify();

const props = defineProps<{
    item: BrowserObjectWrapper,
}>();

const emit = defineEmits<{
  previewClick: [BrowserObject];
  deleteFileClick: [BrowserObject];
  shareClick: [BrowserObject];
}>();

/**
 * Returns object preview URL from cache.
 */
const objectPreviewUrl = computed((): string => {
    const cache = cachedObjectPreviewURLs.value.get(encodedFilePath.value);
    const url = cache?.url || '';
    return `${url}?view=1`;
});

/**
 * Returns object preview URLs cache from store.
 */
const cachedObjectPreviewURLs = computed((): Map<string, PreviewCache> => {
    return obStore.state.cachedObjectPreviewURLs;
});

/**
 * Retrieve the encoded filepath.
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
 * Returns the string form of the file's size.
 */
function getFormattedSize(file: BrowserObject): string {
    const size = new Size(file.Size);
    return `${size.formattedBytes} ${size.label}`;
}

/**
 * Returns the string form of the file's last modified date.
 */
function getFormattedDate(file: BrowserObject): string {
    const date = file.LastModified;
    return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
}
</script>

<style scoped lang="scss">
.file-icon-container {
    height: 200px;
}
</style>
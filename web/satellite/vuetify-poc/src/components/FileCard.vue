// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <div class="h-100 d-flex flex-column justify-space-between">
            <v-container v-if="isLoading" class="fill-height flex-column justify-center align-center mt-n16">
                <v-progress-circular indeterminate />
            </v-container>
            <div
                v-else
                class="d-flex flex-column justify-center align-center file-icon-container"
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
                    {{ getFormattedSize(item.browserObject) }}
                </v-card-subtitle>
                <v-card-subtitle>
                    {{ getFormattedDate(item.browserObject) }}
                </v-card-subtitle>
            </v-card-item>
            <v-card-text class="flex-grow-0">
                <v-divider class="mt-1 mb-4" />
                <browser-row-actions
                    :file="item.browserObject"
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
import {
    VCard,
    VCardItem,
    VCardSubtitle,
    VCardText,
    VCardTitle,
    VContainer,
    VDivider,
    VProgressCircular,
} from 'vuetify/components';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { Size } from '@/utils/bytesSize';
import { useLoading } from '@/composables/useLoading';

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

const { isLoading, withLoading } = useLoading();

const props = defineProps<{
    item: BrowserObjectWrapper,
}>();

const emit = defineEmits<{
  previewClick: [BrowserObject];
  deleteFileClick: [BrowserObject];
  shareClick: [BrowserObject];
}>();

/**
 * Returns the string form of the file's size.
 */
function getFormattedSize(file: BrowserObject): string {
    if (file.type === 'folder') return '---';
    const size = new Size(file.Size);
    return `${size.formattedBytes} ${size.label}`;
}

/**
 * Returns the string form of the file's last modified date.
 */
function getFormattedDate(file: BrowserObject): string {
    if (file.type === 'folder') return '---';
    const date = file.LastModified;
    return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
}
</script>

<style scoped lang="scss">
.file-icon-container {
    height: 200px;
}
</style>
// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent">
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-share size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Share {{ shareText }}
                </v-card-title>
                <template #append>
                    <v-btn
                        id="Close"
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <div class="pa-6 share-dialog__content" :class="{ 'share-dialog__content--loading': isLoading }">
                <v-row>
                    <v-col cols="12">
                        <v-alert type="info" variant="tonal">
                            Public link sharing. Allows anyone with the link to view your shared {{ shareText.toLowerCase() }}.
                        </v-alert>
                    </v-col>
                    <v-col cols="12">
                        <p class="text-subtitle-2 font-weight-bold mb-1">Share via</p>
                        <v-chip-group class="mx-n1" column>
                            <v-chip
                                v-for="opt in ShareOptions"
                                :key="opt"
                                :color="SHARE_BUTTON_CONFIGS[opt].color"
                                :href="SHARE_BUTTON_CONFIGS[opt].getLink(link)"
                                target="_blank"
                                rel="noopener noreferrer"
                                link
                                class="ma-1 font-weight-medium"
                            >
                                <component
                                    :is="SHARE_BUTTON_CONFIGS[opt].icon"
                                    class="share-dialog__content__icon"
                                    size="16"
                                />
                                {{ opt }}
                            </v-chip>
                        </v-chip-group>
                    </v-col>

                    <v-col cols="12">
                        <p class="text-subtitle-2 font-weight-bold mb-2">Shared link</p>
                        <v-textarea :model-value="link" variant="solo-filled" rounded="lg" hide-details="auto" rows="1" auto-grow no-resize flat readonly class="text-caption">
                            <template #append-inner>
                                <input-copy-button :value="link" />
                            </template>
                        </v-textarea>
                    </v-col>
                </v-row>
            </div>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Close
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            :color="justCopied ? 'success' : 'primary'"
                            variant="flat"
                            :prepend-icon="justCopied ? mdiCheck : mdiContentCopy"
                            :disabled="isLoading"
                            block
                            @click="onCopy"
                        >
                            {{ justCopied ? 'Copied' : 'Copy Link' }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { Component, computed, ref, watch } from 'vue';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VChip,
    VChipGroup,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
    VTextarea,
} from 'vuetify/components';
import { mdiCheck, mdiContentCopy } from '@mdi/js';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';
import { ShareType, useLinksharing } from '@/composables/useLinksharing';
import { SHARE_BUTTON_CONFIGS, ShareOptions } from '@/types/browser';
import { BrowserObject } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import IconShare from '@/components/icons/IconShare.vue';
import InputCopyButton from '@/components/InputCopyButton.vue';

const props = defineProps<{
    bucketName: string,
    file?: BrowserObject,
}>();

const emit = defineEmits<{
    'contentRemoved': [];
}>();

const model = defineModel<boolean>({ required: true });

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();

const notify = useNotify();
const { generateBucketShareURL, generateFileOrFolderShareURL } = useLinksharing();

const innerContent = ref<Component | null>(null);
const isLoading = ref<boolean>(true);
const link = ref<string>('');

const copiedTimeout = ref<ReturnType<typeof setTimeout> | null>(null);
const justCopied = computed<boolean>(() => copiedTimeout.value !== null);

const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

const shareText = computed<string>(() => !props.file ? 'Bucket' : props.file.type === 'folder' ? 'Folder' : 'File');

/**
 * Saves link to clipboard.
 */
function onCopy(): void {
    navigator.clipboard.writeText(link.value);
    analyticsStore.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);

    if (copiedTimeout.value) clearTimeout(copiedTimeout.value);
    copiedTimeout.value = setTimeout(() => {
        copiedTimeout.value = null;
    }, 750);
}

/**
 * Generates linksharing URL when the dialog is opened.
 */
watch(() => innerContent.value, async (comp: Component | null): Promise<void> => {
    if (!comp) {
        emit('contentRemoved');
        return;
    }

    isLoading.value = true;
    link.value = '';
    analyticsStore.eventTriggered(AnalyticsEvent.LINK_SHARED);

    try {
        if (!props.file) {
            link.value = await generateBucketShareURL(props.bucketName);
        } else {
            link.value = await generateFileOrFolderShareURL(
                props.bucketName,
                filePath.value,
                props.file.Key,
                // TODO: replace magic string type of BrowserObject.type with some constant/enum.
                props.file.type === 'folder' ? ShareType.Folder : ShareType.Object,
            );
        }
    } catch (error) {
        error.message = `Unable to get sharing URL. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.SHARE_MODAL);
        model.value = false;
        return;
    }

    isLoading.value = false;
});
</script>

<style scoped lang="scss">
.share-dialog__content {
    transition: opacity 250ms cubic-bezier(0.4, 0, 0.2, 1);

    &--loading {
        opacity: 0.3;
        transition: opacity 0s;
        pointer-events: none;
    }

    &__icon {
        margin-right: 5.5px;
    }
}
</style>

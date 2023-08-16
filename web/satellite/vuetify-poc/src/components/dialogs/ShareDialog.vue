// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-card-item class="pl-7 py-4 share-dialog__header">
                <template #prepend>
                    <v-sheet
                        class="bg-on-surface-variant d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-share size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Share {{ !filePath ? 'Bucket' : isFolder ? 'Folder' : 'File' }}
                </v-card-title>
                <template #append>
                    <v-btn
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

            <div class="pa-7 share-dialog__content" :class="{ 'share-dialog__content--loading': isLoading }">
                <v-row>
                    <v-col cols="12">
                        <p class="text-subtitle-2 font-weight-bold mb-4">Share via</p>
                        <div class="ma-n2">
                            <v-chip
                                v-for="opt in ShareOptions"
                                :key="opt"
                                :color="SHARE_BUTTON_CONFIGS[opt].color"
                                :href="SHARE_BUTTON_CONFIGS[opt].getLink(link)"
                                link
                                class="ma-2"
                            >
                                <component
                                    :is="SHARE_BUTTON_CONFIGS[opt].icon"
                                    class="share-dialog__content__icon"
                                    size="21"
                                />
                                {{ opt }}
                            </v-chip>
                        </div>
                    </v-col>

                    <v-divider class="my-2" />

                    <v-col cols="12">
                        <p class="text-subtitle-2 font-weight-bold mb-2">Copy link</p>
                        <v-textarea :model-value="link" variant="solo-filled" rows="1" auto-grow no-resize flat readonly />
                    </v-col>
                </v-row>
            </div>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            :color="justCopied ? 'success' : 'primary'"
                            variant="flat"
                            :prepend-icon="justCopied ? 'mdi-check' : 'mdi-content-copy'"
                            :disabled="isLoading"
                            block
                            @click="onCopy"
                        >
                            {{ justCopied ? 'Copied' : 'Copy' }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, Component } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VSheet,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VChip,
    VTextarea,
} from 'vuetify/components';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';
import { useLinksharing } from '@/composables/useLinksharing';
import { SHARE_BUTTON_CONFIGS, ShareOptions } from '@/types/browser';

import IconShare from '@poc/components/icons/IconShare.vue';

const props = defineProps<{
    modelValue: boolean,
    bucketName: string;
    filePath?: string;
    isFolder?: boolean;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean],
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const analyticsStore = useAnalyticsStore();

const notify = useNotify();
const { generateBucketShareURL, generateFileOrFolderShareURL } = useLinksharing();

const innerContent = ref<Component | null>(null);
const isLoading = ref<boolean>(true);
const link = ref<string>('');

const copiedTimeout = ref<ReturnType<typeof setTimeout> | null>(null);
const justCopied = computed<boolean>(() => copiedTimeout.value !== null);

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
    if (!comp) return;

    isLoading.value = true;
    link.value = '';
    analyticsStore.eventTriggered(AnalyticsEvent.LINK_SHARED);

    try {
        if (!props.filePath) {
            link.value = await generateBucketShareURL(props.bucketName);
        } else {
            link.value = await generateFileOrFolderShareURL(
                props.bucketName,
                props.filePath,
                props.isFolder,
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
.share-dialog {

    &__header {
        position: relative;
    }

    &__content {
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
}
</style>

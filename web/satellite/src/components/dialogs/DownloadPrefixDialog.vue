// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="Download" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Download {{ prefixType }}
                </v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>
            <v-divider />
            <div class="pa-6 download-dialog__content" :class="{ 'download-dialog__content--loading': isLoading }">
                <v-row>
                    <v-col cols="12">
                        <v-card
                            class="mb-4"
                            :class="{'border-colored': downloadFormat === DownloadPrefixFormat.ZIP}"
                            variant="outlined"
                            rounded="lg"
                            :color="downloadFormat === DownloadPrefixFormat.ZIP ? 'primary' : undefined"
                            @click="downloadFormat = DownloadPrefixFormat.ZIP"
                        >
                            <v-card-item>
                                <template #prepend>
                                    <component :is="FolderArchive" :size="18" class="mr-2" />
                                </template>
                                <v-card-title class="d-flex align-center">
                                    .zip format
                                    <component
                                        :is="Check"
                                        v-if="downloadFormat === DownloadPrefixFormat.ZIP"
                                        :size="18"
                                        class="ml-2"
                                    />
                                </v-card-title>
                                <v-card-subtitle class="text-wrap pb-2" :class="{'opacity-100': downloadFormat === DownloadPrefixFormat.ZIP}">
                                    Universal compatibility, no extra software needed.
                                </v-card-subtitle>
                                <v-card-text class="pa-0">
                                    <v-chip size="small" color="success" variant="flat">Limited to {{ zipDownloadLimit.toLocaleString() }} objects</v-chip>
                                </v-card-text>
                            </v-card-item>
                        </v-card>
                        <v-card
                            :class="{'border-colored': downloadFormat === DownloadPrefixFormat.TAR_GZ}"
                            variant="outlined"
                            rounded="lg"
                            :color="downloadFormat === DownloadPrefixFormat.TAR_GZ ? 'primary' : undefined"
                            @click="downloadFormat = DownloadPrefixFormat.TAR_GZ"
                        >
                            <v-card-item>
                                <template #prepend>
                                    <component :is="FolderArchive" :size="18" class="mr-2" />
                                </template>
                                <v-card-title class="d-flex align-center">
                                    .tar.gz format
                                    <component
                                        :is="Check"
                                        v-if="downloadFormat === DownloadPrefixFormat.TAR_GZ"
                                        :size="18"
                                        class="ml-2"
                                    />
                                </v-card-title>
                                <v-card-subtitle class="text-wrap pb-2" :class="{'opacity-100': downloadFormat === DownloadPrefixFormat.TAR_GZ}">
                                    Better compression, suitable for large downloads.
                                </v-card-subtitle>
                                <v-card-text class="pa-0">
                                    <v-chip size="small" color="success" variant="flat">No object limit</v-chip>
                                </v-card-text>
                            </v-card-item>
                        </v-card>
                    </v-col>
                </v-row>
            </div>
            <v-divider />
            <v-card-actions class="pa-6">
                <v-row>
                    <v-col cols="12">
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :disabled="isLoading"
                            @click="onDownload"
                        >
                            <component :is="Download" :size="18" class="mr-2" />
                            Download as {{ downloadFormat }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, computed, h, watch } from 'vue';
import { FolderArchive, Check, Download, X } from 'lucide-vue-next';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardActions,
    VCardText,
    VCardTitle,
    VCardSubtitle,
    VCol,
    VRow,
    VBtn,
    VDivider,
    VSheet,
    VChip,
} from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { useLoading } from '@/composables/useLoading';
import { usePreCheck } from '@/composables/usePreCheck';
import { useNotify } from '@/composables/useNotify';
import { useLinksharing } from '@/composables/useLinksharing';
import { DownloadPrefixFormat, DownloadPrefixType } from '@/types/browser';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { EdgeCredentials } from '@/types/accessGrants';
import { useConfigStore } from '@/store/modules/configStore';

const props = withDefaults(defineProps<{
    prefixType: DownloadPrefixType
    bucket: string
    prefix?: string
}>(), {
    prefix: '',
});

const notify = useNotify();
const { withTrialCheck, withManagedPassphraseCheck } = usePreCheck();
const { isLoading, withLoading } = useLoading();
const { downloadPrefix } = useLinksharing();
const { platform } = useDisplay();

const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();

const model = defineModel<boolean>({ required: true });

const downloadFormat = ref<DownloadPrefixFormat>(DownloadPrefixFormat.ZIP);

const zipDownloadLimit = computed<number>(() => configStore.state.config.zipDownloadLimit);

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => bucketsStore.state.edgeCredentials);

/**
 * Handles download action.
 */
async function onDownload(): Promise<void> {
    withTrialCheck(() => { withManagedPassphraseCheck(async () => {
        await withLoading(async () => {
            if (!edgeCredentials.value.accessKeyId) {
                try {
                    await bucketsStore.setS3Client(projectsStore.state.selectedProject.id);
                } catch (error) {
                    notify.notifyError(error, AnalyticsErrorEventSource.DOWNLOAD_PREFIX_DIALOG);
                    return;
                }
            }

            try {
                await downloadPrefix(props.bucket, props.prefix, downloadFormat.value);
                model.value = false;
                notify.success(
                    () => ['Keep this download link private.', h('br'), 'If you want to share, use the Share option.'],
                    'Download started',
                );
            } catch (error) {
                error.message = `Unable to download ${props.prefixType}. ${error.message}`;
                notify.notifyError(error, AnalyticsErrorEventSource.DOWNLOAD_PREFIX_DIALOG);
            }
        });
    });});
}

watch(model, async newVal => {
    if (newVal) {
        if (platform.value.linux || platform.value.mac) {
            downloadFormat.value = DownloadPrefixFormat.TAR_GZ;
        } else {
            downloadFormat.value = DownloadPrefixFormat.ZIP;
        }
    }
});
</script>

<style scoped lang="scss">
.download-dialog__content {
    transition: opacity 250ms cubic-bezier(0.4, 0, 0.2, 1);

    &--loading {
        opacity: 0.3;
        transition: opacity 0s;
        pointer-events: none;
    }
}

.border-colored {
    border-color: currentcolor !important;
}
</style>

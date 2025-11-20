// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card :loading="isLoading">
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="Share2" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Share {{ shareText }}
                </v-card-title>
                <template #append>
                    <v-btn
                        id="close-share"
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

            <v-window v-model="step">
                <v-window-item :value="ShareStep.OptionsStep">
                    <v-form v-model="formValid" class="pa-6" @submit.prevent>
                        <v-row>
                            <v-col cols="12">
                                <p class="text-subtitle-2 mb-6">Every shared {{ shareText.toLowerCase() }} requires an explicit Access Key.</p>
                                <v-text-field
                                    v-model="accessName"
                                    variant="outlined"
                                    :rules="accessNameRules"
                                    label="Access Key name"
                                    placeholder="Enter access key name"
                                    :hide-details="false"
                                    :maxlength="maxAccessNameLength"
                                    :disabled="isLoading"
                                    required
                                />
                                <template v-if="isPaidProjectOwner">
                                    <p class="text-subtitle-2 mb-6">Set an optional expiration date for this link.</p>

                                    <v-chip-group
                                        v-model="expirationOption"
                                        selected-class="font-weight-bold"
                                        mandatory
                                        column
                                        filter
                                    >
                                        <v-chip v-for="option in Object.values(ExpirationOptions)" :key="option" :value="option">{{ option }}</v-chip>
                                    </v-chip-group>

                                    <v-date-picker
                                        v-if="expirationOption === ExpirationOptions.Custom"
                                        v-model="expiration"
                                        width="100%"
                                        border
                                        elevation="0"
                                        rounded="lg"
                                        :min="minDate"
                                    />
                                </template>
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>
                <v-window-item :value="ShareStep.LinkStep">
                    <v-card-item class="pa-6">
                        <v-row>
                            <v-col cols="12">
                                <v-alert type="info" variant="tonal">
                                    Public link sharing. Anyone with the link can view your shared {{ shareText.toLowerCase() }}. No sign-in required.
                                </v-alert>
                            </v-col>
                            <v-col v-if="shareInfo?.freeTrialExpiration" cols="12" class="pt-0">
                                <v-alert type="warning" variant="tonal">
                                    This link will expire at {{ shareInfo.freeTrialExpiration.toLocaleString() }}.
                                    <a class="text-decoration-underline text-cursor-pointer" @click="appStore.toggleUpgradeFlow(true)">Upgrade</a> your account to avoid expiration limits on future links.
                                </v-alert>
                            </v-col>
                            <v-col cols="12" class="pt-0">
                                <v-tabs v-model="shareTab" color="primary" class="border-b-thin" center-active>
                                    <v-tab value="links"><component :is="Link" :size="16" class="mr-2" />Links</v-tab>
                                    <v-tab v-if="configStore.isDefaultBrand" value="social"><component :is="Share2" :size="16" class="mr-2" />Social</v-tab>
                                    <v-tab v-if="showEmbedCode" value="embed"><component :is="Code2" :size="16" class="mr-2" />Embed</v-tab>
                                </v-tabs>
                            </v-col>
                            <v-col cols="12" class="pt-0">
                                <v-window v-model="shareTab">
                                    <v-window-item value="links">
                                        <p class="text-subtitle-2 font-weight-bold mt-2 d-flex align-center">
                                            <component :is="Eye" :size="16" class="mr-2" /> Interactive Preview Link
                                            <v-chip
                                                v-if="rawLink"
                                                size="x-small"
                                                variant="outlined"
                                                color="default"
                                                class="ml-2"
                                            >
                                                Default
                                            </v-chip>
                                        </p>
                                        <p class="text-caption text-medium-emphasis mb-2">View the {{ shareText.toLowerCase() }} in a browser before downloading</p>
                                        <v-text-field
                                            :model-value="link"
                                            variant="solo-filled"
                                            rounded="lg"
                                            hide-details="auto"
                                            flat
                                            readonly
                                            class="text-caption"
                                        >
                                            <template #append-inner>
                                                <input-copy-button :value="link" />
                                            </template>
                                        </v-text-field>

                                        <template v-if="rawLink">
                                            <p class="text-subtitle-2 font-weight-bold mt-4 d-flex align-center">
                                                <component :is="Download" :size="16" class="mr-2" /> Direct Download Link
                                            </p>
                                            <p class="text-caption text-medium-emphasis mb-2">Download the file immediately without preview</p>
                                            <v-text-field
                                                :model-value="rawLink"
                                                variant="solo-filled"
                                                rounded="lg"
                                                hide-details="auto"
                                                readonly
                                                flat
                                                class="text-caption"
                                            >
                                                <template #append-inner>
                                                    <input-copy-button :value="rawLink" />
                                                </template>
                                            </v-text-field>
                                        </template>
                                    </v-window-item>

                                    <v-window-item value="social">
                                        <p class="text-subtitle-2 font-weight-bold mt-2">Share via</p>
                                        <p class="text-caption text-medium-emphasis mb-2">Share your link on social media or via email</p>
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
                                                    size="16"
                                                />
                                                {{ opt }}
                                            </v-chip>
                                        </v-chip-group>
                                    </v-window-item>

                                    <v-window-item v-if="showEmbedCode" value="embed">
                                        <p class="text-subtitle-2 font-weight-bold mt-2">HTML Embed Code</p>
                                        <p class="text-caption text-medium-emphasis mb-2">Add this code to your website to embed the file</p>
                                        <v-text-field
                                            :model-value="embedCode"
                                            variant="solo-filled"
                                            rounded="lg"
                                            hide-details="auto"
                                            readonly
                                            flat
                                            class="text-caption"
                                        >
                                            <template #append-inner>
                                                <input-copy-button :value="embedCode" />
                                            </template>
                                        </v-text-field>
                                    </v-window-item>
                                </v-window>
                            </v-col>
                        </v-row>
                    </v-card-item>
                </v-window-item>
            </v-window>

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
                            v-if="step === ShareStep.OptionsStep"
                            color="primary"
                            variant="flat"
                            :disabled="!formValid || (expirationOption === ExpirationOptions.Custom && !expiration)"
                            :loading="isLoading"
                            block
                            @click="generateShareLink"
                        >
                            Next ->
                        </v-btn>
                        <v-btn
                            v-else
                            :color="justCopied ? 'success' : 'primary'"
                            variant="flat"
                            :prepend-icon="justCopied ? Check : Copy"
                            :disabled="isLoading"
                            block
                            @click="onCopy"
                        >
                            {{ justCopied ? 'Copied' : 'Copy ' + copyButtonText }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
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
    VDatePicker,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VSheet,
    VTab,
    VTabs,
    VTextField,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { Check, Code2, Copy, Download, Eye, Link, Share2, X } from 'lucide-vue-next';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/composables/useNotify';
import { ShareInfo, ShareType, useLinksharing } from '@/composables/useLinksharing';
import { EXTENSION_PREVIEW_TYPES, PreviewType, SHARE_BUTTON_CONFIGS, ShareOptions } from '@/types/browser';
import { useAppStore } from '@/store/modules/appStore';
import { BrowserObject } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { RequiredRule, ValidationRule } from '@/types/common';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';

import InputCopyButton from '@/components/InputCopyButton.vue';

// Define tab values as string literals for better type support
type ShareTabType = 'links' | 'social' | 'embed';

enum ShareStep {
    OptionsStep = 'options',
    LinkStep = 'link',
}

enum ExpirationOptions {
    NoExpiration = 'No Expiration',
    Custom = 'Custom',
}

const props = defineProps<{
    bucketName: string,
    file?: BrowserObject,
}>();

const emit = defineEmits<{
    'contentRemoved': [];
}>();

const accessNameRules: ValidationRule<string>[] = [
    RequiredRule,
    v => !agStore.state.allAGNames.includes(v) || 'This name is already in use',
];

const model = defineModel<boolean>({ required: true });

const appStore = useAppStore();
const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const projectStore = useProjectsStore();
const configStore = useConfigStore();
const agStore = useAccessGrantsStore();

const notify = useNotify();
const { generateBucketShareURL, generateFileOrFolderShareURL } = useLinksharing();
const { isLoading, withLoading } = useLoading();

const step = ref<ShareStep>(ShareStep.OptionsStep);
const formValid = ref<boolean>(false);
const accessName = ref<string>('');
const expirationOption = ref<ExpirationOptions>(ExpirationOptions.NoExpiration);
const expiration = ref<Date | null>(null);
const shareTab = ref<ShareTabType>('links');
const shareInfo = ref<ShareInfo>();

const type = computed<ShareType>(() => !props.file ? ShareType.Bucket : (props.file.type === 'folder' ? ShareType.Folder : ShareType.Object));

const link = computed<string>(() => shareInfo.value?.url || '');
const rawLink = computed<string>(() => props.file?.type === 'file' ? link.value.replace('/s/', '/raw/') : '');

const copiedTimeout = ref<ReturnType<typeof setTimeout> | null>(null);
const justCopied = computed<boolean>(() => copiedTimeout.value !== null);

const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

const maxAccessNameLength = computed<number>(() => configStore.state.config.maxNameCharacters);
const selectedProjectID = computed<string>(() => projectStore.state.selectedProject.id);

const isPaidProjectOwner = computed<boolean>(() => projectStore.state.selectedProjectConfig.hasPaidPrivileges);

const shareText = computed<string>(() => {
    switch (type.value) {
    case ShareType.Bucket:
        return 'Bucket';
    case ShareType.Folder:
        return 'Folder';
    case ShareType.Object:
        return 'File';
    default:
        return '';
    }
});

const minDate = computed<string>(() => {
    const today = new Date();
    today.setDate(today.getDate()+1);
    return today.toISOString().split('T')[0];
});

const fileType = computed<PreviewType>(() => {
    if (!props.file) return PreviewType.None;

    const dotIdx = props.file.Key.lastIndexOf('.');
    if (dotIdx === -1) return PreviewType.None;

    const ext = props.file.Key.toLowerCase().slice(dotIdx + 1);
    for (const [exts, previewType] of EXTENSION_PREVIEW_TYPES) {
        if (exts.includes(ext)) return previewType;
    }

    return PreviewType.None;
});

const showEmbedCode = computed<boolean>(() => {
    return fileType.value === PreviewType.Video ||
        fileType.value === PreviewType.Audio ||
        fileType.value === PreviewType.Image;
});

const embedCode = computed<string>(() => {
    switch (fileType.value) {
    case PreviewType.Video:
        return `<video src="${rawLink.value}" controls />`;
    case PreviewType.Audio:
        return `<audio src="${rawLink.value}" controls />`;
    case PreviewType.Image:
        return `<img src="${rawLink.value}" alt="Shared image" />`;
    default:
        return '';
    }
});

const copyButtonText = computed<string>(() => {
    switch (shareTab.value) {
    case 'embed':
        return 'Code';
    case 'social':
        return 'Link';
    case 'links':
    default:
        return 'Link';
    }
});

/**
 * Saves the link or embed code to clipboard based on active tab.
 */
function onCopy(): void {
    let contentToCopy = link.value;

    if (shareTab.value === 'embed' && showEmbedCode.value) {
        contentToCopy = embedCode.value;
    }

    navigator.clipboard.writeText(contentToCopy);
    analyticsStore.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);

    if (copiedTimeout.value) clearTimeout(copiedTimeout.value);
    copiedTimeout.value = setTimeout(() => {
        copiedTimeout.value = null;
    }, 750);
}

function generateShareLink(): void {
    withLoading(async () => {
        if (!formValid.value || (expirationOption.value === ExpirationOptions.Custom && !expiration.value)) return;

        try {
            if (!props.file) {
                shareInfo.value = await generateBucketShareURL(
                    props.bucketName,
                    accessName.value,
                    expiration.value,
                );
            } else {
                shareInfo.value = await generateFileOrFolderShareURL(
                    props.bucketName,
                    filePath.value,
                    props.file.Key,
                    // TODO: replace magic string type of BrowserObject.type with some constant/enum.
                    props.file.type === 'folder' ? ShareType.Folder : ShareType.Object,
                    accessName.value,
                    expiration.value,
                );
            }

            step.value = ShareStep.LinkStep;

            analyticsStore.eventTriggered(AnalyticsEvent.LINK_SHARED, { project_id: selectedProjectID.value });
        } catch (error) {
            error.message = `Unable to get sharing URL. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.SHARE_MODAL);
            model.value = false;
        }
    });
}

watch(expirationOption, val => {
    if (val === ExpirationOptions.NoExpiration) expiration.value = null;
});

watch(model, val => {
    if (!val) {
        step.value = ShareStep.OptionsStep;
        shareTab.value = 'links';
        shareInfo.value = undefined;
        emit('contentRemoved');

        return;
    }

    let access = props.bucketName;
    if (props.file) access = props.file.Key;

    access = `${access}_shared-${type.value}_${new Date().toISOString()}`;
    if (access.length > maxAccessNameLength.value) {
        access = `shared-${type.value}_${new Date().toISOString()}`;
    }

    accessName.value = access;
    expirationOption.value = ExpirationOptions.NoExpiration;
    expiration.value = null;

    withLoading(async () => {
        try {
            await agStore.getAllAGNames(selectedProjectID.value);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.SHARE_MODAL);
        }
    });
});
</script>

<style scoped lang="scss">
:deep(.v-field__input) {
    font-size: 0.875rem !important;
}
</style>

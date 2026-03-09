// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card-item class="pa-6">
        <template #prepend>
            <v-sheet
                class="border-sm d-flex justify-center align-center"
                width="40"
                height="40"
                rounded="lg"
            >
                <component :is="Box" :size="18" />
            </v-sheet>
        </template>

        <v-card-title class="font-weight-bold">Create New Project</v-card-title>

        <template #append>
            <v-btn
                :icon="X"
                variant="text"
                size="small"
                color="default"
                :disabled="isLoading"
                @click="emit('cancel')"
            />
        </template>
    </v-card-item>

    <v-divider />

    <v-window v-model="step" :touch="false">
        <v-window-item :value="Step.Info">
            <v-form v-model="formValid" class="pa-6" @submit.prevent>
                <v-row>
                    <v-col cols="12">
                        Projects are where you and your team can upload and manage data, and view usage statistics and billing.
                    </v-col>
                    <v-col cols="12">
                        <v-text-field
                            id="Project Name"
                            v-model="name"
                            variant="outlined"
                            :rules="nameRules"
                            label="Name"
                            placeholder="Enter a name for your project"
                            :counter="MAX_NAME_LENGTH"
                            :maxlength="MAX_NAME_LENGTH"
                            persistent-counter
                            :hide-details="false"
                            autofocus
                            required
                        />
                    </v-col>
                    <v-col cols="12">
                        <v-text-field
                            v-model="description"
                            variant="outlined"
                            :rules="descriptionRules"
                            :hide-details="false"
                            label="Description (optional)"
                            placeholder="Describe the project's purpose"
                            :counter="MAX_DESCRIPTION_LENGTH"
                            :maxlength="MAX_DESCRIPTION_LENGTH"
                            persistent-counter
                            hint="This will appear on project cards."
                        />
                    </v-col>
                </v-row>
            </v-form>
        </v-window-item>

        <v-window-item :value="Step.ManageMode">
            <v-form v-model="formValid" class="pa-6">
                <v-row>
                    <v-col>
                        <p><b>Project Encryption</b></p>
                        <p class="my-2">Choose the encryption method for your data.</p>
                        <v-chip-group v-model="passphraseManageMode" column filter variant="outlined" selected-class="font-weight-bold" mandatory>
                            <v-chip value="auto">
                                Automatic
                            </v-chip>
                            <v-chip value="manual">Self-managed</v-chip>

                            <v-divider thickness="0" class="my-1" />

                            <v-alert v-if="passphraseManageMode === 'auto'" variant="tonal" color="default">
                                <p>
                                    <v-chip rounded="md" class="text-caption font-weight-medium" color="secondary" variant="tonal" size="small">
                                        Recommended for ease of use and teams
                                    </v-chip>
                                </p>
                                <p class="text-body-2 my-2 font-weight-bold">
                                    {{ configStore.brandName }} securely manages the encryption and decryption of your project automatically.
                                </p>
                                <p class="text-body-2 my-2">
                                    Fewer steps to upload, download, manage, and browse your data. No need to remember an additional encryption passphrase.
                                </p>
                                <p class="text-body-2 my-2">
                                    Team members you invite will automatically have access to your project's data.
                                </p>
                                <p class="text-body-2 my-2 font-weight-bold">
                                    Recommended for full S3-compatibility.
                                </p>
                                <p v-if="configStore.isDefaultBrand" class="text-body-2 mt-2">
                                    <a class="link" href="https://storj.dev/dcs/api/s3/s3-compatibility/storj-vs-self-managed-encryption-s3-compatibility-differences" target="_blank" rel="noopener noreferrer">S3 compatibility differences</a>
                                    •
                                    <a class="link" @click="goToDocs">Encryption options documentation</a>
                                </p>
                            </v-alert>

                            <v-alert v-if="passphraseManageMode === 'manual'" variant="tonal" color="default">
                                <p>
                                    <v-chip rounded="md" class="text-caption font-weight-medium" color="secondary" variant="tonal" size="small">
                                        Best for control over your data encryption
                                    </v-chip>
                                </p>
                                <p class="text-body-2 my-2 font-weight-bold">
                                    You are responsible for securely managing your own data encryption passphrase.
                                </p>
                                <p class="text-body-2 my-2">
                                    You need to enter your passphrase each time you access your data. If you forget the passphrase, you can't recover your data.
                                </p>
                                <p class="text-body-2 my-2">
                                    Team members must share and enter the same encryption passphrase to access the data.
                                </p>
                                <p class="text-body-2 my-2 font-weight-bold">
                                    Increased control, limited S3-compatibility.
                                </p>
                                <p v-if="configStore.isDefaultBrand" class="text-body-2 mt-2">
                                    <a class="link" href="https://storj.dev/dcs/api/s3/s3-compatibility/storj-vs-self-managed-encryption-s3-compatibility-differences" target="_blank" rel="noopener noreferrer">S3 compatibility differences</a>
                                    •
                                    <a class="link" @click="goToDocs">Encryption options documentation</a>
                                </p>
                            </v-alert>
                        </v-chip-group>
                    </v-col>
                </v-row>
            </v-form>
        </v-window-item>
    </v-window>

    <v-divider />

    <v-card-actions class="pa-6">
        <v-row>
            <v-col>
                <v-btn
                    variant="outlined"
                    color="default"
                    block
                    :disabled="isLoading"
                    @click="onBackOrCancel"
                >
                    {{ step === Step.ManageMode ? 'Back' : 'Cancel' }}
                </v-btn>
            </v-col>
            <v-col v-if="showEncryptionStep && step === Step.Info">
                <v-btn
                    color="primary"
                    variant="flat"
                    block
                    :append-icon="ArrowRight"
                    :disabled="!formValid"
                    @click="step = Step.ManageMode"
                >
                    Next
                </v-btn>
            </v-col>
            <v-col v-else>
                <v-btn
                    color="primary"
                    variant="flat"
                    :loading="isLoading"
                    block
                    @click="onCreate"
                >
                    Create Project
                </v-btn>
            </v-col>
        </v-row>
    </v-card-actions>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VAlert,
    VBtn,
    VCardActions,
    VCardItem,
    VCardTitle,
    VChip,
    VChipGroup,
    VCol,
    VDivider,
    VForm,
    VRow,
    VSheet,
    VTextField,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { ArrowRight, Box, X } from 'lucide-vue-next';

import { RequiredRule, ValidationRule } from '@/types/common';
import { ManagePassphraseMode, MAX_DESCRIPTION_LENGTH, MAX_NAME_LENGTH, Project, ProjectFields } from '@/types/projects';
import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/composables/useNotify';
import {
    AnalyticsErrorEventSource,
    AnalyticsEvent,
    PageVisitSource,
    SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE,
} from '@/utils/constants/analyticsEventNames';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

enum Step {
    Info,
    ManageMode,
}

const emit = defineEmits<{
    cancel: [];
    created: [project: Project];
    'update:loading': [value: boolean];
}>();

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const formValid = ref(false);
const name = ref('');
const description = ref('');
const passphraseManageMode = ref<ManagePassphraseMode>(
    configStore.state.config.satelliteManagedEncryptionEnabled ? 'auto' : 'manual',
);
const step = ref(Step.Info);

const satelliteManagedEncryptionEnabled = computed(() => configStore.state.config.satelliteManagedEncryptionEnabled);
const hideProjectEncryptionOptions = computed(() => configStore.state.config.hideProjectEncryptionOptions);
const showEncryptionStep = computed(() => satelliteManagedEncryptionEnabled.value && !hideProjectEncryptionOptions.value);

const nameRules: ValidationRule<string>[] = [
    RequiredRule,
    v => v.length <= MAX_NAME_LENGTH || 'Name is too long',
];

const descriptionRules: ValidationRule<string>[] = [
    v => v.length <= MAX_DESCRIPTION_LENGTH || 'Description is too long',
];

async function onCreate(): Promise<void> {
    if (!formValid.value) return;

    await withLoading(async () => {
        let project: Project;
        try {
            const fields = new ProjectFields(name.value, description.value, usersStore.state.user.id, passphraseManageMode.value === 'auto');
            project = await projectsStore.createProject(fields);
        } catch (error) {
            error.message = `Failed to create project. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_PROJECT_MODAL);
            return;
        }
        emit('created', project);
    });
}

function onBackOrCancel(): void {
    if (step.value === Step.ManageMode) {
        step.value = Step.Info;
    } else {
        emit('cancel');
    }
}

function goToDocs(): void {
    analyticsStore.pageVisit(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, '_blank', 'noreferrer');
}

function reset(): void {
    name.value = '';
    description.value = '';
    step.value = Step.Info;
    passphraseManageMode.value = satelliteManagedEncryptionEnabled.value ? 'auto' : 'manual';
}

defineExpose({ reset });

watch(isLoading, v => emit('update:loading', v));
</script>

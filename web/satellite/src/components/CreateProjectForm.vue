// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-form v-model="formValid" @submit.prevent>
        <v-row>
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

        <v-alert v-if="showEncryptionDropdown" color="default" variant="tonal" width="auto" class="mt-4">
            <h2 class="text-subtitle-2 d-flex align-center">
                Project Encryption
                <v-tooltip location="top" max-width="340">
                    <template #activator="{ props: tooltipProps }">
                        <v-icon v-bind="tooltipProps" :icon="Info" size="14" class="ml-1 text-medium-emphasis" />
                    </template>
                    All project data is encrypted.
                    <br><b>Automatic</b> mode lets Storj manage the passphrase for you — ideal for S3 compatibility.
                    <br><b>Self-managed</b> gives you full control of your own passphrase.
                    <a class="d-block mt-1 link" @click.stop="goToDocs">Learn more</a>
                </v-tooltip>
            </h2>
            <p>Choose the encryption method:</p>
            <v-select
                id="Select Passphrase Management Mode"
                v-model="passphraseManageMode"
                class="mt-4"
                :items="passphraseManageModeOptions"
                item-value="value"
                variant="outlined"
                density="comfortable"
                hide-details
            />
        </v-alert>
    </v-form>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VAlert,
    VCol,
    VForm,
    VIcon,
    VRow,
    VSelect,
    VTextField,
    VTooltip,
} from 'vuetify/components';
import { Info } from 'lucide-vue-next';

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

const emit = defineEmits<{
    created: [project: Project];
    'update:loading': [value: boolean];
}>();

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const satelliteManagedEncryptionEnabled = computed(() => configStore.state.config.satelliteManagedEncryptionEnabled);

const showEncryptionDropdown = computed(() =>
    configStore.isDefaultBrand && satelliteManagedEncryptionEnabled.value && !configStore.state.config.hideProjectEncryptionOptions,
);

const passphraseManageModeOptions = [
    { value: 'auto', title: 'Automatic (Default)' },
    { value: 'manual', title: 'Self-Managed' },
];

const nameRules: ValidationRule<string>[] = [
    RequiredRule,
    v => v.length <= MAX_NAME_LENGTH || 'Name is too long',
];

const descriptionRules: ValidationRule<string>[] = [
    v => v.length <= MAX_DESCRIPTION_LENGTH || 'Description is too long',
];

const formValid = ref(false);
const name = ref('');
const description = ref('');
const passphraseManageMode = ref<ManagePassphraseMode>(satelliteManagedEncryptionEnabled.value ? 'auto' : 'manual');

async function submit(): Promise<void> {
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

function goToDocs(): void {
    analyticsStore.pageVisit(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, '_blank', 'noreferrer');
}

function reset(): void {
    name.value = '';
    description.value = '';
    passphraseManageMode.value = satelliteManagedEncryptionEnabled.value ? 'auto' : 'manual';
}

defineExpose({ submit, reset, formValid });

watch(isLoading, v => emit('update:loading', v));
</script>

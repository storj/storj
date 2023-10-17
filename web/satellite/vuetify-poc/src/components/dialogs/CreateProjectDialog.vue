// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        :model-value="model && !isUpgradeDialogShown"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading"
        :scrim="false"
        @update:model-value="v => model = v"
    >
        <v-card rounded="xlg">
            <v-card-item class="pl-7 py-4">
                <template #prepend>
                    <img class="d-block" src="@/../static/images/common/blueBox.svg" alt="Box">
                </template>

                <v-card-title class="font-weight-bold">
                    {{ !isProjectLimitReached ? 'Create New Project' : 'Get More Projects' }}
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

            <v-form v-model="formValid" class="pa-7" @submit.prevent>
                <v-row>
                    <template v-if="!isProjectLimitReached">
                        <v-col cols="12">
                            Projects are where you and your team can upload and manage data, and view usage statistics and billing.
                        </v-col>
                        <v-col cols="12">
                            <v-text-field
                                v-model="name"
                                variant="outlined"
                                :rules="nameRules"
                                label="Project Name"
                                :counter="MAX_NAME_LENGTH"
                                :maxlength="MAX_NAME_LENGTH"
                                persistent-counter
                                :hide-details="false"
                                autofocus
                            />
                            <v-btn
                                v-if="!isDescriptionShown"
                                variant="text"
                                size="small"
                                color="default"
                                prepend-icon="mdi-plus"
                                @click="isDescriptionShown = true"
                            >
                                Add Description (Optional)
                            </v-btn>
                        </v-col>
                        <v-col v-if="isDescriptionShown" cols="12">
                            <v-text-field
                                v-model="description"
                                variant="outlined"
                                :rules="descriptionRules"
                                :hide-details="false"
                                label="Project Description (Optional)"
                                :counter="MAX_DESCRIPTION_LENGTH"
                                :maxlength="MAX_DESCRIPTION_LENGTH"
                                persistent-counter
                            />
                        </v-col>
                    </template>
                    <v-col v-else>
                        Upgrade to Pro Account to create more projects and gain access to higher limits.
                    </v-col>
                </v-row>
            </v-form>

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
                            color="primary"
                            variant="flat"
                            :loading="isLoading"
                            block
                            :append-icon="isProjectLimitReached ? 'mdi-arrow-right' : undefined"
                            @click="onPrimaryClick"
                        >
                            {{ !isProjectLimitReached ? 'Create Project' : 'Upgrade' }}
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <upgrade-account-dialog
        :scrim="false"
        :model-value="model && isUpgradeDialogShown"
        @update:model-value="v => model = isUpgradeDialogShown = v"
    />

    <teleport to="body">
        <v-fade-transition>
            <div v-show="model" class="v-overlay__scrim custom-scrim" />
        </v-fade-transition>
    </teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, Teleport } from 'vue';
import { useRouter } from 'vue-router';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VCardActions,
    VBtn,
    VDivider,
    VForm,
    VRow,
    VCol,
    VTextField,
    VFadeTransition,
} from 'vuetify/components';

import { RequiredRule, ValidationRule } from '@poc/types/common';
import { MAX_DESCRIPTION_LENGTH, MAX_NAME_LENGTH, Project, ProjectFields } from '@/types/projects';
import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import UpgradeAccountDialog from '@poc/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';

const props = defineProps<{
    modelValue: boolean,
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean],
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const router = useRouter();

const formValid = ref<boolean>(false);
const name = ref<string>('');
const description = ref<string>('');
const isDescriptionShown = ref<boolean>(false);
const isProjectLimitReached = ref<boolean>(false);
const isUpgradeDialogShown = ref<boolean>(false);

const nameRules: ValidationRule<string>[] = [
    RequiredRule,
    v => v.length <= MAX_NAME_LENGTH || 'Name is too long',
];

const descriptionRules: ValidationRule<string>[] = [
    v => v.length <= MAX_DESCRIPTION_LENGTH || 'Description is too long',
];

/**
 * Handles primary button click.
 */
async function onPrimaryClick(): Promise<void> {
    if (isProjectLimitReached.value) {
        isUpgradeDialogShown.value = true;
        return;
    }

    if (!formValid.value) return;
    await withLoading(async () => {
        let project: Project;
        try {
            const fields = new ProjectFields(name.value, description.value, usersStore.state.user.id);
            project = await projectsStore.createProject(fields);
        } catch (error) {
            error.message = `Failed to create project. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.CREATE_PROJECT_MODAL);
            return;
        }
        model.value = false;
        router.push(`/projects/${project.urlId}/dashboard`);
        notify.success('Project created.');
    });
}

watch(() => model.value, shown => {
    if (!shown) return;
    isProjectLimitReached.value = projectsStore.state.projects.length >= usersStore.state.user.projectLimit;
    isDescriptionShown.value = false;
    name.value = '';
    description.value = '';
});
</script>

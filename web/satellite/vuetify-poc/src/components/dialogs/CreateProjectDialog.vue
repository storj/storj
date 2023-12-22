// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-overlay v-model="model" persistent />

    <v-dialog
        :model-value="model && !isUpgradeDialogShown"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading"
        :scrim="false"
        @update:model-value="v => model = v"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <img v-if="isProjectLimitReached && usersStore.state.user.paidTier && showLimitIncreaseDialog" class="d-block" src="@/../static/images/modals/limit.svg" alt="Speedometer">
                    <img v-else class="d-block" src="@/../static/images/common/blueBox.svg" alt="Box">
                </template>

                <v-card-title class="font-weight-bold">
                    {{ cardTitle }}
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
                    <template v-if="!billingEnabled || !isProjectLimitReached">
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
                                :prepend-icon="mdiPlus"
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
                    <template v-else-if="isProjectLimitReached && usersStore.state.user.paidTier && !showLimitIncreaseDialog">
                        <v-col cols="12">
                            Request project limit increase.
                        </v-col>
                    </template>
                    <template v-else-if="isProjectLimitReached && usersStore.state.user.paidTier && showLimitIncreaseDialog">
                        <v-col cols="12">
                            Request a projects limit increase for your account.
                        </v-col>
                        <v-col cols="6">
                            <p>Projects Limit</p>
                            <v-text-field
                                class="edit-project-limit__text-field"
                                variant="solo-filled"
                                density="compact"
                                flat
                                readonly
                                :model-value="usersStore.state.user.projectLimit"
                            />
                        </v-col>
                        <v-col cols="6">
                            <p>Requested Limit</p>
                            <v-text-field
                                class="edit-project-limit__text-field"
                                density="compact"
                                flat
                                type="number"
                                :rules="projectLimitRules"
                                :model-value="inputText"
                                maxlength="4"
                                @update:model-value="updateInputText"
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
                            :append-icon="isProjectLimitReached && billingEnabled ? mdiArrowRight : undefined"
                            @click="onPrimaryClick"
                        >
                            {{ buttonTitle }}
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
</template>

<script setup lang="ts">
import { Component, ref, computed, watch } from 'vue';
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
    VOverlay,
} from 'vuetify/components';
import { mdiArrowRight, mdiPlus } from '@mdi/js';

import { RequiredRule, ValidationRule } from '@poc/types/common';
import { MAX_DESCRIPTION_LENGTH, MAX_NAME_LENGTH, Project, ProjectFields } from '@/types/projects';
import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@poc/store/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

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

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();
const appStore = useAppStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const router = useRouter();

const innerContent = ref<Component | null>(null);
const formValid = ref<boolean>(false);
const inputText = ref<string>('');
const name = ref<string>('');
const description = ref<string>('');
const isDescriptionShown = ref<boolean>(false);
const isProjectLimitReached = ref<boolean>(false);
const isUpgradeDialogShown = ref<boolean>(false);
const showLimitIncreaseDialog = ref<boolean>(false);

const nameRules: ValidationRule<string>[] = [
    RequiredRule,
    v => v.length <= MAX_NAME_LENGTH || 'Name is too long',
];

const descriptionRules: ValidationRule<string>[] = [
    v => v.length <= MAX_DESCRIPTION_LENGTH || 'Description is too long',
];

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.state.config.billingFeaturesEnabled);

/**
 * Indicates if limit increase requests can be sent directly from the UI.
 */
const isLimitIncreaseRequestEnabled = computed<boolean>(() => configStore.state.config.limitIncreaseRequestEnabled);

/**
 * Handles primary button click.
 */
async function onPrimaryClick(): Promise<void> {
    if (!isProjectLimitReached.value || !billingEnabled.value) {
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

            analyticsStore.pageVisit('/projects/dashboard');
        });
    } else if (usersStore.state.user.paidTier) {
        if (!isLimitIncreaseRequestEnabled.value) {
            model.value = false;
            window.open('https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212', '_blank', 'noopener');
            return;
        } else if (showLimitIncreaseDialog.value) {
            if (!formValid.value) return;
            await withLoading(async () => {
                try {
                    await usersStore.requestProjectLimitIncrease(inputText.value);
                } catch (error) {
                    error.message = `Failed to request project limit increase. ${error.message}`;
                    notify.notifyError(error, AnalyticsErrorEventSource.CREATE_PROJECT_MODAL);
                    return;
                }
                model.value = false;
                notify.success('Project limit increase requested');
                return;
            });
        } else {
            showLimitIncreaseDialog.value = true;
        }
    } else {
        isUpgradeDialogShown.value = true;
    }
}

/*
 * Returns an array of validation rules applied to the text input.
 */
const projectLimitRules = computed<ValidationRule<string>[]>(() => {
    return [
        RequiredRule,
        v => !(isNaN(+v) || !Number.isInteger((parseFloat(v)))) || 'Invalid number',
        v => (parseFloat(v) > 0) || 'Number must be positive',
    ];
});

/**
 * Updates input refs with value from text field.
 */
function updateInputText(value: string): void {
    inputText.value = value;
}

const buttonTitle = computed((): string => {
    if (!isProjectLimitReached.value || !billingEnabled.value) {
        return 'Create Project';
    }
    if (usersStore.state.user.paidTier) {
        if (showLimitIncreaseDialog.value) {
            return 'Submit';
        }
        return 'Request';
    }
    return 'Upgrade';
});

const cardTitle = computed((): string => {
    if (!isProjectLimitReached.value || !billingEnabled.value) {
        return 'Create New Project';
    }
    if (usersStore.state.user.paidTier && showLimitIncreaseDialog.value) {
        return 'Projects Limit Request';
    }
    return 'Get More Projects';
});

watch(innerContent, comp => {
    if (comp) {
        isProjectLimitReached.value = projectsStore.state.projects.length >= usersStore.state.user.projectLimit;
        isDescriptionShown.value = false;
        name.value = '';
        description.value = '';
        inputText.value = String(usersStore.state.user.projectLimit + 1);
    } else {
        showLimitIncreaseDialog.value = false;
    }
});
</script>

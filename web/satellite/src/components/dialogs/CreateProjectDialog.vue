// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-overlay v-model="model" persistent />

    <v-dialog
        :model-value="model && !isUpgradeDialogShown"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading || satelliteManagedEncryptionEnabled"
        :scrim="false"
        scrollable
        @update:model-value="v => model = v"
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
                        <component :is="Gauge" v-if="isProjectLimitReached && usersStore.state.user.isPaid && showLimitIncreaseDialog" :size="18" />
                        <component :is="Box" v-else :size="18" />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">
                    {{ cardTitle }}
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

            <v-window v-if="!(billingEnabled && (isMemberAccount || isProjectLimitReached))" v-model="createStep" :touch="false">
                <v-window-item :value="CreateSteps.Info">
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
                <v-window-item :value="CreateSteps.ManageMode">
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
                                        <p class="text-body-2 mt-2">
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
                                        <p class="text-body-2 mt-2">
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

            <v-form v-else-if="isProjectLimitReached && usersStore.state.user.hasPaidPrivileges" v-model="formValid" class="pa-6" @submit.prevent>
                <v-row>
                    <template v-if="!showLimitIncreaseDialog">
                        <v-col cols="12">
                            You've reached your project limit. Request an increase to create more projects.
                        </v-col>
                    </template>
                    <template v-else>
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
                </v-row>
            </v-form>

            <v-row v-else-if="isMemberAccount && billingEnabled" class="pa-6">
                <v-col>
                    Your account is currently a Member account with access to shared projects.
                    To create your own project, you'll need to start a free trial or upgrade to a Pro account.
                </v-col>
            </v-row>

            <v-row v-else class="pa-6">
                <v-col>
                    Upgrade to Pro Account to create more projects and gain access to higher limits.
                </v-col>
            </v-row>

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
                            {{ createStep === CreateSteps.ManageMode ? 'Back' : 'Cancel' }}
                        </v-btn>
                    </v-col>
                    <v-col v-if="!(billingEnabled && (isMemberAccount || isProjectLimitReached)) && satelliteManagedEncryptionEnabled && createStep === CreateSteps.Info">
                        <v-btn
                            color="primary"
                            variant="flat"
                            :loading="isLoading"
                            block
                            :append-icon="ArrowRight"
                            :disabled="!formValid"
                            @click="createStep = CreateSteps.ManageMode"
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
                            :append-icon="buttonTitle !== 'Create Project' ? ArrowRight : ''"
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
        :is-member-upgrade="isMemberAccount && billingEnabled"
        :model-value="model && isUpgradeDialogShown"
        @update:model-value="v => model = isUpgradeDialogShown = v"
    />
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VAlert,
    VBtn,
    VCard,
    VCardItem,
    VCardTitle,
    VCardActions,
    VChip,
    VChipGroup,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VCol,
    VTextField,
    VOverlay,
    VWindow,
    VWindowItem,
    VSheet,
} from 'vuetify/components';
import { ArrowRight, Box, Gauge, X } from 'lucide-vue-next';

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
import { ROUTES } from '@/router';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import UpgradeAccountDialog from '@/components/dialogs/upgradeAccountFlow/UpgradeAccountDialog.vue';

enum CreateSteps {
    Info,
    ManageMode,
}

const model = defineModel<boolean>({ required: true });

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const router = useRouter();

const formValid = ref<boolean>(false);
const inputText = ref<string>('');
const name = ref<string>('');
const description = ref<string>('');
const isDescriptionShown = ref<boolean>(false);
const isProjectLimitReached = ref<boolean>(false);
const isUpgradeDialogShown = ref<boolean>(false);
const showLimitIncreaseDialog = ref<boolean>(false);

const passphraseManageMode = ref<ManagePassphraseMode>('auto');
const createStep = ref<CreateSteps>(CreateSteps.Info);

const nameRules: ValidationRule<string>[] = [
    RequiredRule,
    v => v.length <= MAX_NAME_LENGTH || 'Name is too long',
];

const descriptionRules: ValidationRule<string>[] = [
    v => v.length <= MAX_DESCRIPTION_LENGTH || 'Description is too long',
];

const isMemberAccount = computed<boolean>(() => usersStore.state.user.isMember);

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user));

/**
 * Indicates if satellite managed encryption passphrase is enabled.
 */
const satelliteManagedEncryptionEnabled = computed<boolean>(() => configStore.state.config.satelliteManagedEncryptionEnabled);

/**
 * Indicates if limit increase requests can be sent directly from the UI.
 */
const isLimitIncreaseRequestEnabled = computed<boolean>(() => configStore.state.config.limitIncreaseRequestEnabled);

/**
 * Handles primary button click.
 */
async function onPrimaryClick(): Promise<void> {
    if (isMemberAccount.value && billingEnabled.value) {
        isUpgradeDialogShown.value = true;
        return;
    }

    if (!(isProjectLimitReached.value && billingEnabled.value)) {
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
            model.value = false;
            router.push({
                name: ROUTES.Dashboard.name,
                params: { id: project.urlId },
            });
            notify.success('Project created.');
        });
    } else if (usersStore.state.user.isPaid) {
        if (!isLimitIncreaseRequestEnabled.value) {
            model.value = false;
            window.open(`${configStore.supportUrl}?ticket_form_id=360000683212`, '_blank', 'noopener');
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

/**
 * Handles back or cancel button click.
 */
function onBackOrCancel(): void {
    if (createStep.value === CreateSteps.ManageMode) {
        createStep.value = CreateSteps.Info;
    } else {
        model.value = false;
    }
}

function goToDocs() {
    analyticsStore.pageVisit(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open(SATELLITE_MANAGED_ENCRYPTION_DOCS_PAGE, '_blank', 'noreferrer');
}

/**
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
    if (isMemberAccount.value && billingEnabled.value) {
        return 'Update Account';
    }
    if (!(isProjectLimitReached.value && billingEnabled.value)) {
        return 'Create Project';
    }
    if (usersStore.state.user.isPaid) {
        if (showLimitIncreaseDialog.value) {
            return 'Submit';
        }
        return 'Request';
    }
    return 'Upgrade';
});

const cardTitle = computed((): string => {
    if (isMemberAccount.value && billingEnabled.value) {
        return 'Create Your Own Project';
    }
    if (!(isProjectLimitReached.value && billingEnabled.value)) {
        return 'Create New Project';
    }
    if (usersStore.state.user.isPaid && showLimitIncreaseDialog.value) {
        return 'Projects Limit Request';
    }
    return 'Get More Projects';
});

watch(model, val => {
    if (val) {
        const ownedProjects = projectsStore.projects.filter((p) => p.ownerId === usersStore.state.user.id);
        isProjectLimitReached.value = ownedProjects.length >= usersStore.state.user.projectLimit;
        isDescriptionShown.value = false;
        name.value = '';
        description.value = '';
        inputText.value = String(usersStore.state.user.projectLimit + 1);

        if (!satelliteManagedEncryptionEnabled.value) {
            passphraseManageMode.value = 'manual';
        }
    } else {
        createStep.value = CreateSteps.Info;
        passphraseManageMode.value = 'auto';
        showLimitIncreaseDialog.value = false;
    }
});
</script>

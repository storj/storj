// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="pb-14">
        <v-row>
            <v-col>
                <trial-expiration-banner v-if="isTrialExpirationBanner && isUserProjectOwner" :expired="isExpired" />

                <PageTitleComponent title="Project Settings" />
                <PageSubtitleComponent subtitle="Edit project information and set custom project limits." link="https://docs.storj.io/learn/concepts/limits" />
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" sm="6" lg="4">
                <v-card title="Project Name" class="pa-2">
                    <v-card-text>
                        <v-chip color="primary" variant="tonal" size="small" class="font-weight-bold">
                            {{ project.name }}
                        </v-chip>
                        <v-divider class="my-4 border-0" />
                        <v-btn variant="outlined" color="default" :prepend-icon="Pencil" @click="showEditNameDialog">
                            Edit Name
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
            <v-col cols="12" sm="6" lg="4">
                <v-card title="Project Description" class="pa-2">
                    <v-card-text>
                        <v-chip color="primary" variant="tonal" size="small" class="font-weight-bold">
                            {{ project.description }}
                        </v-chip>
                        <v-divider class="my-4 border-0" />
                        <v-btn variant="outlined" color="default" :prepend-icon="Pencil" @click="showEditDescriptionDialog">
                            Edit Description
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
            <v-col v-if="satelliteManagedEncryptionEnabled || hasManagedPassphrase" cols="12" lg="4">
                <v-card title="Project Encryption" class="pa-2">
                    <v-card-text>
                        <v-chip color="primary" variant="tonal" size="small" class="font-weight-bold" :prepend-icon="Check">
                            {{ hasManagedPassphrase ? ProjectEncryption.Automatic : ProjectEncryption.Manual }}
                        </v-chip>
                        <v-divider class="my-4 border-0" />
                        <v-btn variant="outlined" color="default" :prepend-icon="View">
                            View Details
                            <project-encryption-information-dialog @new-project="isCreateProjectDialogShown = true" />
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <template v-if="isProjectOwner || (!isProjectOwner && ownerHasPaidPrivileges)">
            <v-row>
                <v-col>
                    <h3 class="mt-5">Project Limits</h3>
                </v-col>
            </v-row>

            <v-row v-if="!isProjectOwnerPaidTier && billingEnabled">
                <v-col cols="12" lg="4">
                    <v-card title="Free Trial" class="pa-2">
                        <v-card-subtitle>
                            {{ storageLimitFormatted }} Storage / {{ bandwidthLimitFormatted }} Bandwidth. <br>
                            Need more? Upgrade to Pro Account.
                        </v-card-subtitle>
                        <v-card-text>
                            <v-btn variant="flat" color="primary" :append-icon="ArrowRight" @click="toggleUpgradeFlow">
                                Upgrade
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>
            </v-row>

            <v-row v-else>
                <v-col cols="12" sm="6" lg="4">
                    <v-card title="Storage" class="pa-2">
                        <v-card-subtitle>
                            Limit: {{ storageLimitFormatted }} <br>
                            <span v-if="!noLimitsUiEnabled">Available Storage: {{ paidStorageLimitFormatted }}</span>
                        </v-card-subtitle>
                        <v-card-text>
                            <v-btn variant="outlined" color="default" :prepend-icon="InfinityIcon" @click="showStorageLimitDialog">
                                Edit Storage Limit
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>

                <v-col cols="12" sm="6" lg="4">
                    <v-card title="Download" class="pa-2">
                        <v-card-subtitle>
                            Limit: {{ bandwidthLimitFormatted }} {{ bandwidthLimitFormatted === 'No Limit' ? '' : 'per month' }}<br>
                            <span v-if="!noLimitsUiEnabled">Available Download: {{ paidBandwidthLimitFormatted }} per month</span>
                        </v-card-subtitle>
                        <v-card-text>
                            <v-btn variant="outlined" color="default" :prepend-icon="InfinityIcon" @click="showBandwidthLimitDialog">
                                Edit Download Limit
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>

                <v-col v-if="!noLimitsUiEnabled" cols="12" sm="6" lg="4">
                    <v-card title="Account Limits" class="pa-2">
                        <v-card-subtitle>
                            Storage limit: {{ paidStorageLimitFormatted }} <br>
                            Download limit: {{ paidBandwidthLimitFormatted }} per month
                        </v-card-subtitle>
                        <v-card-text>
                            <v-btn
                                variant="outlined"
                                color="default"
                                rounded="md"
                                :append-icon="ExternalLink"
                                @click="openRequestUrl"
                            >
                                Request Limits Increase
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>
            </v-row>
        </template>

        <template v-if="versioningUIEnabled">
            <v-row>
                <v-col>
                    <h3 class="mt-5">Features</h3>
                </v-col>
            </v-row>

            <v-row>
                <v-col v-if="project.isClassic" cols="12" sm="6" lg="4">
                    <v-card title="Migrate to new storage tiers" class="pa-2">
                        <v-card-subtitle>
                            This project uses legacy pricing tiers.
                        </v-card-subtitle>

                        <v-card-text>
                            <v-btn color="primary" @click="isMigrateDialogShown = true">
                                Migrate Project
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>

                <v-col cols="12" sm="6" lg="4">
                    <v-card title="Object Versioning" class="pa-2">
                        <v-card-subtitle>
                            Versioning is enabled for this project.
                        </v-card-subtitle>

                        <v-card-text>
                            <v-btn variant="outlined" color="default">
                                View Details
                                <versioning-info-dialog v-model="isVersioningDialogShown" />
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>

                <v-col v-if="objectLockUIEnabled" cols="12" sm="6" lg="4">
                    <v-card title="Object Lock" class="pa-2">
                        <v-card-subtitle>
                            Object Lock is enabled for this project.
                        </v-card-subtitle>

                        <v-card-text>
                            <v-btn color="default" variant="outlined">
                                View Details
                                <object-lock-info-dialog />
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>
            </v-row>
        </template>

        <template v-if="projectCanBeDeleted">
            <v-row>
                <v-col>
                    <h3 class="mt-5">Danger Zone</h3>
                </v-col>
            </v-row>

            <v-row>
                <v-col cols="12" sm="6" lg="4">
                    <v-card title="Delete Project" class="pa-2">
                        <v-card-text>
                            <v-chip color="default" variant="tonal" size="small" class="font-weight-bold">
                                Delete this project.
                            </v-chip>
                            <v-divider class="my-4 border-0" />
                            <v-btn variant="outlined" color="error" @click="isDeleteProjectDialogShown = true">
                                Delete
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>
            </v-row>
        </template>
    </v-container>

    <create-project-dialog v-model="isCreateProjectDialogShown" />
    <edit-project-details-dialog v-model="isEditDetailsDialogShown" :field="fieldToChange" />
    <edit-project-limit-dialog v-model="isEditLimitDialogShown" :limit-type="limitToChange" />
    <delete-project-dialog v-model="isDeleteProjectDialogShown" />
    <migrate-project-pricing-dialog v-model="isMigrateDialogShown" :project-id="project.id" @success="() => projectsStore.selectProject(project.id)" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VContainer,
    VCard,
    VCardText,
    VCardSubtitle,
    VDivider,
    VBtn,
    VCol,
    VRow,
    VChip,
} from 'vuetify/components';
import { ArrowRight, Check, Pencil, View, Infinity as InfinityIcon, ExternalLink } from 'lucide-vue-next';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { FieldToChange, LimitToChange, Project, ProjectEncryption } from '@/types/projects';
import { useUsersStore } from '@/store/modules/usersStore';
import { Memory, Size } from '@/utils/bytesSize';
import { useConfigStore } from '@/store/modules/configStore';
import { decimalShift } from '@/utils/strings';
import { useNotify } from '@/composables/useNotify';
import {
    AnalyticsErrorEventSource,
} from '@/utils/constants/analyticsEventNames';
import { useAppStore } from '@/store/modules/appStore';
import { usePreCheck } from '@/composables/usePreCheck';
import { ProjectRole } from '@/types/projectMembers';

import EditProjectDetailsDialog from '@/components/dialogs/EditProjectDetailsDialog.vue';
import EditProjectLimitDialog from '@/components/dialogs/EditProjectLimitDialog.vue';
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';
import VersioningInfoDialog from '@/components/dialogs/VersioningInfoDialog.vue';
import ProjectEncryptionInformationDialog from '@/components/dialogs/ProjectEncryptionInformationDialog.vue';
import CreateProjectDialog from '@/components/dialogs/CreateProjectDialog.vue';
import DeleteProjectDialog  from '@/components/dialogs/DeleteProjectDialog.vue';
import ObjectLockInfoDialog from '@/components/dialogs/ObjectLockInfoDialog.vue';
import MigrateProjectPricingDialog from '@/components/dialogs/MigrateProjectPricingDialog.vue';

const isCreateProjectDialogShown = ref<boolean>(false);
const isDeleteProjectDialogShown = ref<boolean>(false);
const isEditDetailsDialogShown = ref<boolean>(false);
const isEditLimitDialogShown = ref<boolean>(false);
const isVersioningDialogShown = ref<boolean>(false);
const isMigrateDialogShown = ref<boolean>(false);
const fieldToChange = ref<FieldToChange>(FieldToChange.Name);
const limitToChange = ref<LimitToChange>(LimitToChange.Storage);

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const notify = useNotify();
const { isTrialExpirationBanner, isUserProjectOwner, isExpired } = usePreCheck();

/**
 * Whether the new no-limits UI is enabled.
 */
const noLimitsUiEnabled = computed((): boolean => {
    return configStore.state.config.noLimitsUiEnabled;
});

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user));

/**
 * Returns selected project from the store.
 */
const project = computed<Project>(() => {
    return projectsStore.state.selectedProject;
});

const projectCanBeDeleted = computed(() => {
    return deleteProjectEnabled.value && isProjectOwner.value && !usersStore.state.user.externalID;
});

/**
 * Returns whether project deletion is enabled.
 */
const deleteProjectEnabled = computed(() => {
    return configStore.state.config.deleteProjectEnabled;
});

/**
 * Returns whether this project is owner by the current user
 */
const isProjectOwner = computed(() => {
    return project.value.ownerId === usersStore.state.user.id;
});

/**
 * Returns whether this project is owned by the current user
 * or whether they're an admin.
 */
const isProjectOwnerOrAdmin = computed(() => {
    const isAdmin = projectsStore.selectedProjectConfig.role === ProjectRole.Admin;
    return isProjectOwner.value || isAdmin;
});

/**
 * Whether versioning UI is enabled.
 */
const versioningUIEnabled = computed(() => configStore.state.config.versioningUIEnabled);

/**
 * Whether object lock UI is enabled.
 */
const objectLockUIEnabled = computed(() => configStore.state.config.objectLockUIEnabled);

/**
 * whether this project has a satellite managed passphrase.
 */
const hasManagedPassphrase = computed(() => projectsStore.state.selectedProjectConfig.hasManagedPassphrase);

/**
 * Indicates if satellite managed encryption passphrase is enabled.
 */
const satelliteManagedEncryptionEnabled = computed<boolean>(() => configStore.state.config.satelliteManagedEncryptionEnabled);

/**
 * Returns whether this project is owned by a paid tier user.
 */
const isProjectOwnerPaidTier = computed(() => projectsStore.selectedProjectConfig.isOwnerPaidTier);
/**
 * Returns whether the owner of this project has paid privileges
 */
const ownerHasPaidPrivileges = computed(() => projectsStore.selectedProjectConfig.hasPaidPrivileges);

/**
 * Returns the current project limits from store.
 */
const currentLimits = computed(() => projectsStore.state.currentLimits);

/**
 * Returns formatted storage limit.
 */
const storageLimitFormatted = computed<string>(() => {
    if (isProjectOwnerPaidTier.value && noLimitsUiEnabled.value && !currentLimits.value.userSetStorageLimit) {
        return 'No Limit';
    }
    return formatLimit(currentLimits.value.userSetStorageLimit || currentLimits.value.storageLimit);
});

/**
 * Returns formatted bandwidth limit.
 */
const bandwidthLimitFormatted = computed<string>(() => {
    if (isProjectOwnerPaidTier.value && noLimitsUiEnabled.value && !currentLimits.value.userSetBandwidthLimit) {
        return 'No Limit';
    }
    return formatLimit(currentLimits.value.userSetBandwidthLimit || currentLimits.value.bandwidthLimit);
});

/**
 * Returns formatted paid tier storage limit.
 */
const paidStorageLimitFormatted = computed<string>(() => {
    const limit = Math.max(
        projectsStore.state.currentLimits.storageLimit,
        parseConfigLimit(configStore.state.config.defaultPaidStorageLimit),
    );
    return formatLimit(limit);
});

/**
 * Returns formatted paid tier bandwidth limit.
 */
const paidBandwidthLimitFormatted = computed<string>(() => {
    const limit = Math.max(
        projectsStore.state.currentLimits.bandwidthLimit,
        parseConfigLimit(configStore.state.config.defaultPaidBandwidthLimit),
    );
    return formatLimit(limit);
});

/**
 * Returns project limits increase request URL from config.
 */
const projectLimitsIncreaseRequestURL = computed((): string => {
    return configStore.state.config.projectLimitsIncreaseRequestURL;
});

function toggleUpgradeFlow(): void {
    appStore.toggleUpgradeFlow(true);
}

/**
 * Returns formatted limit value.
 */
function formatLimit(limit: number): string {
    const size = new Size(limit, 2);
    return `${decimalShift(size.formattedBytes, 0)} ${size.label}`;
}

/**
 * Displays the Edit Project Name dialog.
 */
function showEditNameDialog(): void {
    fieldToChange.value = FieldToChange.Name;
    isEditDetailsDialogShown.value = true;
}

/**
 * Displays the Edit Description dialog.
 */
function showEditDescriptionDialog(): void {
    fieldToChange.value = FieldToChange.Description;
    isEditDetailsDialogShown.value = true;
}

/**
 * Displays the Storage Limit dialog.
 */
function showStorageLimitDialog(): void {
    if (!isProjectOwnerOrAdmin.value) {
        notify.notify('Contact project owner or admin to request increase');
        return;
    }
    limitToChange.value = LimitToChange.Storage;
    isEditLimitDialogShown.value = true;
}

/**
 * Displays the Bandwidth Limit dialog.
 */
function showBandwidthLimitDialog(): void {
    if (!isProjectOwnerOrAdmin.value) {
        notify.notify('Contact project owner or admin to request increase');
        return;
    }
    limitToChange.value = LimitToChange.Bandwidth;
    isEditLimitDialogShown.value = true;
}

function openRequestUrl(): void {
    if (!isProjectOwnerOrAdmin.value) {
        notify.notify('Contact project owner or admin to request increase');
        return;
    }
    window.open(projectLimitsIncreaseRequestURL.value, '_blank', 'noreferrer');
}

/**
 * Parses limit value from config, returning it as a byte amount.
 */
function parseConfigLimit(limit: string): number {
    const [value, unit] = limit.split(' ');
    return parseFloat(value) * Memory[unit === 'B' ? 'Bytes' : unit];
}

/**
 * Lifecycle hook after initial render.
 * Fetches project limits.
 */
onMounted(async () => {
    try {
        await projectsStore.getProjectLimits(project.value.id);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_SETTINGS_AREA);
    }
});
</script>

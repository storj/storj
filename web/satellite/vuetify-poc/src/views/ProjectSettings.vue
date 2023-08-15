// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <h1 class="text-h5 font-weight-bold mb-2">Project Settings</h1>

        <v-card class="my-6">
            <v-list lines="three">
                <v-list-subheader class="mb-2">Details</v-list-subheader>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Project Name</v-list-item-title>
                    <v-list-item-subtitle>{{ project.name }}</v-list-item-subtitle>
                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="outlined" color="default" size="small" @click="showEditNameDialog">
                                Edit Project Name
                            </v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>

                <v-divider />

                <v-list-item>
                    <v-list-item-title>Description</v-list-item-title>
                    <v-list-item-subtitle>{{ project.description }}</v-list-item-subtitle>
                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="outlined" color="default" size="small" @click="showEditDescriptionDialog">
                                Edit Description
                            </v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>
            </v-list>
        </v-card>

        <v-card class="my-6">
            <v-list lines="three">
                <v-list-subheader class="mb-2">Limits</v-list-subheader>

                <v-divider />

                <v-list-item v-if="!isPaidTier">
                    <v-list-item-title>Free Account</v-list-item-title>
                    <v-list-item-subtitle>
                        {{ storageLimitFormatted }} Storage / {{ bandwidthLimitFormatted }} Bandwidth.
                        Need more? Upgrade now.
                    </v-list-item-subtitle>
                    <template #append>
                        <v-list-item-action>
                            <v-btn variant="flat" color="primary" size="small">
                                Upgrade to Pro
                            </v-btn>
                        </v-list-item-action>
                    </template>
                </v-list-item>

                <template v-else>
                    <v-list-item>
                        <v-list-item-title>Storage</v-list-item-title>
                        <v-list-item-subtitle>
                            {{ storageLimitFormatted }} of {{ paidStorageLimitFormatted }} available storage.
                        </v-list-item-subtitle>
                        <template #append>
                            <v-list-item-action>
                                <v-btn variant="outlined" color="default" size="small" @click="showStorageLimitDialog">
                                    Edit Storage Limit
                                </v-btn>
                            </v-list-item-action>
                        </template>
                    </v-list-item>

                    <v-divider />

                    <v-list-item>
                        <v-list-item-title>Bandwidth</v-list-item-title>
                        <v-list-item-subtitle>
                            {{ bandwidthLimitFormatted }} of {{ paidBandwidthLimitFormatted }} available bandwidth.
                        </v-list-item-subtitle>
                        <template #append>
                            <v-list-item-action>
                                <v-btn variant="outlined" color="default" size="small" @click="showBandwidthLimitDialog">
                                    Edit Bandwidth Limit
                                </v-btn>
                            </v-list-item-action>
                        </template>
                    </v-list-item>

                    <v-divider />

                    <v-list-item>
                        <v-list-item-title>Account</v-list-item-title>
                        <v-list-item-subtitle>
                            {{ paidStorageLimitFormatted }} storage and {{ paidBandwidthLimitFormatted }} bandwidth per month.
                        </v-list-item-subtitle>
                        <template #append>
                            <v-list-item-action>
                                <v-btn
                                    variant="outlined"
                                    color="default"
                                    size="small"
                                    :href="projectLimitsIncreaseRequestURL"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    Request Increase
                                </v-btn>
                            </v-list-item-action>
                        </template>
                    </v-list-item>
                </template>
            </v-list>
        </v-card>
    </v-container>

    <edit-project-details-dialog v-model="isEditDetailsDialogShown" :field="fieldToChange" />
    <edit-project-limit-dialog v-model="isEditLimitDialogShown" :limit-type="limitToChange" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VContainer,
    VCard,
    VList,
    VListSubheader,
    VDivider,
    VListItem,
    VListItemTitle,
    VListItemSubtitle,
    VListItemAction,
    VBtn,
} from 'vuetify/components';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { FieldToChange, LimitToChange, Project } from '@/types/projects';
import { useUsersStore } from '@/store/modules/usersStore';
import { Memory, Size } from '@/utils/bytesSize';
import { useConfigStore } from '@/store/modules/configStore';
import { decimalShift } from '@/utils/strings';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import EditProjectDetailsDialog from '@poc/components/dialogs/EditProjectDetailsDialog.vue';
import EditProjectLimitDialog from '@poc/components/dialogs/EditProjectLimitDialog.vue';

const isEditDetailsDialogShown = ref<boolean>(false);
const isEditLimitDialogShown = ref<boolean>(false);
const fieldToChange = ref<FieldToChange>(FieldToChange.Name);
const limitToChange = ref<LimitToChange>(LimitToChange.Storage);

const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const notify = useNotify();

/**
 * Returns selected project from the store.
 */
const project = computed<Project>(() => {
    return projectsStore.state.selectedProject;
});

/**
 * Returns user's paid tier status from store.
 */
const isPaidTier = computed<boolean>(() => {
    return usersStore.state.user.paidTier;
});

/**
 * Returns formatted storage limit.
 */
const storageLimitFormatted = computed<string>(() => {
    return formatLimit(projectsStore.state.currentLimits.storageLimit);
});

/**
 * Returns formatted bandwidth limit.
 */
const bandwidthLimitFormatted = computed<string>(() => {
    return formatLimit(projectsStore.state.currentLimits.bandwidthLimit);
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
    limitToChange.value = LimitToChange.Storage;
    isEditLimitDialogShown.value = true;
}

/**
 * Displays the Bandwidth Limit dialog.
 */
function showBandwidthLimitDialog(): void {
    limitToChange.value = LimitToChange.Bandwidth;
    isEditLimitDialogShown.value = true;
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
        notify.error(`Error fetching project limits. ${error.message}`, AnalyticsErrorEventSource.PROJECT_SETTINGS_AREA);
    }
});
</script>

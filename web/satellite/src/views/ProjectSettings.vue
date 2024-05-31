// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col>
                <trial-expiration-banner v-if="isTrialExpirationBanner && isUserProjectOwner" :expired="isExpired" />

                <PageTitleComponent title="Project Settings" />
                <PageSubtitleComponent subtitle="Edit project information and set custom project limits." link="https://docs.storj.io/learn/concepts/limits" />
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" lg="4">
                <v-card title="Project Name">
                    <v-card-text>
                        <v-chip color="default" variant="tonal" size="small" class="font-weight-bold">
                            {{ project.name }}
                        </v-chip>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" color="default" size="small" rounded="md" @click="showEditNameDialog">
                            Edit Name
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
            <v-col cols="12" lg="4">
                <v-card title="Project Description">
                    <v-card-text>
                        <v-chip color="default" variant="tonal" size="small" class="font-weight-bold">
                            {{ project.description }}
                        </v-chip>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" color="default" size="small" rounded="md" @click="showEditDescriptionDialog">
                            Edit Description
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
            <v-col cols="12" lg="4">
                <v-card title="Delete Project">
                    <v-card-text>
                        <v-chip color="default" variant="tonal" size="small" class="font-weight-bold" disabled>
                            Not Available
                        </v-chip>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" color="default" size="small" rounded="md" href="https://docs.storj.io/support/projects#delete-the-existing-project" target="_blank">
                            Learn More
                            <v-icon end :icon="mdiOpenInNew" />
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row>
            <v-col>
                <h3 class="mt-5">Limits</h3>
            </v-col>
        </v-row>

        <v-row v-if="!isPaidTier && billingEnabled">
            <v-col cols="12">
                <v-card title="Free Trial">
                    <v-card-subtitle>
                        {{ storageLimitFormatted }} Storage / {{ bandwidthLimitFormatted }} Bandwidth. <br>
                        Need more? Upgrade to Pro Account.
                    </v-card-subtitle>
                    <v-card-text>
                        <v-divider class="mb-4" />
                        <v-btn variant="flat" color="primary" size="small" rounded="md" :append-icon="mdiArrowRight" @click="toggleUpgradeFlow">
                            Upgrade
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row v-else>
            <v-col cols="12" lg="4">
                <v-card title="Storage Limit">
                    <v-card-subtitle>
                        Storage Limit: {{ storageLimitFormatted }} <br>
                        Available Storage: {{ paidStorageLimitFormatted }}
                    </v-card-subtitle>
                    <v-card-text>
                        <v-divider class="mb-4" />
                        <v-btn variant="outlined" color="default" size="small" rounded="md" @click="showStorageLimitDialog">
                            Edit Storage Limit
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" lg="4">
                <v-card title="Download Limit">
                    <v-card-subtitle>
                        Download Limit: {{ bandwidthLimitFormatted }} per month<br>
                        Available Download: {{ paidBandwidthLimitFormatted }} per month
                    </v-card-subtitle>
                    <v-card-text>
                        <v-divider class="mb-4" />
                        <v-btn variant="outlined" color="default" size="small" rounded="md" @click="showBandwidthLimitDialog">
                            Edit Download Limit
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" lg="4">
                <v-card title="Account Limits">
                    <v-card-subtitle>
                        Storage limit: {{ paidStorageLimitFormatted }} <br>
                        Download limit: {{ paidBandwidthLimitFormatted }} per month
                    </v-card-subtitle>
                    <v-card-text>
                        <v-divider class="mb-4" />
                        <v-btn
                            variant="outlined"
                            color="default"
                            size="small"
                            :href="projectLimitsIncreaseRequestURL"
                            target="_blank"
                            rel="noopener noreferrer"
                            rounded="md"
                        >
                            Request Limits Increase
                            <v-icon end :icon="mdiOpenInNew" />
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <template v-if="promptForVersioningBeta || versioningUIEnabled">
            <v-row>
                <v-col>
                    <h3 class="mt-5">Features</h3>
                </v-col>
            </v-row>

            <v-row>
                <v-col cols="12" lg="4">
                    <v-card title="Object Versioning">
                        <v-card-subtitle v-if="versioningUIEnabled">
                            Versioning (beta) is enabled for this project.
                        </v-card-subtitle>
                        <v-card-subtitle v-else>
                            Enable object versioning (beta) on this project.
                        </v-card-subtitle>

                        <v-card-text>
                            <v-divider class="mb-4" />
                            <v-btn v-if="allowVersioningToggle" color="primary" size="small">
                                Learn More
                                <versioning-beta-dialog v-model="isVersioningDialogShown" />
                            </v-btn>
                            <v-btn v-else-if="versioningUIEnabled" :prepend-icon="mdiInformationOutline" color="primary" size="small">
                                Enabled
                                <versioning-beta-dialog info />
                            </v-btn>
                        </v-card-text>
                    </v-card>
                </v-col>
            </v-row>
        </template>
    </v-container>

    <edit-project-details-dialog v-model="isEditDetailsDialogShown" :field="fieldToChange" />
    <edit-project-limit-dialog v-model="isEditLimitDialogShown" :limit-type="limitToChange" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import {
    VContainer,
    VCard,
    VCardText,
    VCardSubtitle,
    VDivider,
    VBtn,
    VCol,
    VRow,
    VIcon,
    VChip,
} from 'vuetify/components';
import { mdiOpenInNew, mdiArrowRight, mdiInformationOutline } from '@mdi/js';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { FieldToChange, LimitToChange, Project } from '@/types/projects';
import { useUsersStore } from '@/store/modules/usersStore';
import { Memory, Size } from '@/utils/bytesSize';
import { useConfigStore } from '@/store/modules/configStore';
import { decimalShift } from '@/utils/strings';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useAppStore } from '@/store/modules/appStore';
import { useTrialCheck } from '@/composables/useTrialCheck';

import EditProjectDetailsDialog from '@/components/dialogs/EditProjectDetailsDialog.vue';
import EditProjectLimitDialog from '@/components/dialogs/EditProjectLimitDialog.vue';
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';
import VersioningBetaDialog from '@/components/dialogs/VersioningBetaDialog.vue';

const isEditDetailsDialogShown = ref<boolean>(false);
const isEditLimitDialogShown = ref<boolean>(false);
const isVersioningDialogShown = ref<boolean>(false);
const allowVersioningToggle = ref<boolean>(false);
const fieldToChange = ref<FieldToChange>(FieldToChange.Name);
const limitToChange = ref<LimitToChange>(LimitToChange.Storage);

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const notify = useNotify();
const { isTrialExpirationBanner, isUserProjectOwner, isExpired } = useTrialCheck();

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user.hasVarPartner));

/**
 * Returns selected project from the store.
 */
const project = computed<Project>(() => {
    return projectsStore.state.selectedProject;
});

const promptForVersioningBeta = computed<boolean>(() => projectsStore.promptForVersioningBeta);

/**
 * Whether versioning has been enabled for current project.
 */
const versioningUIEnabled = computed(() => projectsStore.versioningUIEnabled);

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

watch(() => [projectsStore.promptForVersioningBeta, isVersioningDialogShown.value], (values) => {
    if (values[0] && !allowVersioningToggle.value) {
        allowVersioningToggle.value = true;
    } else if (!values[0] && !values[1] && allowVersioningToggle.value) {
        // throttle the banner dismissal for the dialog close animation.
        setTimeout(() => allowVersioningToggle.value = false, 500);
    }
}, { immediate: true });
</script>

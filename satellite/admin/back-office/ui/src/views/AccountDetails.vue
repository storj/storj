// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col cols="12" md="8">
                <PageTitleComponent title="Account Details" />
                <v-chip class="mr-2 mb-2 mb-md-0 pr-4 font-weight-medium" color="default">
                    <v-tooltip activator="parent" location="top">
                        This account
                    </v-tooltip>
                    <template #prepend>
                        <svg class="mr-1" width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path
                                d="M12.1271 6C14.1058 6 15.7099 7.60408 15.7099 9.58281C15.7099 10.7701 15.1324 11.8226 14.2429 12.4745C16.0273 13.1299 17.4328 14.5717 18.0402 16.3804C18.059 16.4363 18.077 16.4925 18.0942 16.5491L18.1195 16.6342C18.2377 17.0429 18.0022 17.4701 17.5934 17.5883C17.5239 17.6084 17.4518 17.6186 17.3794 17.6186H6.764C6.34206 17.6186 6 17.2765 6 16.8545C6 16.7951 6.00695 16.7358 6.02069 16.678L6.02974 16.6434C6.05458 16.5571 6.08121 16.4714 6.10959 16.3866C6.7237 14.5517 8.15871 13.0936 9.97792 12.4495C9.10744 11.7961 8.54432 10.7552 8.54432 9.58281C8.54432 7.60408 10.1484 6 12.1271 6ZM12.076 13.2168C9.95096 13.2168 8.07382 14.5138 7.29168 16.4355L7.26883 16.4925H16.8831L16.8826 16.4916C16.1224 14.5593 14.2607 13.2444 12.1429 13.2173L12.076 13.2168ZM12.1271 7.12603C10.7703 7.12603 9.67035 8.22596 9.67035 9.58281C9.67035 10.9397 10.7703 12.0396 12.1271 12.0396C13.4839 12.0396 14.5839 10.9397 14.5839 9.58281C14.5839 8.22596 13.4839 7.12603 12.1271 7.12603Z"
                                fill="currentColor"
                            />
                        </svg>
                    </template>
                    {{ userAccount.email }}
                </v-chip>

                <v-chip class="mr-2 mb-2 mb-md-0" variant="text">
                    Customer for {{ Math.floor((Date.now() - createdAt.getTime()) / MS_PER_DAY).toLocaleString() }} days
                    <v-tooltip activator="parent" location="top">
                        Account created:
                        {{ createdAt.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' }) }}
                    </v-tooltip>
                </v-chip>
            </v-col>
            <v-col cols="12" md="4" class="d-flex justify-start justify-md-end align-top align-md-center">
                <v-btn>
                    <template #append>
                        <v-icon icon="mdi-chevron-down" />
                    </template>
                    Account Actions
                    <AccountActionsMenu />
                </v-btn>
            </v-col>
        </v-row>

        <v-row v-if="usageCacheError">
            <v-col>
                <v-alert variant="tonal" color="error" rounded="lg" density="comfortable" border>
                    <div class="d-flex align-center">
                        <v-icon icon="mdi-alert-circle" color="error" class="mr-3" />
                        An error occurred when retrieving project usage data.
                        Please retry after a few minutes and report the issue if it persists.
                    </div>
                </v-alert>
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" sm="6" md="3">
                <v-card title="Account" :subtitle="userAccount.fullName" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip :color="userAccount.paidTier ? 'success' : 'default'" variant="tonal" class="mr-2 font-weight-bold">
                            {{ userAccount.paidTier ? 'Pro' : 'Free' }}
                        </v-chip>
                        <template v-if="featureFlags.account.updateInfo">
                            <v-divider class="my-4" />
                            <v-btn variant="outlined" size="small" color="default">
                                Edit Account Information
                                <AccountInformationDialog />
                            </v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <v-card title="Status" subtitle="Account" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip color="success" variant="tonal" class="mr-2 font-weight-bold">
                            {{ userAccount.status }}
                        </v-chip>
                        <template v-if="featureFlags.account.updateStatus">
                            <v-divider class="my-4" />
                            <v-btn variant="outlined" size="small" color="default">
                                Set Account Status
                                <AccountStatusDialog />
                            </v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <v-card title="Value" subtitle="Attribution" variant="flat" :border="true" rounded="xlg" class="mb-3">
                    <v-card-text>
                        <!-- <p class="mb-3">Attribution</p> -->
                        <v-chip :variant="userAccount.userAgent ? 'tonal' : 'text'" class="mr-2">
                            {{ userAccount.userAgent || 'None' }}
                        </v-chip>
                        <template v-if="featureFlags.account.updateValueAttribution">
                            <v-divider class="my-4" />
                            <v-btn variant="outlined" size="small" color="default">
                                Set Value Attribution
                                <AccountUserAgentsDialog />
                            </v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <v-card title="Placement" subtitle="Region" variant="flat" :border="true" rounded="xlg">
                    <v-card-text>
                        <!-- <p class="mb-3">Region</p> -->
                        <v-chip variant="tonal" class="mr-2">
                            {{ placementText }}
                        </v-chip>
                        <template v-if="featureFlags.account.updatePlacement">
                            <v-divider class="my-4" />
                            <v-btn variant="outlined" size="small" color="default">
                                Set Account Placement
                                <AccountGeofenceDialog />
                            </v-btn>
                        </template>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" sm="6" md="3">
                <card-stats-component title="Projects" subtitle="Total" :data="userAccount.projects?.length.toString() || '0'" />
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <card-stats-component title="Storage" subtitle="Total">
                    <template #data>
                        <v-chip v-if="totalUsage.storage !== null" class="font-weight-bold">
                            {{ sizeToBase10String(totalUsage.storage) }}
                        </v-chip>
                        <v-icon v-else icon="mdi-alert-circle-outline" color="error" size="x-large" />
                    </template>
                </card-stats-component>
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <card-stats-component title="Download" subtitle="This month" :data="sizeToBase10String(totalUsage.download)" />
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <card-stats-component title="Segments" subtitle="Total">
                    <template #data>
                        <v-chip v-if="totalUsage.segments !== null" class="font-weight-bold">
                            {{ totalUsage.segments.toLocaleString() }}
                        </v-chip>
                        <v-icon v-else icon="mdi-alert-circle-outline" color="error" size="x-large" />
                    </template>
                </card-stats-component>
            </v-col>
        </v-row>

        <v-row v-if="featureFlags.account.projects">
            <v-col>
                <h3 class="my-4">Projects</h3>
                <AccountProjectsTableComponent />
            </v-col>
        </v-row>

        <v-row v-if="featureFlags.account.history">
            <v-col>
                <h3 class="my-4">History</h3>
                <LogsTableComponent />
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, onUnmounted } from 'vue';
import { useRouter } from 'vue-router';
import {
    VContainer,
    VRow,
    VCol,
    VChip,
    VTooltip,
    VIcon,
    VCard,
    VCardText,
    VDivider,
    VBtn,
    VAlert,
} from 'vuetify/components';

import { FeatureFlags, UserAccount } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { sizeToBase10String } from '@/utils/memory';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import AccountProjectsTableComponent from '@/components/AccountProjectsTableComponent.vue';
import LogsTableComponent from '@/components/LogsTableComponent.vue';
import AccountActionsMenu from '@/components/AccountActionsMenu.vue';
import AccountUserAgentsDialog from '@/components/AccountUserAgentsDialog.vue';
import AccountGeofenceDialog from '@/components/AccountGeofenceDialog.vue';
import AccountInformationDialog from '@/components/AccountInformationDialog.vue';
import AccountStatusDialog from '@/components/AccountStatusDialog.vue';
import CardStatsComponent from '@/components/CardStatsComponent.vue';

const MS_PER_DAY = 1000 * 60 * 60 * 24;

const appStore = useAppStore();
const router = useRouter();
const featureFlags = appStore.state.settings.admin.features as FeatureFlags;

/**
 * Returns user account info from store.
 */
const userAccount = computed<UserAccount>(() => appStore.state.userAccount as UserAccount);

/**
 * Returns the date that the user was created.
 */
const createdAt = computed<Date>(() => new Date(userAccount.value.createdAt));

type Usage = {
    storage: number | null;
    download: number;
    segments: number | null;
};

/**
 * Returns the user's total project usage.
 */
const totalUsage = computed<Usage>(() => {
    const total: Usage = {
        storage: 0,
        download: 0,
        segments: 0,
    };

    if (!userAccount.value.projects?.length) {
        return total;
    }

    for (const project of userAccount.value.projects) {
        if (total.storage !== null) {
            total.storage = project.storageUsed !== null ? total.storage + project.storageUsed : null;
        }
        if (total.segments !== null) {
            total.segments = project.segmentUsed !== null ? total.segments + project.segmentUsed : null;
        }
        total.download += project.bandwidthUsed;
    }

    return total;
});

const placementText = computed<string>(() => {
    return appStore.getPlacementText(userAccount.value.defaultPlacement);
});

/**
 * Returns whether an error occurred retrieving usage data from the Redis live accounting cache.
 */
const usageCacheError = computed<boolean>(() => {
    return !!userAccount.value.projects?.some(project =>
        project.storageUsed === null ||
        project.segmentUsed === null,
    );
});

onBeforeMount(() => !userAccount.value && router.push('/accounts'));
onUnmounted(() => {
    if (appStore.state.selectedProject === null) {
        appStore.clearUser;
    }
});
</script>

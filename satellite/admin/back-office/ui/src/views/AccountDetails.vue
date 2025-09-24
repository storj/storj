// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isLoading || !userAccount" class="d-flex justify-center align-center" style="height: 100vh;">
        <v-skeleton-loader width="300" height="200" type="article" />
    </div>
    <v-container v-else>
        <v-row>
            <v-col cols="12" md="8">
                <PageTitleComponent title="Account Details" />
                <v-chip class="mr-2 mb-2 mb-md-0 pr-4 font-weight-medium" color="default">
                    <v-tooltip activator="parent" location="top">
                        This account
                    </v-tooltip>
                    <template #prepend>
                        <v-icon class="mr-1" :icon="User" />
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
                        <v-icon :icon="ChevronDown" />
                    </template>
                    Account Actions
                    <AccountActionsMenu
                        :user="userAccount"
                        @update="updateAccountDialogEnabled = true"
                        @toggle-freeze="toggleFreeze"
                    />
                </v-btn>
            </v-col>
        </v-row>

        <v-row v-if="usageCacheError">
            <v-col>
                <v-alert variant="tonal" color="error" rounded="lg" density="comfortable" border>
                    <div class="d-flex align-center">
                        <v-icon :icon="AlertCircle" color="error" class="mr-3" />
                        An error occurred when retrieving project usage data.
                        Please retry after a few minutes and report the issue if it persists.
                    </div>
                </v-alert>
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" sm="6" md="3">
                <v-card :title="userAccount.fullName" :subtitle="kindDetails" variant="flat" :border="true" rounded="xlg">
                    <template #subtitle>
                        <span v-if="kindDetails">{{ kindDetails }}</span>
                        <span v-else>&nbsp;</span>
                    </template>
                    <template #append>
                        <v-btn
                            icon
                            variant="outlined"
                            size="small"
                            color="default"
                            density="comfortable"
                        >
                            <v-icon :icon="MoreHorizontal" />
                            <AccountActionsMenu
                                :user="userAccount"
                                @update="updateAccountDialogEnabled = true"
                                @toggle-freeze="toggleFreeze"
                            />
                        </v-btn>
                    </template>
                    <v-card-text>
                        <v-chip
                            :color="userIsPaid(userAccount) ? 'success' : userIsNFR(userAccount) ? 'warning' : 'info'"
                            variant="tonal" class="mr-2 font-weight-bold"
                        >
                            {{ userAccount.kind.name }}
                        </v-chip>
                        <v-chip
                            v-if="userAccount.freezeStatus"
                            color="error"
                            variant="tonal"
                            class="mr-2 font-weight-bold"
                        >
                            {{ userAccount.freezeStatus?.name }}
                        </v-chip>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <v-card title="Status" subtitle="Account" variant="flat" :border="true" rounded="xlg">
                    <template v-if="featureFlags.account.updateStatus && !userAccount.freezeStatus" #append>
                        <v-btn
                            :icon="UserPen"
                            variant="outlined" size="small"
                            density="comfortable" color="default"
                            @click="updateAccountDialogEnabled = true"
                        />
                    </template>
                    <v-card-text>
                        <v-chip :color="statusColor" variant="tonal" class="mr-2 font-weight-bold">
                            {{ userAccount.status.name }}
                        </v-chip>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <v-card title="Attribution" subtitle="User agent" variant="flat" :border="true" rounded="xlg" class="mb-3">
                    <template v-if="featureFlags.account.updateUserAgent" #append>
                        <v-btn
                            :icon="UserPen"
                            variant="outlined" size="small"
                            density="comfortable" color="default"
                            @click="updateAccountDialogEnabled = true"
                        />
                    </template>
                    <v-card-text>
                        <v-chip variant="tonal" class="mr-2">
                            {{ userAccount.userAgent || 'None' }}
                        </v-chip>
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
                <AccountProjectsTableComponent :account="userAccount" />
            </v-col>
        </v-row>

        <v-row v-if="featureFlags.account.history">
            <v-col>
                <h3 class="my-4">History</h3>
                <LogsTableComponent />
            </v-col>
        </v-row>
    </v-container>
    <AccountFreezeDialog v-if="userAccount" v-model="freezeDialogEnabled" :account="userAccount" />
    <AccountUpdateDialog v-if="userAccount" v-model="updateAccountDialogEnabled" :account="userAccount" />
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
import { useRouter } from 'vue-router';
import {
    VAlert,
    VBtn,
    VCard,
    VCardText,
    VChip,
    VCol,
    VContainer,
    VDivider,
    VIcon,
    VRow,
    VSkeletonLoader,
    VTooltip,
} from 'vuetify/components';
import { AlertCircle, ChevronDown, MoreHorizontal, User, UserPen } from 'lucide-vue-next';
import { useDate } from 'vuetify';

import { FeatureFlags, UserAccount } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { userIsFree, userIsNFR, userIsPaid } from '@/types/user';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/users';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { sizeToBase10String } from '@/utils/memory';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import AccountProjectsTableComponent from '@/components/AccountProjectsTableComponent.vue';
import LogsTableComponent from '@/components/LogsTableComponent.vue';
import AccountActionsMenu from '@/components/AccountActionsMenu.vue';
import AccountGeofenceDialog from '@/components/AccountGeofenceDialog.vue';
import CardStatsComponent from '@/components/CardStatsComponent.vue';
import AccountFreezeDialog from '@/components/AccountFreezeDialog.vue';
import AccountUpdateDialog from '@/components/AccountUpdateDialog.vue';

const MS_PER_DAY = 1000 * 60 * 60 * 24;

const usersStore = useUsersStore();
const appStore = useAppStore();
const router = useRouter();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const date = useDate();

const unfreezing = ref<boolean>(false);
const freezeDialogEnabled = ref<boolean>(false);
const updateAccountDialogEnabled = ref<boolean>(false);

const statusColor = computed(() => {
    if (!userAccount.value) {
        return 'default';
    }
    const status = userAccount.value.status.name.toLowerCase();
    if (status.includes('deletion') || status.includes('deleted')) {
        return 'error';
    }
    if (status.includes('active')) {
        return 'success';
    }

    return 'warning';
});

/**
 * Return the enabled feature flags from the store.
 */
const featureFlags = computed(() => appStore.state.settings.admin.features as FeatureFlags);

/**
 * Returns user account info from store.
 */
const userAccount = computed<UserAccount>(() => usersStore.state.currentAccount as UserAccount);

/**
 * Returns the date that the user was created.
 */
const createdAt = computed<Date>(() => new Date(userAccount.value.createdAt));

const kindDetails = computed<string>(() => {
    if (!userAccount.value || userIsNFR(userAccount.value)) {
        return '';
    }
    if (userIsPaid(userAccount.value)) {
        if (!userAccount.value.upgradeTime)
            return '';
        return `Upgraded on ${date.format(userAccount.value.upgradeTime, 'fullDate')}`;
    }
    if (userIsFree(userAccount.value)) {
        if (!userAccount.value.trialExpiration)
            return '';
        return `Expires on ${date.format(userAccount.value.trialExpiration, 'fullDate')}`;
    }
    return '';
});

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

function toggleFreeze() {
    if (userAccount.value.freezeStatus) {
        unfreezeAccount();
        return;
    }
    freezeDialogEnabled.value = true;
}

async function unfreezeAccount() {
    if (unfreezing.value) return;
    if (!userAccount.value.freezeStatus) {
        notify.error('Account is not frozen.');
        return;
    }
    unfreezing.value = true;
    try {
        await usersStore.unfreezeUser(userAccount.value.id);
        notify.success('Account unfrozen successfully.');

        await usersStore.updateCurrentUser(userAccount.value.id);
    } catch (error) {
        notify.error(`Failed to toggle account freeze status. ${error.message}`);
        return;
    } finally {
        unfreezing.value = false;
    }
}

onBeforeMount(() => {
    if (userAccount.value) {
        return;
    }
    withLoading(async () => {
        try {
            const userID = router.currentRoute.value.params.userID as string;
            await usersStore.updateCurrentUser(userID);
        } catch (error) {
            notify.error(`Failed to get account details. ${error.message}`);
            router.push({ name: ROUTES.AccountSearch.name });
        }
    });
});
</script>

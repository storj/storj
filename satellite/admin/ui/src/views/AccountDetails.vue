// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container v-if="userAccount">
        <div class="d-flex ga-2 flex-wrap justify-space-between align-center mb-5">
            <div>
                <PageTitleComponent title="Account Details" />
                <div class="d-flex ga-2 flex-wrap justify-start align-center mt-3">
                    <v-chip v-tooltip="'This account'" class="pl-4" :prepend-icon="User">
                        <span class="font-weight-medium text-truncate">{{ userAccount.email }} </span>
                    </v-chip>

                    <v-chip>
                        Customer for {{ date.getDiff(Date.now(), createdAt, 'days') }} day(s)
                        <v-tooltip activator="parent" location="top">
                            Account created:
                            {{ date.format(createdAt, 'fullDate') }}
                        </v-tooltip>
                    </v-chip>
                </div>
            </div>

            <v-btn :append-icon="ChevronDown">
                Account Actions
                <AccountActionsMenu
                    :user="userAccount"
                    @update="updateAccountDialogEnabled = true"
                    @update-limits="updateLimitsDialogEnabled = true"
                    @delete="deleteAccountDialogEnabled = true"
                    @disable-mfa="disableMFADialogEnabled = true"
                    @toggle-freeze="toggleFreeze"
                    @create-rest-key="createRestKeyDialogEnabled = true"
                    @mark-pending-deletion="_ => {
                        deleteAccountDialogEnabled = true;
                        markPendingDeletion = true;
                    }"
                />
            </v-btn>
        </div>

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
                <v-card :title="userAccount.fullName || '__'" :subtitle="kindDetails" variant="flat" :border="true" rounded="xlg">
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
                                @update-limits="updateLimitsDialogEnabled = true"
                                @delete="deleteAccountDialogEnabled = true"
                                @disable-mfa="disableMFADialogEnabled = true"
                                @toggle-freeze="toggleFreeze"
                                @create-rest-key="createRestKeyDialogEnabled = true"
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
                <card-stats-component
                    title="Projects"
                    subtitle="Total"
                    :used="userAccount.projects?.length || 0"
                    :limit="userAccount.projectLimit"
                    :update-disabled="!!userAccount.freezeStatus"
                    @update-limits="updateLimitsDialogEnabled = true"
                />
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <card-stats-component
                    title="Storage"
                    subtitle="Total"
                    :used="new Size(totalUsage.storage || 0)"
                    :limit="new Size(userAccount.storageLimit, 2)"
                    :update-disabled="!!userAccount.freezeStatus"
                    @update-limits="updateLimitsDialogEnabled = true"
                />
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <card-stats-component
                    title="Download"
                    subtitle="This month"
                    :used="new Size(totalUsage.download || 0)"
                    :limit="new Size(userAccount.bandwidthLimit, 2)"
                    :update-disabled="!!userAccount.freezeStatus"
                    @update-limits="updateLimitsDialogEnabled = true"
                />
            </v-col>

            <v-col cols="12" sm="6" md="3">
                <card-stats-component
                    title="Segments"
                    subtitle="Total"
                    :used="totalUsage.segments || 0"
                    :limit="userAccount.segmentLimit"
                    :data="userAccount.segmentLimit"
                    :update-disabled="!!userAccount.freezeStatus"
                    @update-limits="updateLimitsDialogEnabled = true"
                />
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
                <AccountHistoryTableComponent :account="userAccount" />
            </v-col>
        </v-row>
    </v-container>
    <AccountFreezeDialog v-if="userAccount" v-model="freezeDialogEnabled" :account="userAccount" />
    <AccountUpdateDialog v-if="userAccount" v-model="updateAccountDialogEnabled" :account="userAccount" />
    <AccountUpdateLimitsDialog v-if="userAccount" v-model="updateLimitsDialogEnabled" :account="userAccount" />
    <AccountDeleteDialog v-if="userAccount" v-model="deleteAccountDialogEnabled" v-model:mark-pending-deletion="markPendingDeletion" :account="userAccount" />
    <AccountDisableMFADialog v-if="userAccount" v-model="disableMFADialogEnabled" :account="userAccount" />
    <AccountCreateRestKeyDialog v-if="userAccount" v-model="createRestKeyDialogEnabled" :account="userAccount" />
    <AccountUnfreezeDialog v-if="userAccount" v-model="unfreezeDialogEnabled" :account="userAccount" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
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
    VTooltip,
} from 'vuetify/components';
import { AlertCircle, ChevronDown, MoreHorizontal, User, UserPen } from 'lucide-vue-next';
import { useDate } from 'vuetify';

import { FeatureFlags, UserAccount } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { userIsFree, userIsNFR, userIsPaid } from '@/types/user';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/users';
import { useNotify } from '@/composables/useNotify';
import { Size } from '@/utils/bytesSize';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import AccountProjectsTableComponent from '@/components/AccountProjectsTableComponent.vue';
import AccountActionsMenu from '@/components/AccountActionsMenu.vue';
import AccountGeofenceDialog from '@/components/AccountGeofenceDialog.vue';
import CardStatsComponent from '@/components/CardStatsComponent.vue';
import AccountFreezeDialog from '@/components/AccountFreezeDialog.vue';
import AccountUpdateDialog from '@/components/AccountUpdateDialog.vue';
import AccountUpdateLimitsDialog from '@/components/AccountUpdateLimitsDialog.vue';
import AccountDeleteDialog from '@/components/AccountDeleteDialog.vue';
import AccountDisableMFADialog from '@/components/AccountDisableMFADialog.vue';
import AccountCreateRestKeyDialog from '@/components/AccountCreateRestKeyDialog.vue';
import AccountUnfreezeDialog from '@/components/AccountUnfreezeDialog.vue';
import AccountHistoryTableComponent from '@/components/AccountHistoryTableComponent.vue';

const usersStore = useUsersStore();
const appStore = useAppStore();
const router = useRouter();

const notify = useNotify();
const date = useDate();

const freezeDialogEnabled = ref<boolean>(false);
const unfreezeDialogEnabled = ref<boolean>(false);
const updateAccountDialogEnabled = ref<boolean>(false);
const updateLimitsDialogEnabled = ref<boolean>(false);
const deleteAccountDialogEnabled = ref<boolean>(false);
const markPendingDeletion = ref<boolean>(false);
const disableMFADialogEnabled = ref<boolean>(false);
const createRestKeyDialogEnabled = ref<boolean>(false);

const statusColor = computed(() => {
    if (!userAccount.value) {
        return 'default';
    }
    const status = userAccount.value.status.name.toLowerCase();
    if (status.includes('deleted')) {
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
        unfreezeDialogEnabled.value = true;
        return;
    }
    freezeDialogEnabled.value = true;
}

watch(() => router.currentRoute.value.params.userID as string, (userID) => {
    if (!userID || (userAccount.value && userAccount.value.id === userID)) {
        return;
    }
    appStore.load(async () => {
        try {
            await usersStore.updateCurrentUser(userID);
        } catch (error) {
            notify.error(`Failed to get account details. ${error.message}`);
            router.push({ name: ROUTES.Accounts.name });
        }
    });
}, { immediate: true });
</script>

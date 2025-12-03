// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container v-if="project && userAccount">
        <div class="d-flex ga-2 flex-wrap justify-space-between align-center mb-5">
            <div>
                <PageTitleComponent title="Project Details" />
                <div class="d-flex ga-1 flex-wrap justify-start align-center mt-3">
                    <v-btn
                        v-tooltip="'Go to account'"
                        variant="outlined"
                        density="compact"
                        :prepend-icon="ChevronLeft"
                        :text="project.owner.email"
                        color="default"
                        @click="goToAccount"
                    />

                    <v-icon :icon="ChevronRight" />
                    <v-chip
                        v-tooltip="'This project ID'"
                        class="pl-4"
                        color="default"
                        :prepend-icon="Box"
                        @click="copyProjectID"
                    >
                        <span
                            class="font-weight-medium text-truncate"
                            :style="{ maxWidth: smAndDown ? '100px' : '' }"
                        >{{ project.id }} </span>
                    </v-chip>
                </div>
            </div>

            <v-btn>
                Project Actions
                <template #append>
                    <v-icon :icon="ChevronDown" />
                </template>
                <ProjectActionsMenu
                    :project-id="project.id" :owner="project.owner"
                    :active="project.status?.value === UserStatus.Active"
                    @update-limits="onUpdateLimitsClicked"
                    @update="updateDialog = true"
                    @view-entitlements="viewEntitlementsDialog = true"
                    @delete="deleteProjectDialog = true"
                    @mark-pending-deletion="_ => {
                        deleteProjectDialog = true;
                        markPendingDeletion = true;
                    }"
                />
            </v-btn>
        </div>

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
            <v-col cols="12" md="3">
                <v-card :title="project.name" variant="flat" :border="true" rounded="xlg">
                    <template v-if="featureFlags.project.updateInfo" #append>
                        <v-btn
                            :icon="FilePen"
                            variant="outlined" size="small"
                            density="comfortable" color="default"
                            @click="updateDialog = true"
                        />
                    </template>
                    <v-card-text>
                        <v-chip :color="statusColor" variant="tonal" class="mr-2 font-weight-bold">
                            {{ project?.status?.name }}
                        </v-chip>
                        <v-chip
                            :color="userIsPaid(userAccount) ? 'success' : userIsNFR(userAccount) ? 'warning' : 'info'"
                            variant="tonal" class="font-weight-bold"
                        >
                            {{ userAccount.kind.name }}
                        </v-chip>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="3">
                <v-card title="User agent" variant="flat" :border="true" rounded="xlg" class="mb-3">
                    <template v-if="featureFlags.project.updateValueAttribution" #append>
                        <v-btn
                            :icon="FilePen"
                            variant="outlined" size="small"
                            density="comfortable" color="default"
                            @click="updateDialog = true"
                        />
                    </template>
                    <v-card-text>
                        <v-chip color="default" :variant="project.userAgent ? 'tonal' : 'text'" class="mr-2">{{ project.userAgent || 'None' }}</v-chip>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="3">
                <v-card title="Placement" variant="flat" :border="true" rounded="xlg">
                    <template v-if="featureFlags.project.updatePlacement" #append>
                        <v-btn
                            :icon="FilePen"
                            variant="outlined" size="small"
                            density="comfortable" color="default"
                            @click="updateDialog = true"
                        />
                    </template>
                    <v-card-text>
                        <v-chip variant="tonal" class="mr-2">
                            {{ placementText }}
                        </v-chip>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" md="3">
                <v-card title="Entitlements" variant="flat" :border="true" rounded="xlg">
                    <template #append>
                        <v-btn
                            variant="outlined"
                            size="small"
                            density="comfortable"
                            color="default"
                            @click="viewEntitlementsDialog = true"
                        >
                            View
                        </v-btn>
                    </template>
                    <v-card-text>
                        <v-chip variant="tonal" class="mr-2">
                            {{ project.entitlements ? 'Set' : 'Not Set' }}
                        </v-chip>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row>
            <v-col cols="12" sm="6" lg="4">
                <!-- TODO: get bucket count -->
                <UsageProgressComponent
                    title="Buckets" :only-limit="true"
                    :limit="project.maxBuckets"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Storage" :is-bytes="true"
                    :used="project.storageUsed ?? 0"
                    :limit="project.storageLimit ?? 0"
                    :user-specified="project.userSetStorageLimit"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Download" :is-bytes="true"
                    :used="project.bandwidthUsed"
                    :limit="project.bandwidthLimit ?? 0"
                    :user-specified="project.userSetBandwidthLimit"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Segments"
                    :used="project.segmentUsed ?? 0"
                    :limit="project.segmentLimit ?? 0"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Rate" :only-limit="true"
                    :limit="project.rateLimit"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>

            <v-col cols="12" sm="6" lg="4">
                <UsageProgressComponent
                    title="Burst" :only-limit="true"
                    :limit="project.burstLimit"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>
        </v-row>
        <v-row>
            <v-col>
                <UsageProgressComponent
                    title="Head limits"
                    :rate-and-burst="{ rate: project.rateLimitHead, burst: project.burstLimitHead }"
                    :limit="0"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>
            <v-col>
                <UsageProgressComponent
                    title="Get limits"
                    :rate-and-burst="{ rate: project.rateLimitGet, burst: project.burstLimitGet }"
                    :limit="0"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>
            <v-col>
                <UsageProgressComponent
                    title="List limits"
                    :rate-and-burst="{ rate: project.rateLimitList, burst: project.burstLimitList }"
                    :limit="0"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>
            <v-col>
                <UsageProgressComponent
                    title="Put limits"
                    :rate-and-burst="{ rate: project.rateLimitPut, burst: project.burstLimitPut }"
                    :limit="0"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>
            <v-col>
                <UsageProgressComponent
                    title="Delete limits"
                    :rate-and-burst="{ rate: project.rateLimitDelete, burst: project.burstLimitDelete }"
                    :limit="0"
                    @update-limits="onUpdateLimitsClicked"
                />
            </v-col>
        </v-row>

        <v-row>
            <v-col v-if="project && featureFlags.bucket.list">
                <h3 class="my-4">Buckets</h3>
                <BucketsTableComponent :project="project" />
            </v-col>
        </v-row>

        <v-row>
            <v-col v-if="featureFlags.project.memberList">
                <h3 class="my-4">Users</h3>
                <UsersTableComponent />
            </v-col>
        </v-row>

        <v-row v-if="featureFlags.project.history">
            <v-col>
                <ProjectHistoryTableComponent :project />
            </v-col>
        </v-row>
    </v-container>

    <ProjectUpdateLimitsDialog
        v-if="project && featureFlags.project.updateLimits"
        v-model="updateLimitsDialog"
        :project="project"
    />
    <ProjectUpdateDialog
        v-if="project && hasUpdateProjectPerm"
        v-model="updateDialog"
        :project="project"
    />
    <EntitlementsDialog
        v-if="project"
        v-model="viewEntitlementsDialog"
        :project="project"
    />
    <ProjectDeleteDialog
        v-if="project && featureFlags.project.delete"
        v-model="deleteProjectDialog"
        v-model:mark-pending-deletion="markPendingDeletion"
        :project="project"
    />
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue';
import { VAlert, VBtn, VCard, VCardText, VChip, VCol, VContainer, VIcon, VRow } from 'vuetify/components';
import { useRouter } from 'vue-router';
import { Box, ChevronDown, ChevronLeft, ChevronRight, FilePen } from 'lucide-vue-next';
import { useDisplay } from 'vuetify';

import { FeatureFlags, Project, UserAccount } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/users';
import { useNotificationsStore } from '@/store/notifications';
import { userIsNFR, userIsPaid, UserStatus } from '@/types/user';
import { useProjectsStore } from '@/store/projects';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import UsageProgressComponent from '@/components/UsageProgressComponent.vue';
import BucketsTableComponent from '@/components/BucketsTableComponent.vue';
import UsersTableComponent from '@/components/UsersTableComponent.vue';
import ProjectActionsMenu from '@/components/ProjectActionsMenu.vue';
import ProjectUpdateLimitsDialog from '@/components/ProjectUpdateLimitsDialog.vue';
import ProjectUpdateDialog from '@/components/ProjectUpdateDialog.vue';
import EntitlementsDialog from '@/components/EntitlementsDialog.vue';
import ProjectDeleteDialog from '@/components/ProjectDeleteDialog.vue';
import ProjectHistoryTableComponent from '@/components/ProjectHistoryTableComponent.vue';

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();

const { smAndDown } = useDisplay();
const router = useRouter();
const notify = useNotificationsStore();

const updateLimitsDialog = ref<boolean>(false);
const updateDialog = ref<boolean>(false);
const viewEntitlementsDialog = ref<boolean>(false);
const deleteProjectDialog = ref<boolean>(false);
const markPendingDeletion = ref<boolean>(false);

const featureFlags = computed(() => appStore.state.settings.admin.features as FeatureFlags);

const hasUpdateProjectPerm = computed(() => {
    return featureFlags.value.project.updateInfo ||
      featureFlags.value.project.updatePlacement ||
      featureFlags.value.project.updateValueAttribution;
});

const userAccount = computed<UserAccount>(() => usersStore.state.currentAccount as UserAccount);
const project = computed<Project | null>(() => projectsStore.state.currentProject);

const placementText = computed<string>(() => {
    return appStore.getPlacementText(project.value?.defaultPlacement || 0);
});

const statusColor = computed(() => {
    if (!project.value || !project.value.status) {
        return 'default';
    }
    const status = project.value.status.name.toLowerCase();
    if (status.includes('disabled')) {
        return 'error';
    }
    if (status.includes('active')) {
        return 'success';
    }

    return 'warning';
});

/**
 * Returns whether an error occurred retrieving usage data from the Redis live accounting cache.
 */
const usageCacheError = computed<boolean>(() => {
    return !!project.value && (project.value.storageUsed === null || project.value.segmentUsed === null);
});

async function onUpdateLimitsClicked() {
    updateLimitsDialog.value = true;
}

function goToAccount() {
    router.push({ name: ROUTES.Account.name, params: { userID: userAccount.value.id } });
}

function copyProjectID() {
    if (!project.value) return;

    navigator.clipboard.writeText(project.value.id).then(() => {
        notify.notifySuccess('Project ID copied to clipboard');
    }).catch(() => {
        notify.notifyError('Failed to copy Project ID to clipboard');
    });
}

watch(() => router.currentRoute.value.params.projectID as string, (projectID) => {
    if (!projectID) {
        return;
    }
    const userID = router.currentRoute.value.params.userID as string;
    const promises: Promise<void>[] = [];
    if (!userAccount.value || userAccount.value.id !== userID) promises.push(usersStore.updateCurrentUser(userID));
    if (!project.value || project.value.id !== projectID) promises.push(projectsStore.updateCurrentProject(projectID));

    appStore.load(async () => {
        try {
            await Promise.all(promises);
        } catch (error) {
            notify.notifyError('Failed to load project details. ' + error.message);
            router.push({ name: ROUTES.Accounts.name });
        }
    });
}, { immediate: true });

onUnmounted(() => projectsStore.clearCurrentProject());
</script>

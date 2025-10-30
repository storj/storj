// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <announcement-banner />

        <trial-expiration-banner v-if="isTrialExpirationBanner" :expired="isExpired" />

        <minimum-charge-banner v-if="billingEnabled" />

        <card-expire-banner />

        <low-token-balance-banner
            v-if="!isLoading && isLowBalance && billingEnabled"
            cta-label="Go to billing"
            @click="redirectToBilling"
        />
        <PageTitleComponent title="All Projects" />

        <v-row class="mt-0">
            <v-col>
                <v-btn
                    class="mr-3"
                    color="default"
                    variant="outlined"
                    :prepend-icon="CirclePlus"
                    @click="newProjectClicked"
                >
                    New Project
                </v-btn>
            </v-col>

            <template v-if="items.length">
                <v-spacer />

                <v-col class="text-right">
                    <v-btn-toggle
                        mandatory
                        border
                        inset
                        rounded="lg"
                        class="pa-1 bg-surface"
                    >
                        <v-btn
                            rounded="md"
                            active-class="active"
                            :active="!isTableView"
                            aria-label="Toggle Cards View"
                            @click="isTableView = false"
                        >
                            <template #prepend>
                                <component :is="Grid2X2" :size="14" />
                            </template>
                            Cards
                        </v-btn>
                        <v-btn
                            rounded="md"
                            active-class="active"
                            :active="isTableView"
                            aria-label="Toggle Table View"
                            @click="isTableView = true"
                        >
                            <template #prepend>
                                <component :is="Table" :size="15" />
                            </template>
                            Table
                        </v-btn>
                    </v-btn-toggle>
                </v-col>
            </template>
        </v-row>

        <v-row v-if="isTableView" class="mt-1">
            <v-col>
                <ProjectsTableComponent
                    :items="items"
                    @join-click="onJoinClicked"
                    @edit-click="editClick"
                    @update-limits-click="updateLimitsClick"
                    @invite-click="(item) => onInviteClicked(item)"
                />
            </v-col>
        </v-row>

        <v-row v-else>
            <v-col v-if="!items.length" cols="12" sm="6" md="4" lg="3">
                <ProjectCard @create-click="newProjectClicked" />
            </v-col>
            <v-col v-for="item in items" v-else :key="item.id" cols="12" sm="6" md="4" lg="3">
                <ProjectCard
                    :item="item"
                    @join-click="onJoinClicked(item)"
                    @invite-click="onInviteClicked(item)"
                    @edit-click="(field) => editClick(item, field)"
                    @update-limits-click="(limit) => updateLimitsClick(item, limit)"
                />
            </v-col>
        </v-row>
    </v-container>

    <join-project-dialog
        v-if="joiningItem"
        :id="joiningItem.id"
        v-model="isJoinProjectDialogShown"
        :name="joiningItem.name"
    />
    <create-project-dialog v-model="isCreateProjectDialogShown" />
    <add-team-member-dialog v-model="isAddMemberDialogShown" :project-id="addMemberProjectId" />
    <edit-project-details-dialog v-model="isEditProjectDialogShown" :field="fieldToEdit" />
    <edit-project-limit-dialog v-model="isUpdateLimitsDialogShown" :limit-type="limitToChange" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VBtn,
    VSpacer,
    VBtnToggle,
} from 'vuetify/components';
import { useRouter } from 'vue-router';
import { CirclePlus, Grid2X2, Table } from 'lucide-vue-next';

import { FieldToChange, LimitToChange, ProjectItemModel } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ProjectRole } from '@/types/projectMembers';
import { useAppStore } from '@/store/modules/appStore';
import { useLowTokenBalance } from '@/composables/useLowTokenBalance';
import { useConfigStore } from '@/store/modules/configStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { ROUTES } from '@/router';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { usePreCheck } from '@/composables/usePreCheck';
import { AccountBalance, CreditCard } from '@/types/payments';
import { useNotify } from '@/composables/useNotify';

import ProjectCard from '@/components/ProjectCard.vue';
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import ProjectsTableComponent from '@/components/ProjectsTableComponent.vue';
import JoinProjectDialog from '@/components/dialogs/JoinProjectDialog.vue';
import CreateProjectDialog from '@/components/dialogs/CreateProjectDialog.vue';
import AddTeamMemberDialog from '@/components/dialogs/AddTeamMemberDialog.vue';
import LowTokenBalanceBanner from '@/components/LowTokenBalanceBanner.vue';
import TrialExpirationBanner from '@/components/TrialExpirationBanner.vue';
import CardExpireBanner from '@/components/CardExpireBanner.vue';
import EditProjectDetailsDialog from '@/components/dialogs/EditProjectDetailsDialog.vue';
import EditProjectLimitDialog from '@/components/dialogs/EditProjectLimitDialog.vue';
import MinimumChargeBanner from '@/components/MinimumChargeBanner.vue';
import AnnouncementBanner from '@/components/AnnouncementBanner.vue';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();
const billingStore = useBillingStore();
const notify = useNotify();

const router = useRouter();
const isLowBalance = useLowTokenBalance();
const { isTrialExpirationBanner, isExpired, withTrialCheck } = usePreCheck();

const joiningItem = ref<ProjectItemModel | null>(null);
const isJoinProjectDialogShown = ref<boolean>(false);
const isCreateProjectDialogShown = ref<boolean>(false);
const addMemberProjectId = ref<string>('');
const isAddMemberDialogShown = ref<boolean>(false);
const limitToChange = ref(LimitToChange.Storage);
const fieldToEdit = ref(FieldToChange.Name);
const isEditProjectDialogShown = ref(false);
const isUpdateLimitsDialogShown = ref(false);
const isLoading = ref<boolean>(true);

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.getBillingEnabled(usersStore.state.user));

/**
 * Returns whether to use the table view.
 */
const isTableView = computed<boolean>({
    get: () => {
        if (!items.value.length) return false;
        if (!appStore.hasProjectTableViewConfigured() && items.value.length > 8) return true;
        return appStore.state.isProjectTableViewEnabled;
    },
    set: value => appStore.toggleProjectTableViewEnabled(value),
});

/**
 * Returns the project items from the store.
 */
const items = computed((): ProjectItemModel[] => {
    const projects: ProjectItemModel[] = [];

    projects.push(...projectsStore.state.invitations.map<ProjectItemModel>(invite => new ProjectItemModel(
        invite.projectID,
        invite.projectName,
        invite.projectDescription,
        ProjectRole.Invited,
        null,
        invite.createdAt,
    )));

    projects.push(...projectsStore.projects.map<ProjectItemModel>(project => new ProjectItemModel(
        project.id,
        project.name,
        project.description,
        project.ownerId === usersStore.state.user.id ? ProjectRole.Owner : ProjectRole.Member,
        project.memberCount,
        new Date(project.createdAt),
        project.storageUsed,
        project.bandwidthUsed,
        project.encryption,
        project.isClassic,
    )).sort((projA, projB) => {
        if (projA.role === ProjectRole.Owner && projB.role === ProjectRole.Member) return -1;
        if (projA.role === ProjectRole.Member && projB.role === ProjectRole.Owner) return 1;
        return 0;
    }));

    return projects;
});

function newProjectClicked() {
    withTrialCheck(() => {
        analyticsStore.eventTriggered(AnalyticsEvent.NEW_PROJECT_CLICKED);
        isCreateProjectDialogShown.value = true;
    }, true);
}

/**
 * Redirects to Billing Page tab.
 */
function redirectToBilling(): void {
    router.push({ name: ROUTES.Billing.name });
}

/**
 * Displays the Join Project modal.
 */
function onJoinClicked(item: ProjectItemModel): void {
    joiningItem.value = item;
    isJoinProjectDialogShown.value = true;
}

/**
 * Displays the Add Members dialog.
 */
function onInviteClicked(item: ProjectItemModel): void {
    withTrialCheck(() => {
        addMemberProjectId.value = item.id;
        isAddMemberDialogShown.value = true;
    }, true);
}

function editClick(item: ProjectItemModel, field: FieldToChange): void {
    projectsStore.selectProject(item.id);
    fieldToEdit.value = field;
    isEditProjectDialogShown.value = true;
}

async function updateLimitsClick(item: ProjectItemModel, limit: LimitToChange): Promise<void> {
    try {
        projectsStore.selectProject(item.id);
        // await projectsStore.getProjectLimits(item.id);
        isUpdateLimitsDialogShown.value = true;
        limitToChange.value = limit;
    } catch (error) {
        projectsStore.deselectProject();
        notify.notifyError(error, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
    }
}

watch([isEditProjectDialogShown, isUpdateLimitsDialogShown], ([edit, update]) => {
    if (edit || update) return;
    projectsStore.deselectProject();
});

onMounted(async () => {
    if (billingEnabled.value) {
        const promises: Promise<CreditCard[] | AccountBalance | void>[] = [billingStore.getCreditCards()];

        if (configStore.state.config.nativeTokenPaymentsEnabled) {
            promises.push(billingStore.getBalance(), billingStore.getNativePaymentsHistory());
        }
        await Promise.all(promises).catch(_ => {});
    }

    isLoading.value = false;
});
</script>

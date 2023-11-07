// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <low-token-balance-banner
            v-if="isLowBalance && billingEnabled"
            cta-label="Go to billing"
            @click="redirectToBilling"
        />
        <PageTitleComponent title="My Projects" />
        <!-- <PageSubtitleComponent subtitle="Projects are where you and your team can upload and manage data, view usage statistics and billing."/> -->

        <v-row>
            <v-col>
                <v-btn
                    class="mr-3"
                    color="default"
                    variant="outlined"
                    density="comfortable"
                    @click="isCreateProjectDialogShown = true"
                >
                    <svg width="14" height="14" class="mr-2" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M10 1C14.9706 1 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10C1 5.02944 5.02944 1 10 1ZM10 2.65C5.94071 2.65 2.65 5.94071 2.65 10C2.65 14.0593 5.94071 17.35 10 17.35C14.0593 17.35 17.35 14.0593 17.35 10C17.35 5.94071 14.0593 2.65 10 2.65ZM10.7496 6.8989L10.7499 6.91218L10.7499 9.223H12.9926C13.4529 9.223 13.8302 9.58799 13.8456 10.048C13.8602 10.4887 13.5148 10.8579 13.0741 10.8726L13.0608 10.8729L10.7499 10.873L10.75 13.171C10.75 13.6266 10.3806 13.996 9.925 13.996C9.48048 13.996 9.11807 13.6444 9.10066 13.2042L9.1 13.171L9.09985 10.873H6.802C6.34637 10.873 5.977 10.5036 5.977 10.048C5.977 9.60348 6.32857 9.24107 6.76882 9.22366L6.802 9.223H9.09985L9.1 6.98036C9.1 6.5201 9.46499 6.14276 9.925 6.12745C10.3657 6.11279 10.7349 6.45818 10.7496 6.8989Z" fill="currentColor" />
                    </svg>
                    <!-- <IconNew class="mr-2" width="12px"/> -->
                    New Project
                </v-btn>
            </v-col>

            <template v-if="items.length">
                <v-spacer />

                <v-col class="text-right">
                    <!-- Projects Card/Table View -->
                    <v-btn-toggle
                        mandatory
                        border
                        inset
                        density="comfortable"
                        class="pa-1"
                    >
                        <v-btn
                            size="small"
                            rounded="xl"
                            active-class="active"
                            :active="!isTableView"
                            aria-label="Toggle Cards View"
                            @click="isTableView = false"
                        >
                            <icon-card-view />
                            Cards
                        </v-btn>
                        <v-btn
                            size="small"
                            rounded="xl"
                            active-class="active"
                            :active="isTableView"
                            aria-label="Toggle Table View"
                            @click="isTableView = true"
                        >
                            <icon-table-view />
                            Table
                        </v-btn>
                    </v-btn-toggle>
                </v-col>
            </template>
        </v-row>

        <v-row v-if="isLoading" class="justify-center">
            <v-progress-circular indeterminate color="primary" size="48" />
        </v-row>

        <v-row v-else-if="isTableView">
            <!-- Table view -->
            <v-col>
                <ProjectsTableComponent :items="items" @join-click="onJoinClicked" />
            </v-col>
        </v-row>

        <v-row v-else>
            <!-- Card view -->
            <v-col v-if="!items.length" cols="12" sm="6" md="4" lg="3">
                <ProjectCard class="h-100" @create-click="isCreateProjectDialogShown = true" />
            </v-col>
            <v-col v-for="item in items" v-else :key="item.id" cols="12" sm="6" md="4" lg="3">
                <ProjectCard :item="item" class="h-100" @join-click="onJoinClicked(item)" @invite-click="onInviteClicked(item)" />
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
    <account-setup-dialog />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VBtn,
    VSpacer,
    VBtnToggle,
    VProgressCircular,
} from 'vuetify/components';
import { useRouter } from 'vue-router';

import { ProjectItemModel } from '@poc/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ProjectRole } from '@/types/projectMembers';
import { useAppStore } from '@/store/modules/appStore';
import { useLowTokenBalance } from '@/composables/useLowTokenBalance';
import { useConfigStore } from '@/store/modules/configStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { RouteName } from '@poc/router';

import ProjectCard from '@poc/components/ProjectCard.vue';
import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import ProjectsTableComponent from '@poc/components/ProjectsTableComponent.vue';
import JoinProjectDialog from '@poc/components/dialogs/JoinProjectDialog.vue';
import CreateProjectDialog from '@poc/components/dialogs/CreateProjectDialog.vue';
import AddTeamMemberDialog from '@poc/components/dialogs/AddTeamMemberDialog.vue';
import IconCardView from '@poc/components/icons/IconCardView.vue';
import IconTableView from '@poc/components/icons/IconTableView.vue';
import LowTokenBalanceBanner from '@poc/components/LowTokenBalanceBanner.vue';
import AccountSetupDialog from '@poc/components/dialogs/AccountSetupDialog.vue';

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();
const billingStore = useBillingStore();

const router = useRouter();
const isLowBalance = useLowTokenBalance();

const isLoading = ref<boolean>(true);
const joiningItem = ref<ProjectItemModel | null>(null);
const isJoinProjectDialogShown = ref<boolean>(false);
const isCreateProjectDialogShown = ref<boolean>(false);
const addMemberProjectId = ref<string>('');
const isAddMemberDialogShown = ref<boolean>(false);

/**
 * Indicates if billing features are enabled.
 */
const billingEnabled = computed<boolean>(() => configStore.state.config.billingFeaturesEnabled);

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
    )).sort((projA, projB) => {
        if (projA.role === ProjectRole.Owner && projB.role === ProjectRole.Member) return -1;
        if (projA.role === ProjectRole.Member && projB.role === ProjectRole.Owner) return 1;
        return 0;
    }));

    return projects;
});

/**
 * Redirects to Billing Page tab.
 */
function redirectToBilling(): void {
    router.push({ name: RouteName.Billing });
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
    addMemberProjectId.value = item.id;
    isAddMemberDialogShown.value = true;
}

onMounted(async (): Promise<void> => {
    await usersStore.getUser().catch(_ => {});
    await projectsStore.getProjects().catch(_ => {});
    await projectsStore.getUserInvitations().catch(_ => {});

    isLoading.value = false;

    if (configStore.state.config.nativeTokenPaymentsEnabled && configStore.state.config.billingFeaturesEnabled) {
        Promise.all([
            billingStore.getBalance(),
            billingStore.getCreditCards(),
            billingStore.getNativePaymentsHistory(),
        ]).catch(_ => {});
    }
});
</script>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <low-token-balance-banner
            v-if="isLowBalance && billingEnabled"
            cta-label="Go to billing"
            @click="redirectToBilling"
        />
        <PageTitleComponent title="All Projects" />
        <!-- <PageSubtitleComponent subtitle="Projects are where you and your team can upload and manage data, view usage statistics and billing."/> -->

        <v-row>
            <v-col>
                <v-btn
                    class="mr-3"
                    color="default"
                    variant="outlined"
                    density="comfortable"
                    @click="newProjectClicked"
                >
                    <IconNew class="mr-2" size="14" bold />
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

        <v-row v-if="isTableView">
            <!-- Table view -->
            <v-col>
                <ProjectsTableComponent :items="items" @join-click="onJoinClicked" @invite-click="(item) => onInviteClicked(item)" />
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
} from 'vuetify/components';
import { useRouter } from 'vue-router';

import { ProjectItemModel } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ProjectRole } from '@/types/projectMembers';
import { useAppStore } from '@/store/modules/appStore';
import { useLowTokenBalance } from '@/composables/useLowTokenBalance';
import { useConfigStore } from '@/store/modules/configStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { ROUTES } from '@/router';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { Dimensions, Size } from '@/utils/bytesSize';

import ProjectCard from '@/components/ProjectCard.vue';
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import ProjectsTableComponent from '@/components/ProjectsTableComponent.vue';
import JoinProjectDialog from '@/components/dialogs/JoinProjectDialog.vue';
import CreateProjectDialog from '@/components/dialogs/CreateProjectDialog.vue';
import AddTeamMemberDialog from '@/components/dialogs/AddTeamMemberDialog.vue';
import IconCardView from '@/components/icons/IconCardView.vue';
import IconTableView from '@/components/icons/IconTableView.vue';
import IconNew from '@/components/icons/IconNew.vue';
import LowTokenBalanceBanner from '@/components/LowTokenBalanceBanner.vue';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();
const billingStore = useBillingStore();

const router = useRouter();
const isLowBalance = useLowTokenBalance();

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
        formattedValue(new Size(project.storageUsed, 2)),
        formattedValue(new Size(project.bandwidthUsed, 2)),
    )).sort((projA, projB) => {
        if (projA.role === ProjectRole.Owner && projB.role === ProjectRole.Member) return -1;
        if (projA.role === ProjectRole.Member && projB.role === ProjectRole.Owner) return 1;
        return 0;
    }));

    return projects;
});

function newProjectClicked() {
    analyticsStore.eventTriggered(AnalyticsEvent.NEW_PROJECT_CLICKED);
    isCreateProjectDialogShown.value = true;
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
    addMemberProjectId.value = item.id;
    isAddMemberDialogShown.value = true;
}

/**
 * Formats value to needed form and returns it.
 */
function formattedValue(value: Size): string {
    switch (value.label) {
    case Dimensions.Bytes:
        return '0';
    default:
        return `${value.formattedBytes.replace(/\.0+$/, '')}${value.label}`;
    }
}

onMounted(() => {
    if (configStore.state.config.nativeTokenPaymentsEnabled && configStore.state.config.billingFeaturesEnabled) {
        Promise.all([
            billingStore.getBalance(),
            billingStore.getCreditCards(),
            billingStore.getNativePaymentsHistory(),
        ]).catch(_ => {});
    }
});
</script>

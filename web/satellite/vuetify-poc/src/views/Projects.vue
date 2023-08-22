// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
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
                    Create Project
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
                            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <rect x="6.99902" y="6.99951" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
                                <rect x="6.99902" y="13.0005" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
                                <rect x="12.999" y="6.99951" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
                                <rect x="12.999" y="13.0005" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
                            </svg>
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
                            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path fill-rule="evenodd" clip-rule="evenodd" d="M9 8C9 8.55228 8.55228 9 8 9V9C7.44772 9 7 8.55228 7 8V8C7 7.44772 7.44772 7 8 7V7C8.55228 7 9 7.44772 9 8V8Z" fill="currentColor" />
                                <path fill-rule="evenodd" clip-rule="evenodd" d="M9 12C9 12.5523 8.55228 13 8 13V13C7.44772 13 7 12.5523 7 12V12C7 11.4477 7.44772 11 8 11V11C8.55228 11 9 11.4477 9 12V12Z" fill="currentColor" />
                                <path fill-rule="evenodd" clip-rule="evenodd" d="M9 16C9 16.5523 8.55228 17 8 17V17C7.44772 17 7 16.5523 7 16V16C7 15.4477 7.44772 15 8 15V15C8.55228 15 9 15.4477 9 16V16Z" fill="currentColor" />
                                <path fill-rule="evenodd" clip-rule="evenodd" d="M18 8C18 8.55228 17.5523 9 17 9H11C10.4477 9 10 8.55228 10 8V8C10 7.44772 10.4477 7 11 7H17C17.5523 7 18 7.44772 18 8V8Z" fill="currentColor" />
                                <path fill-rule="evenodd" clip-rule="evenodd" d="M18 12C18 12.5523 17.5523 13 17 13H11C10.4477 13 10 12.5523 10 12V12C10 11.4477 10.4477 11 11 11H17C17.5523 11 18 11.4477 18 12V12Z" fill="currentColor" />
                                <path fill-rule="evenodd" clip-rule="evenodd" d="M18 16C18 16.5523 17.5523 17 17 17H11C10.4477 17 10 16.5523 10 16V16C10 15.4477 10.4477 15 11 15H17C17.5523 15 18 15.4477 18 16V16Z" fill="currentColor" />
                            </svg>
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
                <ProjectCard :item="item" class="h-100" @join-click="onJoinClicked(item)" />
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
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VBtn,
    VDialog,
    VCard,
    VSheet,
    VCardItem,
    VCardTitle,
    VDivider,
    VForm,
    VTextField,
    VCardActions,
    VSpacer,
    VBtnToggle,
    VProgressCircular,
} from 'vuetify/components';

import { ProjectItemModel } from '@poc/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ProjectRole } from '@/types/projectMembers';
import { useAppStore } from '@/store/modules/appStore';

import ProjectCard from '@poc/components/ProjectCard.vue';
import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import ProjectsTableComponent from '@poc/components/ProjectsTableComponent.vue';
import JoinProjectDialog from '@poc/components/dialogs/JoinProjectDialog.vue';
import CreateProjectDialog from '@poc/components/dialogs/CreateProjectDialog.vue';

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const usersStore = useUsersStore();

const dialog = ref<boolean>(false);
const valid = ref<boolean>(false);
const name = ref<string>('');
const isLoading = ref<boolean>(true);

const joiningItem = ref<ProjectItemModel | null>(null);
const isJoinProjectDialogShown = ref<boolean>(false);
const isCreateProjectDialogShown = ref<boolean>(false);

const nameRules = [
    value => (!!value || 'Project name is required.'),
    value => ((value?.length <= 100) || 'Name must be less than 100 characters.'),
];

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
 * Displays the Join Project modal.
 */
function onJoinClicked(item: ProjectItemModel): void {
    joiningItem.value = item;
    isJoinProjectDialogShown.value = true;
}

onMounted(async (): Promise<void> => {
    await usersStore.getUser().catch(_ => {});
    await projectsStore.getProjects().catch(_ => {});
    await projectsStore.getUserInvitations().catch(_ => {});

    isLoading.value = false;
});
</script>

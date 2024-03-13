// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-navigation-drawer v-model="model">
        <v-sheet class="px-2 py-1">
            <!-- Project -->
            <v-menu location="end" transition="scale-transition">
                <template #activator="{ props: activatorProps }">
                    <navigation-item title="Project" :subtitle="selectedProject.name" class="pa-4" v-bind="activatorProps">
                        <template #prepend>
                            <IconProject />
                        </template>
                        <template #append>
                            <img src="@/assets/icon-right.svg" class="ml-3" alt="Project" width="10">
                        </template>
                    </navigation-item>
                </template>

                <!-- Project Menu -->
                <v-list class="pa-2">
                    <!-- My Projects -->
                    <template v-if="ownProjects.length">
                        <v-list-item router-link :to="ROUTES.Projects.path" @click="() => registerLinkClick(ROUTES.Projects.path)">
                            <template #prepend>
                                <IconProject />
                            </template>
                            <v-list-item-title class="ml-3">
                                <v-chip color="secondary" variant="tonal" size="small" rounded="xl" class="font-weight-bold" link @click="() => registerLinkClick(ROUTES.Projects.path)">
                                    My Projects
                                </v-chip>
                            </v-list-item-title>
                        </v-list-item>

                        <!-- Selected Project -->
                        <v-list-item
                            v-for="project in ownProjects"
                            :key="project.id"
                            :active="project.isSelected"
                            @click="() => onProjectSelected(project)"
                        >
                            <template v-if="project.isSelected" #prepend>
                                <img src="@/assets/icon-check-color.svg" alt="Selected Project">
                            </template>
                            <v-list-item-title :class="project.isSelected ? 'ml-3' : 'ml-7'">
                                {{ project.name }}
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider class="my-2" />
                    </template>

                    <!-- Shared With Me -->
                    <template v-if="sharedProjects.length">
                        <v-list-item router-link :to="ROUTES.Projects.path" @click="() => registerLinkClick(ROUTES.Projects.path)">
                            <template #prepend>
                                <IconProject />
                            </template>
                            <v-list-item-title class="ml-3">
                                <v-chip color="success" variant="tonal" size="small" rounded="xl" class="font-weight-bold" link @click="() => registerLinkClick(ROUTES.Projects.path)">
                                    Shared Projects
                                </v-chip>
                            </v-list-item-title>
                        </v-list-item>

                        <!-- Other Project -->
                        <v-list-item
                            v-for="project in sharedProjects"
                            :key="project.id"
                            :active="project.isSelected"
                            @click="() => onProjectSelected(project)"
                        >
                            <template v-if="project.isSelected" #prepend>
                                <img src="@/assets/icon-check-color.svg" alt="Selected Project">
                            </template>
                            <v-list-item-title :class="project.isSelected ? 'ml-3' : 'ml-7'">
                                {{ project.name }}
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider class="my-2" />
                    </template>

                    <!-- Project Settings -->
                    <v-list-item router-link :to="settingsURL" @click="() => registerLinkClick(ROUTES.ProjectSettingsAnalyticsLink)">
                        <template #prepend>
                            <IconSettings />
                        </template>
                        <v-list-item-title class="ml-3">
                            Project Settings
                        </v-list-item-title>
                    </v-list-item>

                    <v-divider class="my-2" />

                    <!-- View All Projects -->
                    <v-list-item router-link :to="ROUTES.Projects.path" @click="() => registerLinkClick(ROUTES.Projects.path)">
                        <template #prepend>
                            <IconAllProjects />
                        </template>
                        <v-list-item-title class="ml-3">
                            View All Projects
                        </v-list-item-title>
                    </v-list-item>

                    <v-divider class="my-2" />

                    <!-- Create New Project -->
                    <v-list-item link @click="onCreateProject">
                        <template #prepend>
                            <IconNew size="18" />
                        </template>
                        <v-list-item-title class="ml-3">
                            Create New Project
                        </v-list-item-title>
                    </v-list-item>

                    <v-divider class="my-2" />

                    <!-- Manage Passphrase -->
                    <v-list-item link class="mt-1" @click="isManagePassphraseDialogShown = true">
                        <template #prepend>
                            <IconPassphrase />
                        </template>
                        <v-list-item-title class="ml-3">
                            Manage Passphrase
                        </v-list-item-title>
                    </v-list-item>
                </v-list>
            </v-menu>

            <v-divider class="my-2" />

            <navigation-item :title="ROUTES.Dashboard.name" :to="dashboardURL" @click="() => registerDashboardLinkClick(ROUTES.DashboardAnalyticsLink)">
                <template #prepend>
                    <IconDashboard />
                </template>
            </navigation-item>

            <navigation-item title="Browse" :to="bucketsURL">
                <template #prepend>
                    <IconFolder size="18" />
                </template>
            </navigation-item>

            <navigation-item :title="ROUTES.Access.name" :to="accessURL" @click="() => registerLinkClick(ROUTES.AccessAnalyticsLink)">
                <template #prepend>
                    <IconAccess size="18" />
                </template>
            </navigation-item>

            <navigation-item :title="ROUTES.Applications.name" :to="appsURL" @click="() => registerLinkClick(ROUTES.ApplicationsAnalyticsLink)">
                <template #prepend>
                    <IconApplications />
                </template>
            </navigation-item>

            <navigation-item :title="ROUTES.Team.name" :to="teamURL" @click="() => registerLinkClick(ROUTES.TeamAnalyticsLink)">
                <template #prepend>
                    <IconTeam size="18" />
                </template>
            </navigation-item>

            <v-divider class="my-2" />

            <!-- Resources Menu -->
            <v-menu location="end" transition="scale-transition">
                <template #activator="{ props: activatorProps }">
                    <navigation-item title="Resources" v-bind="activatorProps">
                        <template #prepend>
                            <IconResources />
                        </template>
                        <template #append>
                            <img src="@/assets/icon-right.svg" alt="Resources" width="10">
                        </template>
                    </navigation-item>
                </template>

                <v-list class="pa-2">
                    <v-list-item
                        class="py-3"
                        href="https://docs.storj.io/"
                        target="_blank"
                        rel="noopener noreferrer"
                        @click="() => trackViewDocsEvent('https://docs.storj.io/')"
                    >
                        <template #prepend>
                            <IconDocs />
                        </template>
                        <v-list-item-title class="mx-3">
                            Documentation
                        </v-list-item-title>
                        <v-list-item-subtitle class="mx-3">
                            <small>Go to the Storj docs.</small>
                        </v-list-item-subtitle>
                    </v-list-item>

                    <v-list-item
                        class="py-3"
                        href="https://forum.storj.io/"
                        target="_blank"
                        rel="noopener noreferrer"
                        @click="() => trackViewForumEvent('https://forum.storj.io/')"
                    >
                        <template #prepend>
                            <IconForum />
                        </template>
                        <v-list-item-title class="mx-3">
                            Community Forum
                        </v-list-item-title>
                        <v-list-item-subtitle class="mx-3">
                            <small>Join our global community.</small>
                        </v-list-item-subtitle>
                    </v-list-item>

                    <v-list-item
                        class="py-3"
                        href="https://supportdcs.storj.io/hc/en-us"
                        target="_blank"
                        rel="noopener noreferrer"
                        @click="() => trackViewSupportEvent('https://supportdcs.storj.io/hc/en-us')"
                    >
                        <template #prepend>
                            <IconSupport />
                        </template>
                        <v-list-item-title class="mx-3">
                            Storj Support
                        </v-list-item-title>
                        <v-list-item-subtitle class="mx-3">
                            <small>Need help? Get support.</small>
                        </v-list-item-subtitle>
                    </v-list-item>
                </v-list>
            </v-menu>

            <v-divider class="my-2" />
        </v-sheet>
    </v-navigation-drawer>

    <create-project-dialog v-model="isCreateProjectDialogShown" />
    <manage-passphrase-dialog v-model="isManagePassphraseDialogShown" />
    <enter-project-passphrase-dialog />
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
    VNavigationDrawer,
    VSheet,
    VList,
    VListItem,
    VListItemTitle,
    VListItemSubtitle,
    VMenu,
    VChip,
    VDivider,
} from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { Project } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { ROUTES } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';
import { useTrialCheck } from '@/composables/useTrialCheck';

import IconProject from '@/components/icons/IconProject.vue';
import IconSettings from '@/components/icons/IconSettings.vue';
import IconAllProjects from '@/components/icons/IconAllProjects.vue';
import IconNew from '@/components/icons/IconNew.vue';
import IconPassphrase from '@/components/icons/IconPassphrase.vue';
import IconDashboard from '@/components/icons/IconDashboard.vue';
import IconAccess from '@/components/icons/IconAccess.vue';
import IconTeam from '@/components/icons/IconTeam.vue';
import IconDocs from '@/components/icons/IconDocs.vue';
import IconForum from '@/components/icons/IconForum.vue';
import IconSupport from '@/components/icons/IconSupport.vue';
import IconResources from '@/components/icons/IconResources.vue';
import CreateProjectDialog from '@/components/dialogs/CreateProjectDialog.vue';
import ManagePassphraseDialog from '@/components/dialogs/ManagePassphraseDialog.vue';
import NavigationItem from '@/layouts/default/NavigationItem.vue';
import IconFolder from '@/components/icons/IconFolder.vue';
import IconApplications from '@/components/icons/IconApplications.vue';
import EnterProjectPassphraseDialog
    from '@/components/dialogs/EnterProjectPassphraseDialog.vue';

const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const bucketsStore = useBucketsStore();

const route = useRoute();
const router = useRouter();

const { withTrialCheck } = useTrialCheck();
const { mdAndDown } = useDisplay();

const model = computed<boolean>({
    get: () => appStore.state.isNavigationDrawerShown,
    set: value => appStore.toggleNavigationDrawer(value),
});

const isCreateProjectDialogShown = ref<boolean>(false);
const isManagePassphraseDialogShown = ref<boolean>(false);

/**
 * Returns the selected project from the store.
 */
const selectedProject = computed((): Project => {
    return projectsStore.state.selectedProject;
});

const projectURLBase = computed<string>(() => `${ROUTES.Projects.path}/${selectedProject.value.urlId}`);

const settingsURL = computed<string>(() => `${projectURLBase.value}/${ROUTES.ProjectSettings.path}`);

const accessURL = computed<string>(() => `${projectURLBase.value}/${ROUTES.Access.path}`);

const bucketsURL = computed<string>(() => `${projectURLBase.value}/${ROUTES.Buckets.path}`);

const dashboardURL = computed<string>(() => `${projectURLBase.value}/${ROUTES.Dashboard.path}`);

const teamURL = computed<string>(() => `${projectURLBase.value}/${ROUTES.Team.path}`);

const appsURL = computed<string>(() => `${projectURLBase.value}/${ROUTES.Applications.path}`);

/**
 * Returns user's own projects.
 */
const ownProjects = computed((): Project[] => {
    const projects = projectsStore.projects.filter((p) => p.ownerId === usersStore.state.user.id);
    return projects.sort(compareProjects);
});

/**
 * Returns projects the user is a member of but doesn't own.
 */
const sharedProjects = computed((): Project[] => {
    const projects = projectsStore.projects.filter((p) => p.ownerId !== usersStore.state.user.id);
    return projects.sort(compareProjects);
});

/**
 * Conditionally closes the navigation drawer and tracks page visit.
 */
function registerLinkClick(page: string): void {
    if (mdAndDown.value) {
        model.value = false;
    }
    trackPageVisitEvent(page);
}

/**
 * Sends "Page Visit" event to segment and opens link.
 */
function trackPageVisitEvent(page: string): void {
    analyticsStore.pageVisit(page);
}

function registerDashboardLinkClick(page: string): void {
    registerLinkClick(page);
    analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
}

/**
 * Sends "View Docs" event to segment and opens link.
 */
function trackViewDocsEvent(link: string): void {
    registerLinkClick(link);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open(link);
}

/**
 * Sends "View Forum" event to segment and opens link.
 */
function trackViewForumEvent(link: string): void {
    registerLinkClick(link);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_FORUM_CLICKED);
    window.open(link);
}

/**
 * Sends "View Support" event to segment and opens link.
 */
function trackViewSupportEvent(link: string): void {
    registerLinkClick(link);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_SUPPORT_CLICKED);
    window.open(link);
}

/**
 * Starts create project flow if user's free trial is not expired.
 */
function onCreateProject() {
    withTrialCheck(() => {
        isCreateProjectDialogShown.value = true;
    });
}

/**
 * This comparator is used to sort projects by isSelected.
 */
function compareProjects(a: Project, b: Project): number {
    if (a.isSelected) return -1;
    if (b.isSelected) return 1;
    return 0;
}

/**
 * Handles click event for items in the project dropdown.
 */
async function onProjectSelected(project: Project): Promise<void> {
    analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
    if (route.name === ROUTES.Bucket.name) {
        await router.push({
            name: ROUTES.Buckets.name,
            params: { id: project.urlId },
        });
    } else {
        await router.push({
            name: route.name || ROUTES.Dashboard.name,
            params: {
                ...route.params,
                id: project.urlId,
            },
        });
    }

    bucketsStore.clearS3Data();
}

onBeforeMount(() => {
    if (mdAndDown.value) {
        model.value = false;
    }
});
</script>

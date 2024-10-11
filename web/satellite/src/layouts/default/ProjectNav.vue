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
                            <component :is="Box" :size="18" />
                        </template>
                        <template #append>
                            <img src="@/assets/icon-right.svg" class="ml-4" alt="Project" width="10">
                        </template>
                    </navigation-item>
                </template>

                <!-- Project Menu -->
                <v-list class="pa-2">
                    <!-- My Projects -->
                    <template v-if="ownProjects.length">
                        <v-list-item router-link :to="ROUTES.Projects.path" @click="closeDrawer">
                            <template #prepend>
                                <component :is="Box" :size="18" />
                            </template>
                            <v-list-item-title class="ml-4">
                                <v-chip color="secondary" variant="tonal" size="small" class="font-weight-bold" link @click="closeDrawer">
                                    My Projects
                                </v-chip>
                            </v-list-item-title>
                        </v-list-item>

                        <!-- Selected Project -->
                        <v-list-item
                            v-for="project in ownProjects"
                            :key="project.id"
                            :active="selectedProject.id === project.id"
                            @click="() => onProjectSelected(project)"
                        >
                            <template v-if="selectedProject.id === project.id" #prepend>
                                <img src="@/assets/icon-check-color.svg" alt="Selected Project" width="18" height="18">
                            </template>
                            <v-list-item-title :class="selectedProject.id === project.id ? 'ml-4' : 'ml-9'">
                                {{ project.name }}
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider class="my-2" />
                    </template>

                    <!-- Shared With Me -->
                    <template v-if="sharedProjects.length">
                        <v-list-item router-link :to="ROUTES.Projects.path" @click="closeDrawer">
                            <template #prepend>
                                <component :is="Box" :size="18" />
                            </template>
                            <v-list-item-title class="ml-4">
                                <v-chip color="success" variant="tonal" size="small" class="font-weight-bold" link @click="closeDrawer">
                                    Shared Projects
                                </v-chip>
                            </v-list-item-title>
                        </v-list-item>

                        <!-- Other Project -->
                        <v-list-item
                            v-for="project in sharedProjects"
                            :key="project.id"
                            :active="selectedProject.id === project.id"
                            @click="() => onProjectSelected(project)"
                        >
                            <template v-if="selectedProject.id === project.id" #prepend>
                                <img src="@/assets/icon-check-color.svg" alt="Selected Project">
                            </template>
                            <v-list-item-title :class="selectedProject.id === project.id ? 'ml-4' : 'ml-9'">
                                {{ project.name }}
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider class="my-2" />
                    </template>

                    <!-- Project Settings -->
                    <v-list-item router-link :to="settingsURL" @click="closeDrawer">
                        <template #prepend>
                            <component :is="Settings" :size="18" />
                        </template>
                        <v-list-item-title class="ml-4">
                            Project Settings
                        </v-list-item-title>
                    </v-list-item>

                    <v-divider class="my-2" />

                    <!-- View All Projects -->
                    <v-list-item router-link :to="ROUTES.Projects.path" @click="closeDrawer">
                        <template #prepend>
                            <component :is="Layers" :size="18" />
                        </template>
                        <v-list-item-title class="ml-4">
                            View All Projects
                        </v-list-item-title>
                    </v-list-item>

                    <v-divider class="my-2" />

                    <!-- Create New Project -->
                    <v-list-item link @click="onCreateProject">
                        <template #prepend>
                            <component :is="CirclePlus" :size="18" />
                        </template>
                        <v-list-item-title class="ml-4">
                            Create New Project
                        </v-list-item-title>
                    </v-list-item>

                    <template v-if="!hasManagedPassphrase">
                        <v-divider class="my-2" />

                        <!-- Manage Passphrase -->
                        <v-list-item link class="mt-1" @click="isManagePassphraseDialogShown = true">
                            <template #prepend>
                                <component :is="LockKeyhole" :size="18" />
                            </template>
                            <v-list-item-title class="ml-4">
                                Manage Passphrase
                            </v-list-item-title>
                        </v-list-item>
                    </template>
                </v-list>
            </v-menu>

            <v-divider class="my-2" />

            <navigation-item :title="ROUTES.Dashboard.name" :to="dashboardURL" @click="closeDrawer">
                <template #prepend>
                    <component :is="LayoutDashboard" :size="18" />
                </template>
            </navigation-item>

            <navigation-item title="Browse" :to="bucketsURL">
                <template #prepend>
                    <component :is="FolderOpen" :size="18" />
                </template>
            </navigation-item>

            <navigation-item :title="ROUTES.Access.name" :to="accessURL" @click="closeDrawer">
                <template #prepend>
                    <component :is="KeyRound" :size="18" />
                </template>
            </navigation-item>

            <navigation-item :title="ROUTES.Applications.name" :to="appsURL" @click="closeDrawer">
                <template #prepend>
                    <component :is="AppWindow" :size="18" />
                </template>
            </navigation-item>

            <navigation-item v-if="domainsPageEnabled" :title="ROUTES.Domains.name" :to="domainsURL" @click="closeDrawer">
                <template #prepend>
                    <component :is="Globe" :size="18" />
                </template>
            </navigation-item>

            <navigation-item :title="ROUTES.Team.name" :to="teamURL" @click="closeDrawer">
                <template #prepend>
                    <component :is="Users" :size="18" />
                </template>
            </navigation-item>

            <v-divider class="my-2" />

            <navigation-item v-if="valdiSignUpURL" title="Cloud GPUs" @click="onCloudGPUClicked">
                <template #prepend>
                    <component :is="Microchip" :size="18" />
                </template>
                <template #chip>
                    <v-chip color="success" class="ml-1" size="small">New</v-chip>
                </template>
            </navigation-item>

            <!-- Resources Menu -->
            <v-menu location="end" transition="scale-transition">
                <template #activator="{ props: activatorProps }">
                    <navigation-item title="Resources" v-bind="activatorProps">
                        <template #prepend>
                            <component :is="BookMarked" :size="18" />
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
                            <component :is="BookOpenText" :size="18" />
                        </template>
                        <v-list-item-title class="mx-4">
                            Documentation
                        </v-list-item-title>
                        <v-list-item-subtitle class="mx-4">
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
                            <component :is="MessagesSquare" :size="18" />
                        </template>
                        <v-list-item-title class="mx-4">
                            Community Forum
                        </v-list-item-title>
                        <v-list-item-subtitle class="mx-4">
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
                            <component :is="MessageCircleQuestion" :size="18" />
                        </template>
                        <v-list-item-title class="mx-4">
                            Storj Support
                        </v-list-item-title>
                        <v-list-item-subtitle class="mx-4">
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
    <cloud-gpu-dialog v-if="valdiSignUpURL" v-model="isCloudGpuDialogShown" />
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
import {
    Users,
    AppWindow,
    KeyRound,
    FolderOpen,
    LayoutDashboard,
    BookMarked,
    Box,
    Globe,
    Layers,
    Settings,
    CirclePlus,
    LockKeyhole,
    MessagesSquare,
    MessageCircleQuestion,
    BookOpenText,
    Microchip,
} from 'lucide-vue-next';

import { Project } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsEvent, PageVisitSource } from '@/utils/constants/analyticsEventNames';
import { ROUTES } from '@/router';
import { usePreCheck } from '@/composables/usePreCheck';
import { useConfigStore } from '@/store/modules/configStore';

import CreateProjectDialog from '@/components/dialogs/CreateProjectDialog.vue';
import ManagePassphraseDialog from '@/components/dialogs/ManagePassphraseDialog.vue';
import NavigationItem from '@/layouts/default/NavigationItem.vue';
import EnterProjectPassphraseDialog from '@/components/dialogs/EnterProjectPassphraseDialog.vue';
import CloudGpuDialog from '@/components/dialogs/CloudGpuDialog.vue';

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const configStore = useConfigStore();

const route = useRoute();
const router = useRouter();

const { withTrialCheck } = usePreCheck();
const { mdAndDown } = useDisplay();

const model = computed<boolean>({
    get: () => appStore.state.isNavigationDrawerShown,
    set: value => appStore.toggleNavigationDrawer(value),
});

const isCreateProjectDialogShown = ref<boolean>(false);
const isManagePassphraseDialogShown = ref<boolean>(false);
const isCloudGpuDialogShown = ref<boolean>(false);

const domainsPageEnabled = computed<boolean>(() => configStore.state.config.domainsPageEnabled);

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

const domainsURL = computed<string>(() => `${projectURLBase.value}/${ROUTES.Domains.path}`);

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
 * Returns whether this project has passphrase managed by the satellite.
 */
const hasManagedPassphrase = computed((): boolean => {
    return projectsStore.state.selectedProjectConfig.hasManagedPassphrase;
});

const valdiSignUpURL = computed<string>(() => configStore.state.config.valdiSignUpURL);

/**
 * Conditionally closes the navigation drawer.
 */
function closeDrawer(): void {
    if (mdAndDown.value) {
        model.value = false;
    }
}

function onCloudGPUClicked() {
    closeDrawer();
    analyticsStore.eventTriggered(AnalyticsEvent.CLOUD_GPU_NAVIGATION_ITEM_CLICKED);
    isCloudGpuDialogShown.value = true;
}

/**
 * Sends "View Docs" event to segment and opens link.
 */
function trackViewDocsEvent(link: string): void {
    analyticsStore.pageVisit(link, PageVisitSource.DOCS);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
    window.open(link);
}

/**
 * Sends "View Forum" event to segment and opens link.
 */
function trackViewForumEvent(link: string): void {
    analyticsStore.pageVisit(link, PageVisitSource.FORUM);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_FORUM_CLICKED);
    window.open(link);
}

/**
 * Sends "View Support" event to segment and opens link.
 */
function trackViewSupportEvent(link: string): void {
    analyticsStore.pageVisit(link, PageVisitSource.SUPPORT);
    analyticsStore.eventTriggered(AnalyticsEvent.VIEW_SUPPORT_CLICKED);
    window.open(link);
}

/**
 * Starts create project flow if user's free trial is not expired.
 */
function onCreateProject() {
    withTrialCheck(() => {
        isCreateProjectDialogShown.value = true;
    }, true);
}

/**
 * This comparator is used to sort projects by isSelected.
 */
function compareProjects(a: Project, b: Project): number {
    if (selectedProject.value.id === a.id) return -1;
    if (selectedProject.value.id === b.id) return 1;
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
}

onBeforeMount(() => {
    if (mdAndDown.value) {
        model.value = false;
    }
});
</script>

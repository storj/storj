// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-navigation-drawer v-model="model" class="py-1">
        <v-sheet>
            <v-list class="px-2" color="default" variant="flat">
                <!-- Project -->
                <v-list-item link class="pa-4 rounded-lg">
                    <v-menu activator="parent" location="end" transition="scale-transition">
                        <!-- Project Menu -->
                        <v-list class="pa-2">
                            <!-- My Projects -->
                            <template v-if="ownProjects.length">
                                <v-list-item rounded="lg" link router-link to="/projects" @click="() => registerLinkClick('/projects')">
                                    <template #prepend>
                                        <!-- <img src="@poc/assets/icon-project.svg" alt="Projects"> -->
                                        <IconProject />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        <v-chip color="purple2" variant="tonal" size="small" rounded="xl" class="font-weight-bold" link>
                                            My Projects
                                        </v-chip>
                                    </v-list-item-title>
                                </v-list-item>

                                <!-- Selected Project -->
                                <v-list-item
                                    v-for="project in ownProjects"
                                    :key="project.id"
                                    rounded="lg"
                                    :active="project.isSelected"
                                    @click="() => onProjectSelected(project)"
                                >
                                    <template v-if="project.isSelected" #prepend>
                                        <img src="@poc/assets/icon-check-color.svg" alt="Selected Project">
                                    </template>
                                    <v-list-item-title class="text-body-2" :class="project.isSelected ? 'ml-3' : 'ml-7'">
                                        {{ project.name }}
                                    </v-list-item-title>
                                </v-list-item>

                                <v-divider class="my-2" />
                            </template>

                            <!-- Shared With Me -->
                            <template v-if="sharedProjects.length">
                                <v-list-item rounded="lg" link router-link to="/projects" @click="() => registerLinkClick('/projects')">
                                    <template #prepend>
                                        <IconProject />
                                    </template>
                                    <v-list-item-title class="text-body-2 ml-3">
                                        <v-chip color="green" variant="tonal" size="small" rounded="xl" class="font-weight-bold" link>
                                            Shared Projects
                                        </v-chip>
                                    </v-list-item-title>
                                </v-list-item>

                                <!-- Other Project -->
                                <v-list-item
                                    v-for="project in sharedProjects"
                                    :key="project.id"
                                    rounded="lg"
                                    :active="project.isSelected"
                                    @click="() => onProjectSelected(project)"
                                >
                                    <template v-if="project.isSelected" #prepend>
                                        <img src="@poc/assets/icon-check-color.svg" alt="Selected Project">
                                    </template>
                                    <v-list-item-title class="text-body-2" :class="project.isSelected ? 'ml-3' : 'ml-7'">
                                        {{ project.name }}
                                    </v-list-item-title>
                                </v-list-item>

                                <v-divider class="my-2" />
                            </template>

                            <!-- Project Settings -->
                            <v-list-item link rounded="lg" :to="`/projects/${selectedProject.urlId}/settings`">
                                <template #prepend>
                                    <IconSettings />
                                </template>
                                <v-list-item-title class="text-body-2 ml-3">
                                    Project Settings
                                </v-list-item-title>
                            </v-list-item>

                            <!-- <v-divider class="my-2"></v-divider> -->

                            <!-- View All Projects -->
                            <v-list-item link rounded="lg" router-link to="/projects" @click="() => registerLinkClick('/projects')">
                                <template #prepend>
                                    <IconAllProjects />
                                </template>
                                <v-list-item-title class="text-body-2 ml-3">
                                    View All Projects
                                </v-list-item-title>
                            </v-list-item>

                            <!-- Create New Project -->
                            <v-list-item link rounded="lg" @click="isCreateProjectDialogShown = true">
                                <template #prepend>
                                    <IconNew />
                                </template>
                                <v-list-item-title class="text-body-2 ml-3">
                                    Create New Project
                                </v-list-item-title>
                            </v-list-item>

                            <v-divider class="my-2" />

                            <!-- Manage Passphrase -->
                            <v-list-item link class="mt-1" rounded="lg" @click="isManagePassphraseDialogShown = true">
                                <template #prepend>
                                    <IconPassphrase />
                                </template>
                                <v-list-item-title class="text-body-2 ml-3">
                                    Manage Passphrase
                                </v-list-item-title>
                            </v-list-item>
                        </v-list>
                    </v-menu>
                    <template #prepend>
                        <IconProject />
                    </template>
                    <v-list-item-title link class="text-body-2 ml-3">
                        Project
                    </v-list-item-title>
                    <v-list-item-subtitle class="ml-3">
                        {{ selectedProject.name }}
                    </v-list-item-subtitle>
                    <template #append>
                        <img src="@poc/assets/icon-right.svg" class="ml-3" alt="Project" width="10">
                    </template>
                </v-list-item>

                <v-divider class="my-2" />

                <v-list-item link router-link :to="`/projects/${selectedProject.urlId}/dashboard`" class="my-1 py-3" rounded="lg" @click="() => registerLinkClick('/dashboard')">
                    <template #prepend>
                        <IconDashboard />
                    </template>
                    <v-list-item-title class="text-body-2 ml-3">
                        Overview
                    </v-list-item-title>
                </v-list-item>

                <v-list-item link router-link :to="`/projects/${selectedProject.urlId}/buckets`" class="my-1" rounded="lg" @click="() => registerLinkClick('/buckets')">
                    <template #prepend>
                        <IconBucket />
                    </template>
                    <v-list-item-title class="text-body-2 ml-3">
                        Buckets
                    </v-list-item-title>
                </v-list-item>

                <v-list-item link router-link :to="`/projects/${selectedProject.urlId}/access`" class="my-1" rounded="lg" @click="() => registerLinkClick('/access')">
                    <template #prepend>
                        <IconAccess size="18" />
                    </template>
                    <v-list-item-title class="text-body-2 ml-3">
                        Access
                    </v-list-item-title>
                </v-list-item>

                <v-list-item link router-link :to="`/projects/${selectedProject.urlId}/team`" class="my-1" rounded="lg" @click="() => registerLinkClick('/team')">
                    <template #prepend>
                        <IconTeam size="18" />
                    </template>
                    <v-list-item-title class="text-body-2 ml-3">
                        Team
                    </v-list-item-title>
                </v-list-item>

                <v-divider class="my-2" />

                <!-- Resources Menu -->
                <v-list-item link class="rounded-lg">
                    <v-menu activator="parent" location="end" transition="scale-transition">
                        <v-list class="pa-2">
                            <v-list-item
                                class="py-3"
                                rounded="lg"
                                href="https://docs.storj.io/"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                <template #prepend>
                                    <!-- <img src="@poc/assets/icon-docs.svg" alt="Docs"> -->
                                    <IconDocs />
                                </template>
                                <v-list-item-title class="text-body-2 mx-3">
                                    Documentation
                                </v-list-item-title>
                                <v-list-item-subtitle class="mx-3">
                                    <small>Go to the Storj docs.</small>
                                </v-list-item-subtitle>
                            </v-list-item>

                            <v-list-item
                                class="py-3"
                                rounded="lg"
                                href="https://forum.storj.io/"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                <template #prepend>
                                    <IconForum />
                                </template>
                                <v-list-item-title class="text-body-2 mx-3">
                                    Community Forum
                                </v-list-item-title>
                                <v-list-item-subtitle class="mx-3">
                                    <small>Join our global community.</small>
                                </v-list-item-subtitle>
                            </v-list-item>

                            <v-list-item
                                class="py-3"
                                rounded="lg"
                                href="https://supportdcs.storj.io/hc/en-us"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                <template #prepend>
                                    <IconSupport />
                                </template>
                                <v-list-item-title class="text-body-2 mx-3">
                                    Storj Support
                                </v-list-item-title>
                                <v-list-item-subtitle class="mx-3">
                                    <small>Need help? Get support.</small>
                                </v-list-item-subtitle>
                            </v-list-item>
                        </v-list>
                    </v-menu>

                    <template #prepend>
                        <IconResources />
                    </template>
                    <v-list-item-title class="text-body-2 ml-3">
                        Resources
                    </v-list-item-title>
                    <template #append>
                        <img src="@poc/assets/icon-right.svg" alt="Resources" width="10">
                    </template>
                </v-list-item>

                <v-divider class="my-2" />

                <!-- <v-list-item link class="my-1" router-link to="/design-library" rounded="lg">
                    <template v-slot:prepend>
                        <img src="@poc/assets/icon-bookmark.svg" alt="Design Library" class="mr-3">
                    </template>
                    <v-list-item-title class="text-body-2">
                        Design Library
                    </v-list-item-title>
                </v-list-item> -->
            </v-list>
        </v-sheet>
    </v-navigation-drawer>

    <create-project-dialog v-model="isCreateProjectDialogShown" />
    <manage-passphrase-dialog v-model="isManagePassphraseDialogShown" />
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
import { useAppStore } from '@poc/store/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import IconProject from '@poc/components/icons/IconProject.vue';
import IconSettings from '@poc/components/icons/IconSettings.vue';
import IconAllProjects from '@poc/components/icons/IconAllProjects.vue';
import IconNew from '@poc/components/icons/IconNew.vue';
import IconPassphrase from '@poc/components/icons/IconPassphrase.vue';
import IconDashboard from '@poc/components/icons/IconDashboard.vue';
import IconBucket from '@poc/components/icons/IconBucket.vue';
import IconAccess from '@poc/components/icons/IconAccess.vue';
import IconTeam from '@poc/components/icons/IconTeam.vue';
import IconDocs from '@poc/components/icons/IconDocs.vue';
import IconForum from '@poc/components/icons/IconForum.vue';
import IconSupport from '@poc/components/icons/IconSupport.vue';
import IconResources from '@poc/components/icons/IconResources.vue';
import CreateProjectDialog from '@poc/components/dialogs/CreateProjectDialog.vue';
import ManagePassphraseDialog from '@poc/components/dialogs/ManagePassphraseDialog.vue';

const analyticsStore = useAnalyticsStore();
const projectsStore = useProjectsStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const bucketsStore = useBucketsStore();

const route = useRoute();
const router = useRouter();

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

/*
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
    await router.push({
        name: route.name || undefined,
        params: {
            ...route.params,
            id: project.urlId,
        },
    });
    bucketsStore.clearS3Data();
}

onBeforeMount(() => {
    if (mdAndDown.value) {
        model.value = false;
    }
});
</script>

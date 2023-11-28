// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-navigation-drawer v-model="model">
        <v-sheet class="pa-2">
            <!-- Project -->
            <v-menu location="end" transition="scale-transition">
                <template #activator="{ props: activatorProps }">
                    <navigation-item title="Project" :subtitle="selectedProject.name" class="pa-4" v-bind="activatorProps">
                        <template #prepend>
                            <IconProject />
                        </template>
                        <template #append>
                            <img src="@poc/assets/icon-right.svg" class="ml-3" alt="Project" width="10">
                        </template>
                    </navigation-item>
                </template>

                <!-- Project Menu -->
                <v-list class="pa-2">
                    <!-- My Projects -->
                    <template v-if="ownProjects.length">
                        <v-list-item router-link to="/projects" @click="() => registerLinkClick('/projects')">
                            <template #prepend>
                                <!-- <img src="@poc/assets/icon-project.svg" alt="Projects"> -->
                                <IconProject />
                            </template>
                            <v-list-item-title class="ml-3">
                                <v-chip color="secondary" variant="tonal" size="small" rounded="xl" class="font-weight-bold">
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
                                <img src="@poc/assets/icon-check-color.svg" alt="Selected Project">
                            </template>
                            <v-list-item-title :class="project.isSelected ? 'ml-3' : 'ml-7'">
                                {{ project.name }}
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider class="my-2" />
                    </template>

                    <!-- Shared With Me -->
                    <template v-if="sharedProjects.length">
                        <v-list-item router-link to="/projects" @click="() => registerLinkClick('/projects')">
                            <template #prepend>
                                <IconProject />
                            </template>
                            <v-list-item-title class="ml-3">
                                <v-chip color="green" variant="tonal" size="small" rounded="xl" class="font-weight-bold" link>
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
                                <img src="@poc/assets/icon-check-color.svg" alt="Selected Project">
                            </template>
                            <v-list-item-title :class="project.isSelected ? 'ml-3' : 'ml-7'">
                                {{ project.name }}
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider class="my-2" />
                    </template>

                    <!-- Project Settings -->
                    <v-list-item :to="`/projects/${selectedProject.urlId}/settings`">
                        <template #prepend>
                            <IconSettings />
                        </template>
                        <v-list-item-title class="ml-3">
                            Project Settings
                        </v-list-item-title>
                    </v-list-item>

                    <!-- <v-divider class="my-2"></v-divider> -->

                    <!-- View All Projects -->
                    <v-list-item router-link to="/projects" @click="() => registerLinkClick('/projects')">
                        <template #prepend>
                            <IconAllProjects />
                        </template>
                        <v-list-item-title class="ml-3">
                            View All Projects
                        </v-list-item-title>
                    </v-list-item>

                    <!-- Create New Project -->
                    <v-list-item link @click="isCreateProjectDialogShown = true">
                        <template #prepend>
                            <IconNew />
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

            <!--
            <v-list-item
                router-link
                :to="`/projects/${selectedProject.urlId}/dashboard`"
                class="my-1 py-3"
                tabindex="0"
                @click="() => registerLinkClick('/dashboard')"
            >
                <template #prepend>
                    <IconDashboard />
                </template>
                <v-list-item-title class="ml-3">
                    Overview
                </v-list-item-title>
            </v-list-item>
            -->

            <navigation-item title="Overview" :to="`/projects/${selectedProject.urlId}/dashboard`">
                <template #prepend>
                    <IconDashboard />
                </template>
            </navigation-item>

            <navigation-item title="Buckets" :to="`/projects/${selectedProject.urlId}/buckets`">
                <template #prepend>
                    <IconBucket />
                </template>
            </navigation-item>

            <navigation-item title="Access" :to="`/projects/${selectedProject.urlId}/access`">
                <template #prepend>
                    <IconAccess size="18" />
                </template>
            </navigation-item>

            <navigation-item title="Team" :to="`/projects/${selectedProject.urlId}/team`">
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
                            <img src="@poc/assets/icon-right.svg" alt="Resources" width="10">
                        </template>
                    </navigation-item>
                </template>

                <v-list class="pa-2">
                    <v-list-item
                        class="py-3"
                        href="https://docs.storj.io/"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        <template #prepend>
                            <!-- <img src="@poc/assets/icon-docs.svg" alt="Docs"> -->
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

            <!-- <v-list-item link class="my-1" router-link to="/design-library" rounded="lg">
                <template v-slot:prepend>
                    <img src="@poc/assets/icon-bookmark.svg" alt="Design Library" class="mr-3">
                </template>
                <v-list-item-title class="text-body-2">
                    Design Library
                </v-list-item-title>
            </v-list-item> -->
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
import { RouteName } from '@poc/router';

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
import NavigationItem from '@poc/layouts/default/NavigationItem.vue';

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
    if (route.name === RouteName.Bucket) {
        await router.push({
            name: RouteName.Buckets,
            params: { id: project.urlId },
        });
    } else {
        await router.push({
            name: route.name || undefined,
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

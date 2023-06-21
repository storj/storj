// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="projectSelection" class="project-selection">
        <div
            role="button"
            tabindex="0"
            class="project-selection__selected"
            :class="{ active: isDropdownShown }"
            aria-roledescription="project-selection"
            @keyup.enter="toggleSelection"
            @click.stop.prevent="toggleSelection"
        >
            <div class="project-selection__selected__left">
                <ProjectIcon class="project-selection__selected__left__image" />
                <p class="project-selection__selected__left__name" :title="selectedProject.name">{{ selectedProject.name }}</p>
                <p class="project-selection__selected__left__placeholder">Projects</p>
            </div>
            <ArrowImage class="project-selection__selected__arrow" />
        </div>
        <div v-if="isDropdownShown" v-click-outside="closeDropdown" class="project-selection__dropdown" :style="style">
            <div v-if="isLoading" class="project-selection__dropdown__loader-container">
                <VLoader width="30px" height="30px" />
            </div>
            <div v-else class="project-selection__dropdown__items">
                <div tabindex="0" class="project-selection__dropdown__items__choice" @click.prevent.stop="closeDropdown">
                    <div class="project-selection__dropdown__items__choice__mark-container">
                        <CheckmarkIcon class="project-selection__dropdown__items__choice__mark-container__image" />
                    </div>
                    <p class="project-selection__dropdown__items__choice__selected">
                        {{ selectedProject.name }}
                    </p>
                </div>
                <div
                    v-for="project in projects"
                    :key="project.id"
                    class="project-selection__dropdown__items__choice"
                    @click.prevent.stop="onProjectSelected(project.id)"
                    @keyup.enter="onProjectSelected(project.id)"
                >
                    <p class="project-selection__dropdown__items__choice__unselected">{{ project.name }}</p>
                </div>
            </div>
            <div v-if="isAllProjectsDashboard && isProjectOwner" tabindex="0" class="project-selection__dropdown__link-container" @click.stop="onProjectDetailsClick" @keyup.enter="onProjectDetailsClick">
                <InfoIcon />
                <p class="project-selection__dropdown__link-container__label">Project Details</p>
            </div>
            <div v-if="isAllProjectsDashboard" tabindex="0" class="project-selection__dropdown__link-container" @click.stop="onAllProjectsClick" @keyup.enter="onAllProjectsClick">
                <ProjectIcon />
                <p class="project-selection__dropdown__link-container__label">All projects</p>
            </div>
            <div tabindex="0" class="project-selection__dropdown__link-container" @click.stop="onManagePassphraseClick" @keyup.enter="onManagePassphraseClick">
                <PassphraseIcon />
                <p class="project-selection__dropdown__link-container__label">Manage Passphrase</p>
            </div>
            <div v-if="!isAllProjectsDashboard" tabindex="0" class="project-selection__dropdown__link-container" @click.stop="onProjectsLinkClick" @keyup.enter="onProjectsLinkClick">
                <ManageIcon />
                <p class="project-selection__dropdown__link-container__label">Manage Projects</p>
            </div>
            <div tabindex="0" class="project-selection__dropdown__link-container" @click.stop="onCreateLinkClick" @keyup.enter="onCreateLinkClick">
                <CreateProjectIcon />
                <p class="project-selection__dropdown__link-container__label">Create new project</p>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/types/router';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { LocalData } from '@/utils/localData';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { APP_STATE_DROPDOWNS, MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';

import VLoader from '@/components/common/VLoader.vue';

import ProjectIcon from '@/../static/images/navigation/project.svg';
import ArrowImage from '@/../static/images/navigation/arrowExpandRight.svg';
import CheckmarkIcon from '@/../static/images/navigation/checkmark.svg';
import PassphraseIcon from '@/../static/images/navigation/passphrase.svg';
import ManageIcon from '@/../static/images/navigation/manage.svg';
import CreateProjectIcon from '@/../static/images/navigation/createProject.svg';
import InfoIcon from '@/../static/images/navigation/info.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const pmStore = useProjectMembersStore();
const billingStore = useBillingStore();
const userStore = useUsersStore();
const projectsStore = useProjectsStore();
const configStore = useConfigStore();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

const FIRST_PAGE = 1;
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const dropdownYPos = ref<number>(0);
const dropdownXPos = ref<number>(0);
const isLoading = ref<boolean>(false);
const projectSelection = ref<HTMLDivElement>();

/**
 * Returns top and left position of dropdown.
 */
const style = computed((): Record<string, string> => {
    return { top: `${dropdownYPos.value}px`, left: `${dropdownXPos.value}px` };
});

/**
 * Indicates if current route is onboarding tour.
 */
const isOnboardingTour = computed((): boolean => {
    return route.path.includes(RouteConfig.OnboardingTour.path);
});

/*
 * Whether the user is the owner of the selected project.
 */
const isProjectOwner = computed((): boolean => {
    return userStore.state.user.id === projectsStore.state.selectedProject.ownerId;
});

/**
 * Indicates if all projects dashboard is enabled.
 */
const isAllProjectsDashboard = computed((): boolean => {
    return configStore.state.config.allProjectsDashboard;
});

/**
 * Indicates if dropdown is shown.
 */
const isDropdownShown = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.SELECT_PROJECT;
});

/**
 * Returns projects list from store.
 */
const projects = computed((): Project[] => {
    return projectsStore.projectsWithoutSelected;
});

/**
 * Returns selected project from store.
 */
const selectedProject = computed((): Project => {
    return projectsStore.state.selectedProject;
});

/**
 * Indicates if current route is objects view.
 */
const isBucketsView = computed((): boolean => {
    return route.path.includes(RouteConfig.Buckets.path);
});

/**
 * Fetches projects related information and than toggles selection popup.
 */
async function toggleSelection(): Promise<void> {
    if (isOnboardingTour.value || !projectSelection.value) return;

    const selectionContainer = projectSelection.value.getBoundingClientRect();

    const FIVE_PIXELS = 5;
    const TWENTY_PIXELS = 20;

    dropdownYPos.value = selectionContainer.top - FIVE_PIXELS;
    dropdownXPos.value = selectionContainer.right - TWENTY_PIXELS;

    toggleDropdown();

    if (isLoading.value || !isDropdownShown.value) return;

    isLoading.value = true;

    try {
        await projectsStore.getProjects();
        await projectsStore.getProjectLimits(selectedProject.value.id);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.NAVIGATION_PROJECT_SELECTION);
    } finally {
        isLoading.value = false;
    }
}

/**
 * Toggles project dropdown visibility.
 */
function toggleDropdown(): void {
    appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.SELECT_PROJECT);
}

/**
 * Fetches all project related information.
 * @param projectID
 */
async function onProjectSelected(projectID: string): Promise<void> {
    analytics.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
    projectsStore.selectProject(projectID);
    LocalData.setSelectedProjectId(projectID);
    pmStore.setSearchQuery('');
    closeDropdown();

    bucketsStore.clearS3Data();
    if (userStore.state.settings.passphrasePrompt) {
        appStore.updateActiveModal(MODALS.enterPassphrase);
    }

    if (isBucketsView.value) {
        await router.push(RouteConfig.Buckets.path).catch(() => {return; });

        return;
    }

    if (route.name === RouteConfig.ProjectDashboard.name) {
        const now = new Date();
        const past = new Date();
        past.setDate(past.getDate() - 30);

        try {
            await Promise.all([
                projectsStore.getDailyProjectData({ since: past, before: now }),
                billingStore.getProjectUsageAndChargesCurrentRollup(),
                projectsStore.getProjectLimits(projectID),
                bucketsStore.getBuckets(FIRST_PAGE, projectID),
                agStore.getAccessGrants(FIRST_PAGE, projectID),
                pmStore.getProjectMembers(FIRST_PAGE, projectID),
            ]);
        } catch (error) {
            await notify.error(error.message, AnalyticsErrorEventSource.NAVIGATION_PROJECT_SELECTION);
        }

        return;
    }

    if (route.name === RouteConfig.AccessGrants.name) {
        try {
            await agStore.getAccessGrants(FIRST_PAGE, projectID);
        } catch (error) {
            await notify.error(error.message, AnalyticsErrorEventSource.NAVIGATION_PROJECT_SELECTION);
        }

        return;
    }

    if (route.name === RouteConfig.Team.name) {
        try {
            await pmStore.getProjectMembers(FIRST_PAGE, selectedProject.value.id);
        } catch (error) {
            await notify.error(error.message, AnalyticsErrorEventSource.NAVIGATION_PROJECT_SELECTION);
        }
    }
}

/**
 * Closes select project dropdown.
 */
function closeDropdown(): void {
    appStore.closeDropdowns();
}

/**
 * Route to projects list page.
 */
function onProjectsLinkClick(): void {
    if (route.name !== RouteConfig.ProjectsList.name) {
        analytics.pageVisit(RouteConfig.ProjectsList.path);
        analytics.eventTriggered(AnalyticsEvent.MANAGE_PROJECTS_CLICKED);
        router.push(RouteConfig.ProjectsList.path);
    }

    closeDropdown();
}

/**
 * Route to all projects page.
 */
function onAllProjectsClick(): void {
    analytics.pageVisit(RouteConfig.AllProjectsDashboard.path);
    router.push(RouteConfig.AllProjectsDashboard.path);
    closeDropdown();
}

/**
 * Route to project details page.
 */
function onProjectDetailsClick(): void {
    analytics.pageVisit(RouteConfig.EditProjectDetails.path);
    router.push(RouteConfig.EditProjectDetails.path);
    closeDropdown();
}

/**
 * Toggles manage passphrase modal shown.
 */
function onManagePassphraseClick(): void {
    appStore.updateActiveModal(MODALS.manageProjectPassphrase);

    closeDropdown();
}

/**
 * Route to create project page.
 */
function onCreateLinkClick(): void {
    if (route.name !== RouteConfig.CreateProject.name) {
        analytics.eventTriggered(AnalyticsEvent.CREATE_NEW_CLICKED);

        const user: User = userStore.state.user;
        const ownProjectsCount: number = projectsStore.projectsCount(user.id);

        if (!user.paidTier || user.projectLimit === ownProjectsCount) {
            appStore.updateActiveModal(MODALS.createProjectPrompt);
        } else {
            analytics.pageVisit(RouteConfig.CreateProject.path);
            appStore.updateActiveModal(MODALS.newCreateProject);
        }
    }

    closeDropdown();
}
</script>

<style scoped lang="scss">
    .project-selection {
        font-family: 'font_regular', sans-serif;
        position: static;
        width: 100%;

        &__selected {
            box-sizing: border-box;
            padding: 22px 32px;
            outline: none;
            border: none;
            border-left: 4px solid #fff;
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: space-between;
            cursor: pointer;
            position: static;

            &__left {
                display: flex;
                align-items: center;
                max-width: calc(100% - 16px);
                color: var(--c-grey-6);

                &__name {
                    max-width: calc(100% - 24px - 16px);
                    font-size: 14px;
                    line-height: 20px;
                    margin-left: 24px;
                    white-space: nowrap;
                    overflow: hidden;
                    text-overflow: ellipsis;
                }

                &__placeholder {
                    display: none;
                }
            }

            &:hover {
                background-color: var(--c-grey-1);
                border-color: var(--c-grey-1);

                p {
                    color: var(--c-blue-3);
                }

                :deep(path) {
                    fill: var(--c-blue-3);
                }
            }

            &:focus {
                outline: none;
                border-color: var(--c-grey-1);
                background-color: var(--c-grey-1);
                color: var(--c-blue-3);

                p {
                    color: var(--c-blue-3);
                }

                :deep(path) {
                    fill: var(--c-blue-3);
                }
            }
        }

        &__dropdown {
            position: absolute;
            min-width: 240px;
            max-width: 240px;
            background-color: #fff;
            border: 1px solid var(--c-grey-2);
            box-shadow: 0 2px 16px rgb(0 0 0 / 10%);
            border-radius: 8px;
            z-index: 1;

            &__loader-container {
                margin: 10px 0;
                display: flex;
                align-items: center;
                justify-content: center;
                border-radius: 8px 8px 0 0;
            }

            &__items {
                overflow-y: auto;
                max-height: 250px;
                background-color: #fff;
                border-radius: 6px;

                &__choice {
                    display: flex;
                    align-items: center;
                    padding: 8px 16px;
                    cursor: pointer;
                    height: 32px;
                    border-radius: 8px 8px 0 0;

                    &__selected,
                    &__unselected {
                        font-size: 14px;
                        line-height: 20px;
                        color: #1b2533;
                        white-space: nowrap;
                        overflow: hidden;
                        text-overflow: ellipsis;
                    }

                    &__selected {
                        font-family: 'font_bold', sans-serif;
                        margin-left: 24px;
                    }

                    &__unselected {
                        padding-left: 40px;
                    }

                    &:hover {
                        background-color: #f5f6fa;

                        p {
                            color: var(--c-blue-3);
                        }
                    }

                    &__mark-container {
                        width: 16px;
                        height: 16px;

                        &__image {
                            object-fit: cover;
                        }
                    }

                    &:focus {
                        background-color: #f5f6fa;
                    }
                }
            }

            &__link-container {
                padding: 8px 16px;
                height: 32px;
                cursor: pointer;
                display: flex;
                align-items: center;
                border-top: 1px solid var(--c-grey-2);

                &__label {
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-grey-6);
                    margin-left: 24px;
                }

                &:last-of-type {
                    border-radius: 0 0 8px 8px;
                }

                &:hover {
                    background-color: #f5f6fa;

                    p {
                        color: var(--c-blue-3);
                    }

                    :deep(path) {
                        fill: var(--c-blue-3);
                    }
                }

                &:focus {
                    background-color: #f5f6fa;
                }
            }
        }
    }

    .active {
        border-color: #000;

        p {
            color: var(--c-blue-6);
            font-family: 'font_bold', sans-serif;
        }

        :deep(path) {
            fill: #000;
        }
    }

    .active:hover {
        border-color: var(--c-blue-3);
        background-color: #f7f8fb;

        p {
            color: var(--c-blue-3);
        }

        :deep(path) {
            fill: var(--c-blue-3);
        }
    }

    @media screen and (width <= 1280px) and (width >= 500px) {

        .project-selection__selected {
            padding: 10px 0;
            justify-content: center;

            &__left {
                min-width: 18px;
                flex-direction: column;
                align-items: center;

                &__name {
                    display: none;
                }

                &__placeholder {
                    display: block;
                    margin: 10px 0 0;
                    font-family: 'font_medium', sans-serif;
                    font-size: 9px;
                }
            }

            &__arrow {
                display: none;
            }
        }

        .active p {
            font-family: 'font_medium', sans-serif;
        }
    }
</style>

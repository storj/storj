// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="project.id" class="project-item">
        <div class="project-item__header">
            <project-ownership-tag :project="project" />

            <a
                v-click-outside="closeDropDown" href="" class="project-item__header__menu"
                :class="{open: isDropdownOpen}" @click.stop.prevent="toggleDropDown"
            >
                <menu-icon />
            </a>

            <div v-if="isDropdownOpen" class="project-item__header__dropdown">
                <div v-if="isOwner" class="project-item__header__dropdown__item" @click.stop.prevent="goToProjectEdit">
                    <gear-icon />
                    <p class="project-item__header__dropdown__item__label">Project settings</p>
                </div>

                <div class="project-item__header__dropdown__item" @click.stop.prevent="goToProjectMembers">
                    <users-icon />
                    <p class="project-item__header__dropdown__item__label">Invite members</p>
                </div>
            </div>
        </div>

        <p class="project-item__name">
            {{ project.name }}
        </p>

        <p class="project-item__description">
            {{ project.description }}
        </p>

        <VButton
            class="project-item__button"
            width="fit-content"
            height="fit-content"
            border-radius="8px"
            :on-press="onOpenClicked"
            label="Open Project"
        />
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { Project } from '@/types/projects';
import { useNotify, useRouter } from '@/utils/hooks';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';
import { AnalyticsHttpApi } from '@/api/analytics';
import { LocalData } from '@/utils/localData';
import { RouteConfig } from '@/router';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import VButton from '@/components/common/VButton.vue';
import ProjectOwnershipTag from '@/components/project/ProjectOwnershipTag.vue';

import GearIcon from '@/../static/images/common/gearIcon.svg';
import UsersIcon from '@/../static/images/navigation/users.svg';
import MenuIcon from '@/../static/images/allDashboard/menu.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();

const analytics = new AnalyticsHttpApi();

const props = withDefaults(defineProps<{
    project?: Project,
}>(), {
    project: () => new Project(),
});

/**
 * isDropdownOpen if dropdown is open.
 */
const isDropdownOpen = computed((): boolean => {
    return appStore.state.activeDropdown === props.project.id;
});

/**
 * Returns user entity from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Returns projects list from store.
 */
const isOwner = computed((): boolean => {
    return props.project.ownerId === user.value.id;
});

function toggleDropDown() {
    appStore.toggleActiveDropdown(props.project.id);
}

function closeDropDown() {
    appStore.closeDropdowns();
}

/**
 * Fetches all project related information.
 */
async function onOpenClicked(): Promise<void> {
    await selectProject();
    if (usersStore.shouldOnboard) {
        analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
        await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
        return;
    }
    await analytics.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
    appStore.updateActiveModal(MODALS.enterPassphrase);
}

async function selectProject() {
    projectsStore.selectProject(props.project.id);
    LocalData.setSelectedProjectId(props.project.id);
    pmStore.setSearchQuery('');

    bucketsStore.clearS3Data();
}

/**
 * Navigates to project members page.
 */
async function goToProjectMembers(): Promise<void> {
    await selectProject();
    analytics.pageVisit(RouteConfig.Team.path);
    router.push(RouteConfig.Team.path);
    closeDropDown();
}

/**
 * Fetches all project related information and goes to edit project page.
 */
async function goToProjectEdit(): Promise<void> {
    await selectProject();
    analytics.pageVisit(RouteConfig.EditProjectDetails.path);
    router.push(RouteConfig.EditProjectDetails.path);
    closeDropDown();
}
</script>

<style scoped lang="scss">
.project-item {
    display: grid;
    grid-template-rows: 1fr 1fr 1fr 1fr;
    align-items: start;
    padding: 24px;
    height: 200px;
    background: var(--c-white);
    box-shadow: 0 0 20px rgb(0 0 0 / 5%);
    border-radius: 8px;

    &__header {
        width: 100%;
        display: flex;
        justify-content: space-between;
        align-items: center;
        position: relative;

        &__menu {
            width: 24px;
            height: 24px;
            align-content: center;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 4px;
            position: relative;

            &.open {
                background: var(--c-grey-3);
            }
        }

        &__dropdown {
            position: absolute;
            top: 30px;
            right: 0;
            background: #fff;
            box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
            border: 1px solid var(--c-grey-2);
            border-radius: 8px;
            width: 100%;
            z-index: 100;

            &__item {
                display: flex;
                align-items: center;
                padding: 15px 25px;
                font-family: 'font_regular', sans-serif;
                color: var(--c-grey-6);

                &__label {
                    margin: 0 0 0 10px;
                }

                &:hover {
                    background-color: var(--c-grey-1);
                    font-family: 'font_medium', sans-serif;
                    color: var(--c-blue-3);

                    svg :deep(path) {
                        fill: var(--c-blue-3);
                    }
                }
            }
        }
    }

    &__name {
        font-family: 'font_bold', sans-serif;
        font-size: 24px;
        line-height: 31px;
        width: 100%;
        white-space: nowrap;
        text-overflow: ellipsis;
        overflow: hidden;
        text-align: start;
    }

    &__description {
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        color: var(--c-grey-6);
        line-height: 20px;
        width: 100%;
        white-space: nowrap;
        text-overflow: ellipsis;
        overflow: hidden;
    }

    &__button {
        padding: 10px 16px;
    }
}
</style>

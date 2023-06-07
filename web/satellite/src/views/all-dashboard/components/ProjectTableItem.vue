// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        item-type="project"
        :item="itemToRender"
        :on-click="onOpenClicked"
        class="project-item"
    >
        <template #options>
            <th class="project-item__menu options overflow-visible" @click.stop="toggleDropDown">
                <div class="project-item__menu__icon">
                    <div class="project-item__menu__icon__content" :class="{open: isDropdownOpen}">
                        <menu-icon />
                    </div>
                </div>

                <div v-if="isDropdownOpen" v-click-outside="closeDropDown" class="project-item__menu__dropdown">
                    <div class="project-item__menu__dropdown__item" @click.stop="goToProjectEdit">
                        <gear-icon />
                        <p class="project-item__menu__dropdown__item__label">Project settings</p>
                    </div>

                    <div class="project-item__menu__dropdown__item" @click.stop="goToProjectMembers">
                        <users-icon />
                        <p class="project-item__menu__dropdown__item__label">Invite members</p>
                    </div>
                </div>
            </th>
        </template>
        <menu-icon />
    </table-item>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRouter } from 'vue-router';

import { Project } from '@/types/projects';
import { useNotify } from '@/utils/hooks';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';
import { AnalyticsHttpApi } from '@/api/analytics';
import { LocalData } from '@/utils/localData';
import { RouteConfig } from '@/types/router';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useResize } from '@/composables/resize';

import TableItem from '@/components/common/TableItem.vue';

import UsersIcon from '@/../static/images/navigation/users.svg';
import GearIcon from '@/../static/images/common/gearIcon.svg';
import MenuIcon from '@/../static/images/common/horizontalDots.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();

const analytics = new AnalyticsHttpApi();

const props = defineProps<{
    project: Project,
}>();

const { isMobile } = useResize();

const itemToRender = computed((): { [key: string]: unknown | string[] } => {
    if (!isMobile.value) {
        return {
            multi: { title: props.project.name, subtitle: props.project.description },
            date: props.project.createdDate(),
            memberCount: props.project.memberCount.toString(),
            owner: isOwner.value,
        };
    }

    return { info: [ props.project.name, props.project.description ] };
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
 * Returns if the current user is the owner of this project.
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

    if (usersStore.state.settings.passphrasePrompt) {
        appStore.updateActiveModal(MODALS.enterPassphrase);
    }
    analytics.pageVisit(RouteConfig.ProjectDashboard.path);
    router.push(RouteConfig.ProjectDashboard.path);
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

    &__menu {
        padding: 0 10px;
        position: relative;
        cursor: pointer;

        &__icon {

            &__content {
                height: 32px;
                width: 32px;
                margin-left: auto;
                margin-right: 0;
                padding: 12px 5px;
                border-radius: 5px;
                box-sizing: border-box;
                display: flex;
                align-items: center;
                justify-content: center;

                &.open {
                    background: var(--c-grey-3);
                }
            }
        }

        &__dropdown {
            position: absolute;
            top: 55px;
            right: 10px;
            background: var(--c-white);
            box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
            border: 1px solid var(--c-grey-2);
            border-radius: 8px;
            z-index: 100;
            overflow: hidden;

            &__item {
                display: flex;
                align-items: center;
                width: 200px;
                padding: 15px;
                color: var(--c-grey-6);
                cursor: pointer;

                &__label {
                    font-family: 'font_regular', sans-serif;
                    margin: 0 0 0 10px;
                }

                &:hover {
                    font-family: 'font_medium', sans-serif;
                    color: var(--c-blue-3);
                    background-color: var(--c-grey-1);

                    svg :deep(path) {
                        fill: var(--c-blue-3);
                    }
                }
            }
        }
    }
}
</style>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        :item-type="isOwner ? 'project' : 'shared-project'"
        :item="itemToRender"
        :on-click="onOpenClicked"
        class="project-item"
    >
        <template #options>
            <th class="overflow-visible">
                <div class="options">
                    <v-button
                        :on-press="onOpenClicked"
                        is-white
                        border-radius="8px"
                        font-size="12px"
                        label="Open Project"
                        class="project-item__button"
                    />
                    <v-button
                        :on-press="onOpenClicked"
                        is-white
                        border-radius="8px"
                        font-size="12px"
                        label="Open"
                        class="project-item__mobile-button"
                    />
                    <div class="project-item__menu">
                        <div class="project-item__menu__icon" @click.stop="toggleDropDown">
                            <div class="project-item__menu__icon__content" :class="{open: isDropdownOpen}">
                                <menu-icon />
                            </div>
                        </div>

                        <div v-if="isDropdownOpen" v-click-outside="closeDropDown" class="project-item__menu__dropdown">
                            <div v-if="isOwner" class="project-item__menu__dropdown__item" @click.stop="goToProjectEdit">
                                <gear-icon />
                                <p class="project-item__menu__dropdown__item__label">Project settings</p>
                            </div>

                            <div class="project-item__menu__dropdown__item" @click.stop="goToProjectMembers">
                                <users-icon />
                                <p class="project-item__menu__dropdown__item__label">Invite members</p>
                            </div>
                        </div>
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

import { ProjectRole } from '@/types/projectMembers';
import { Project } from '@/types/projects';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';
import { LocalData } from '@/utils/localData';
import { RouteConfig } from '@/types/router';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useResize } from '@/composables/resize';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import TableItem from '@/components/common/TableItem.vue';
import VButton from '@/components/common/VButton.vue';

import UsersIcon from '@/../static/images/navigation/users.svg';
import GearIcon from '@/../static/images/common/gearIcon.svg';
import MenuIcon from '@/../static/images/common/horizontalDots.svg';

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const router = useRouter();

const props = defineProps<{
    project: Project,
}>();

const { isMobile, screenWidth } = useResize();

const itemToRender = computed((): { [key: string]: unknown | string[] } => {
    if (screenWidth.value <= 600 && !isMobile.value) {
        return {
            multi: { title: props.project.name, subtitle: props.project.description },
        };
    }
    if (screenWidth.value <= 850 && !isMobile.value) {
        return {
            multi: { title: props.project.name, subtitle: props.project.description },
            role: isOwner.value ? ProjectRole.Owner : ProjectRole.Member,
        };
    }
    if (isMobile.value) {
        return { info: [ props.project.name, props.project.description ] };
    }

    return {
        multi: { title: props.project.name, subtitle: props.project.description },
        date: props.project.createdDate(),
        memberCount: props.project.memberCount.toString(),
        role: isOwner.value ? ProjectRole.Owner : ProjectRole.Member,
    };
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
        analyticsStore.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
        await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
        return;
    }
    analyticsStore.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);

    if (usersStore.state.settings.passphrasePrompt) {
        appStore.updateActiveModal(MODALS.enterPassphrase);
    }
    analyticsStore.pageVisit(RouteConfig.ProjectDashboard.path);
    await router.push(RouteConfig.ProjectDashboard.path);
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
    analyticsStore.pageVisit(RouteConfig.Team.path);
    await router.push(RouteConfig.Team.path);
    closeDropDown();
}

/**
 * Fetches all project related information and goes to edit project page.
 */
async function goToProjectEdit(): Promise<void> {
    await selectProject();
    analyticsStore.pageVisit(RouteConfig.EditProjectDetails.path);
    await router.push(RouteConfig.EditProjectDetails.path);
    closeDropDown();
}
</script>

<style scoped lang="scss">
.project-item {

    .options {
        display: flex;
        align-items: center;
        justify-content: flex-end;
        column-gap: 20px;
        padding-right: 10px;

        @media screen and (width <= 900px) {
            column-gap: 10px;
        }
    }

    &__button {
        padding: 10px 16px;
        box-shadow: 0 0 20px 0 rgb(0 0 0 / 4%);

        @media screen and (width <= 900px) {
            display: none;
        }
    }

    &__mobile-button {
        display: none;
        padding: 10px 16px;
        box-shadow: 0 0 20px 0 rgb(0 0 0 / 4%);

        @media screen and (width <= 900px) {
            display: flex;
        }
    }

    &__menu {
        position: relative;
        cursor: pointer;

        &__icon {

            &__content {
                height: 32px;
                width: 32px;
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
            top: 40px;
            right: 0;
            background: #fff;
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

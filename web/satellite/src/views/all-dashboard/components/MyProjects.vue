// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="my-projects">
        <div class="my-projects__header">
            <span class="my-projects__header__title">My Projects</span>

            <VButton
                class="my-projects__header__button"
                icon="addcircle"
                is-white
                :on-press="onCreateProjectClicked"
                label="Create a Project"
            />
        </div>

        <div v-if="projects.length" class="my-projects__list">
            <project-item v-for="project in projects" :key="project.id" :project="project" />
        </div>
        <div v-else class="my-projects__empty-area">
            <empty-project-item class="my-projects__empty-area__item" />
            <rocket-icon class="my-projects__empty-area__icon" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { Project } from '@/types/projects';
import { RouteConfig } from '@/router';
import {
    AnalyticsEvent,
} from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { AnalyticsHttpApi } from '@/api/analytics';
import EmptyProjectItem from '@/views/all-dashboard/components/EmptyProjectItem.vue';
import ProjectItem from '@/views/all-dashboard/components/ProjectItem.vue';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import VButton from '@/components/common/VButton.vue';

import RocketIcon from '@/../static/images/common/rocket.svg';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();

const analytics = new AnalyticsHttpApi();

/**
 * Returns projects list from store.
 */
const projects = computed((): Project[] => {
    return projectsStore.projects;
});

/**
 * Route to create project page.
 */
function onCreateProjectClicked(): void {
    analytics.eventTriggered(AnalyticsEvent.CREATE_NEW_CLICKED);

    const user: User = usersStore.state.user;
    const ownProjectsCount: number = projectsStore.projectsCount(user.id);

    if (!user.paidTier && user.projectLimit === ownProjectsCount) {
        appStore.updateActiveModal(MODALS.createProjectPrompt);
    } else {
        analytics.pageVisit(RouteConfig.CreateProject.path);
        appStore.updateActiveModal(MODALS.newCreateProject);
    }
}
</script>

<style scoped lang="scss">
.my-projects {

    &__header {
        display: flex;
        justify-content: space-between;
        align-items: center;

        @media screen and (width <= 425px) {
            flex-direction: column;
            align-items: start;
            gap: 20px;

            &__button {
                width: 100% !important;
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
        }

        &__button {
            padding: 10px 16px;
            border-radius: 8px;
        }
    }

    &__list {
        margin-top: 20px;
        display: grid;
        gap: 10px;
        grid-template-columns: 1fr 1fr 1fr 1fr;

        & :deep(.project-item) {
            overflow: hidden;
        }

        @media screen and (width <= 1024px) {
            grid-template-columns: 1fr 1fr 1fr;
        }

        @media screen and (width <= 786px) {
            grid-template-columns: 1fr 1fr;
        }

        @media screen and (width <= 425px) {
            grid-template-columns: auto;
        }
    }

    &__empty-area {
        display: flex;
        justify-content: center;
        align-items: center;
        padding-top: 60px;
        position: relative;

        &__item {
            position: absolute;
            top: 30px;
            left: 0;
        }

        @media screen and (width <= 425px) {

            & :deep(.empty-project-item) {
                width: 100%;
                box-sizing: border-box;
            }

            &__icon {
                display: none;
            }
        }
    }
}
</style>

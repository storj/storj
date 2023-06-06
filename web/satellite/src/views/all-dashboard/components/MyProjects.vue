// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="my-projects">
        <div class="my-projects__header">
            <span class="my-projects__header__title">My Projects</span>

            <span class="my-projects__header__right">
                <span class="my-projects__header__right__text">View</span>
                <v-chip
                    label="Table"
                    :is-selected="isTableViewSelected"
                    :icon="TableIcon"
                    @select="() => onViewChangeClicked('table')"
                />

                <v-chip
                    label="Cards"
                    :is-selected="!isTableViewSelected"
                    :icon="CardsIcon"
                    @select="() => onViewChangeClicked('cards')"
                />

                <VButton
                    class="my-projects__header__right__button"
                    icon="addcircle"
                    is-white
                    :on-press="onCreateProjectClicked"
                    label="Create a Project"
                />
            </span>
        </div>
        <div v-if="projects.length || invites.length" class="my-projects__list">
            <projects-table v-if="isTableViewSelected" :invites="invites" class="my-projects__list__table" />
            <div v-else-if="!isTableViewSelected" class="my-projects__list__cards">
                <project-item v-for="project in projects" :key="project.id" :project="project" />
                <project-invitation-item v-for="invite in invites" :key="invite.projectID" :invitation="invite" />
            </div>
        </div>
        <div v-else class="my-projects__empty-area">
            <empty-project-item class="my-projects__empty-area__item" />
            <rocket-icon class="my-projects__empty-area__icon" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { Project, ProjectInvitation } from '@/types/projects';
import { RouteConfig } from '@/types/router';
import {
    AnalyticsEvent,
} from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { AnalyticsHttpApi } from '@/api/analytics';
import EmptyProjectItem from '@/views/all-dashboard/components/EmptyProjectItem.vue';
import ProjectItem from '@/views/all-dashboard/components/ProjectItem.vue';
import ProjectInvitationItem from '@/views/all-dashboard/components/ProjectInvitationItem.vue';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import ProjectsTable from '@/views/all-dashboard/components/ProjectsTable.vue';

import VButton from '@/components/common/VButton.vue';
import VChip from '@/components/common/VChip.vue';

import RocketIcon from '@/../static/images/common/rocket.svg';
import CardsIcon from '@/../static/images/common/cardsIcon.svg';
import TableIcon from '@/../static/images/common/tableIcon.svg';

const appStore = useAppStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();

const analytics = new AnalyticsHttpApi();

const hasProjectTableViewConfigured = ref(appStore.hasProjectTableViewConfigured());

/**
 * Whether to use the table view.
 */
const isTableViewSelected = computed((): boolean => {
    if (!hasProjectTableViewConfigured.value && projects.value.length > 1) {
        // show the table by default if the user has more than 8 projects.
        return true;
    }
    return appStore.state.isProjectTableViewEnabled;
});

/**
 * Returns projects list from store.
 */
const projects = computed((): Project[] => {
    return projectsStore.projects;
});

/**
 * Returns project member invitations list from store.
 */
const invites = computed((): ProjectInvitation[] => {
    return projectsStore.state.invitations.slice()
        .sort((a, b) => b.createdAt.getTime() - a.createdAt.getTime());
});

function onViewChangeClicked(view: string): void {
    appStore.toggleProjectTableViewEnabled(view === 'table');
    hasProjectTableViewConfigured.value = true;
}

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

        &__right {
            font-family: 'font_regular', sans-serif;
            display: flex;
            align-items: center;
            justify-content: flex-end;
            column-gap: 12px;

            &__text {
                font-size: 12px;
                line-height: 18px;
                color: var(--c-grey-6);
            }

            &__button {
                padding: 10px 16px;
                border-radius: 8px;
            }
        }
    }

    &__list {
        margin-top: 20px;

        &__cards {
            display: grid;
            gap: 10px;
            grid-template-columns: repeat(4, minmax(0, 1fr));

            & :deep(.project-item) {
                overflow: hidden;
            }

            @media screen and (width <= 1024px) {
                grid-template-columns: repeat(3, minmax(0, 1fr));
            }

            @media screen and (width <= 786px) {
                grid-template-columns: repeat(2, minmax(0, 1fr));
            }

            @media screen and (width <= 425px) {
                grid-template-columns: auto;
            }
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

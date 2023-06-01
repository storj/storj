// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="empty-project-item">
        <div class="empty-project-item__header">
            <div class="empty-project-item__header__tag">
                <box-icon class="empty-project-item__header__tag__icon" />

                <span> Project </span>
            </div>
        </div>

        <p class="empty-project-item__title">
            Welcome
        </p>

        <p class="empty-project-item__subtitle">
            Create a new project to start.
        </p>

        <VButton
            class="empty-project-item__button"
            icon="addcircle"
            :on-press="onCreateProjectClicked"
            label="Create a Project"
        />
    </div>
</template>

<script setup lang="ts">
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';
import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import VButton from '@/components/common/VButton.vue';

import BoxIcon from '@/../static/images/navigation/project.svg';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const analytics = new AnalyticsHttpApi();

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
.empty-project-item {
    padding: 24px;
    background: var(--c-white);
    box-shadow: 0 0 20px rgb(0 0 0 / 4%);
    border-radius: 8px;

    &__header {
        width: 100%;
        display: flex;
        justify-content: space-between;
        align-items: center;
        position: relative;

        &__tag {
            display: flex;
            justify-content: space-between;
            align-items: center;
            gap: 5px;
            padding: 4px 8px;
            border: 1px solid var(--c-light-blue-4);
            border-radius: 24px;
            color: var(--c-blue-4);
            font-size: 12px;
            font-family: 'font_regular', sans-serif;

            &__icon {
                width: 12px;
                height: 12px;

                :deep(path) {
                    fill: var(--c-blue-4);
                }
            }
        }
    }

    &__title {
        margin-top: 16px;
        font-family: 'font_bold', sans-serif;
        font-size: 24px;
        line-height: 31px;
        width: 100%;
        white-space: nowrap;
        text-overflow: ellipsis;
        overflow: hidden;
        text-align: start;
    }

    &__subtitle {
        font-weight: 400;
        font-size: 14px;
        margin-top: 5px;
        font-family: 'font_regular', sans-serif;
        color: var(--c-grey-6);
        line-height: 20px;
    }

    &__button {
        margin-top: 20px;
        padding: 10px 16px;
        border-radius: 8px;

        & :deep(.label svg path) {
            fill: var(--c-white);
        }
    }
}
</style>

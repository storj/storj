// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { User } from '@/types/users';
import { RouteConfig } from '@/types/router';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useAppStore } from '@/store/modules/appStore';

export function useCreateProjectClickHandler() {
    const analyticsStore = useAnalyticsStore();
    const userStore = useUsersStore();
    const projectsStore = useProjectsStore();
    const appStore = useAppStore();

    function handleCreateProjectClick(): void {
        analyticsStore.eventTriggered(AnalyticsEvent.CREATE_NEW_CLICKED);

        const user: User = userStore.state.user;
        const ownProjectsCount: number = projectsStore.projectsCount(user.id);

        if (user.projectLimit > ownProjectsCount) {
            analyticsStore.pageVisit(RouteConfig.CreateProject.path);
            appStore.updateActiveModal(MODALS.newCreateProject);
        } else {
            appStore.updateActiveModal(MODALS.createProjectPrompt);
        }
    }

    return {
        handleCreateProjectClick,
    };
}

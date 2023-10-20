// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="content" class="my-projects">
        <div class="my-projects__header">
            <div class="my-projects__header__title">
                <span>My Projects</span>
                <span class="my-projects__header__title__views">
                    <span
                        class="my-projects__header__title__views__icon"
                        @click="() => onViewChangeClicked('table')"
                    >
                        <table-icon />
                    </span>
                    <span
                        class="my-projects__header__title__views__icon"
                        @click="() => onViewChangeClicked('cards')"
                    >
                        <cards-icon />
                    </span>
                </span>
            </div>

            <span class="my-projects__header__right">
                <span class="my-projects__header__right__text">View</span>
                <v-chip
                    class="my-projects__header__right__table-chip"
                    label="Table"
                    :is-selected="isTableViewSelected"
                    :icon="TableIcon"
                    @select="() => onViewChangeClicked('table')"
                />

                <v-chip
                    class="my-projects__header__right__card-chip"
                    label="Cards"
                    :is-selected="!isTableViewSelected"
                    :icon="CardsIcon"
                    @select="() => onViewChangeClicked('cards')"
                />

                <VButton
                    class="my-projects__header__right__mobile-button"
                    icon="addcircle"
                    border-radius="8px"
                    font-size="12px"
                    is-white
                    :on-press="handleCreateProjectClick"
                    label="Create New Project"
                />

                <VButton
                    class="my-projects__header__right__button"
                    icon="addcircle"
                    border-radius="8px"
                    font-size="12px"
                    is-white
                    :on-press="handleCreateProjectClick"
                    label="Create a Project"
                />
            </span>
        </div>

        <all-projects-dashboard-banners v-if="content" :parent-ref="content" />

        <div
            v-if="projects.length || invites.length" class="my-projects__list"
            :style="{'padding': isTableViewSelected && isMobile ? '0' : '0 20px'}"
        >
            <projects-table v-if="isTableViewSelected" :invites="invites" class="my-projects__list__table" />
            <div v-else-if="!isTableViewSelected" class="my-projects__list__cards">
                <project-invitation-item v-for="invite in invites" :key="invite.projectID" :invitation="invite" />
                <project-item v-for="project in projects" :key="project.id" :project="project" />
            </div>
        </div>
        <div v-else class="my-projects__empty-area">
            <empty-project-item class="my-projects__empty-area__item" />
            <rocket-icon class="my-projects__empty-area__icon" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { Project, ProjectInvitation } from '@/types/projects';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useResize } from '@/composables/resize';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/utils/hooks';
import { useCreateProjectClickHandler } from '@/composables/useCreateProjectClickHandler';

import EmptyProjectItem from '@/views/all-dashboard/components/EmptyProjectItem.vue';
import ProjectItem from '@/views/all-dashboard/components/ProjectItem.vue';
import ProjectInvitationItem from '@/views/all-dashboard/components/ProjectInvitationItem.vue';
import ProjectsTable from '@/views/all-dashboard/components/ProjectsTable.vue';
import AllProjectsDashboardBanners from '@/views/all-dashboard/components/AllProjectsDashboardBanners.vue';
import VButton from '@/components/common/VButton.vue';
import VChip from '@/components/common/VChip.vue';

import RocketIcon from '@/../static/images/common/rocket.svg';
import CardsIcon from '@/../static/images/common/cardsIcon.svg';
import TableIcon from '@/../static/images/common/tableIcon.svg';

const billingStore = useBillingStore();
const appStore = useAppStore();
const configStore = useConfigStore();
const projectsStore = useProjectsStore();

const { handleCreateProjectClick } = useCreateProjectClickHandler();
const notify = useNotify();
const { isMobile } = useResize();

const content = ref<HTMLElement | null>(null);

const hasProjectTableViewConfigured = ref(appStore.hasProjectTableViewConfigured());

/**
 * Whether to use the table view.
 */
const isTableViewSelected = computed((): boolean => {
    if (!hasProjectTableViewConfigured.value && projects.value.length > 8) {
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

onMounted(async () => {
    if (!configStore.state.config.nativeTokenPaymentsEnabled) {
        return;
    }

    try {
        await Promise.all([
            billingStore.getBalance(),
            billingStore.getCreditCards(),
            billingStore.getNativePaymentsHistory(),
        ]);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
    }
});
</script>

<style scoped lang="scss">
.my-projects {

    &__header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0 20px;

        @media screen and (width <= 500px) {
            margin-top: 20px;
            flex-direction: column;
            align-items: start;
            gap: 20px;

            &__title {
                display: flex;
                align-items: center;
                justify-content: space-between;
                width: 100%;

                &__views {
                    display: flex !important;
                    align-items: center;
                    justify-content: flex-end;
                    column-gap: 5px;

                    &__icon {
                        height: 24px;
                        width: 24px;

                        :deep(path),
                        :deep(rect){
                            fill: var(--c-black);
                        }
                    }
                }
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-weight: 700;
            font-size: 24px;
            line-height: 31px;

            @media screen and (width <= 500px) {
                font-size: 18px;
                line-height: 27px;
            }

            &__views {
                display: none;
            }
        }

        &__right {
            font-family: 'font_regular', sans-serif;
            display: flex;
            align-items: center;
            justify-content: flex-end;
            column-gap: 12px;

            @media screen and (width <= 500px) {
                width: 100%;

                &__text,
                &__button,
                &__table-chip,
                &__card-chip {
                    display: none;
                }

                &__mobile-button {
                    display: flex !important;
                }
            }

            &__text {
                font-size: 12px;
                line-height: 18px;
                color: var(--c-grey-6);

            }

            &__button,
            &__mobile-button {
                padding: 10px 16px;
                box-shadow: 0 0 20px rgb(0 0 0 / 4%);

                :deep(.label) {
                    color: var(--c-black) !important;
                    font-weight: 700;
                    line-height: 20px;
                }
            }

            &__mobile-button {
                display: none;
            }
        }
    }

    & :deep(.all-dashboard-banners) {
        padding: 0 20px;
    }

    &__list {
        margin-top: 20px;

        &__cards {
            display: grid;
            gap: 10px;
            grid-template-columns: repeat(4, minmax(0, 1fr));

            & :deep(.project-item), &:deep(.invite-item) {
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

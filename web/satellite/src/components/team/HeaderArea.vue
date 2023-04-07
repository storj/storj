// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-header-container">
        <div class="team-header-container__title-area">
            <h1 class="team-header-container__title-area__title" aria-roledescription="title">Project Members</h1>
            <VInfo class="team-header-container__title-area__info-button">
                <template #icon>
                    <InfoIcon />
                </template>
                <template #message>
                    <p class="team-header-container__title-area__info-button__message">
                        The only project role currently available is Admin, which gives full access to the project.
                    </p>
                </template>
            </VInfo>
        </div>
        <div class="team-header-container__wrapper">
            <VHeader
                ref="headerComponent"
                placeholder="Team Members"
                :search="processSearchQuery"
            >
                <div v-if="isDefaultState" class="header-default-state">
                    <VButton
                        class="button"
                        label="+ Add"
                        width="122px"
                        height="48px"
                        :on-press="toggleTeamMembersModal"
                        :is-disabled="isAddButtonDisabled"
                    />
                </div>
                <div v-if="areProjectMembersSelected" class="header-selected-members">
                    <VButton
                        class="button deletion"
                        label="Delete"
                        width="122px"
                        height="48px"
                        :on-press="onFirstDeleteClick"
                    />
                    <VButton
                        class="button"
                        label="Cancel"
                        width="122px"
                        height="48px"
                        :is-transparent="true"
                        :on-press="onClearSelection"
                    />
                    <span class="header-selected-members__info-text"><b>{{ selectedProjectMembersCount }}</b> users selected</span>
                </div>
                <div v-if="areSelectedProjectMembersBeingDeleted" class="header-after-delete-click">
                    <span class="header-after-delete-click__delete-confirmation">Are you sure you want to delete <b>{{ selectedProjectMembersCount }}</b> {{ userCountTitle }}?</span>
                    <div class="header-after-delete-click__button-area">
                        <VButton
                            class="button deletion"
                            label="Delete"
                            width="122px"
                            height="48px"
                            :on-press="onDelete"
                        />
                        <VButton
                            class="button"
                            label="Cancel"
                            width="122px"
                            height="48px"
                            :is-transparent="true"
                            :on-press="onClearSelection"
                        />
                    </div>
                </div>
            </VHeader>
            <div v-if="isDeleteClicked" class="blur-content" />
            <div v-if="isDeleteClicked" class="blur-search" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue';

import { RouteConfig } from '@/router';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectMemberHeaderState } from '@/types/projectMembers';
import { Project } from '@/types/projects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';

import VInfo from '@/components/common/VInfo.vue';
import VHeader from '@/components/common/VHeader.vue';
import VButton from '@/components/common/VButton.vue';

import InfoIcon from '@/../static/images/team/infoTooltip.svg';

declare interface ClearSearch {
    clearSearch(): void;
}

const pmStore = useProjectMembersStore();
const store = useStore();
const notify = useNotify();
const router = useRouter();

const props = withDefaults(defineProps<{
    headerState: ProjectMemberHeaderState;
    selectedProjectMembersCount: number;
    isAddButtonDisabled: boolean;
}>(), {
    headerState: ProjectMemberHeaderState.DEFAULT,
    selectedProjectMembersCount: 0,
    isAddButtonDisabled: false,
});

const emit = defineEmits(['onSuccessAction']);

const FIRST_PAGE = 1;
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isDeleteClicked = ref<boolean>(false);
const headerComponent = ref<VHeader & ClearSearch>();

const isDefaultState = computed((): boolean => {
    return props.headerState === 0;
});

const areProjectMembersSelected = computed((): boolean => {
    return props.headerState === 1 && !isDeleteClicked.value;
});

const areSelectedProjectMembersBeingDeleted = computed((): boolean => {
    return props.headerState === 1 && isDeleteClicked.value;
});

const userCountTitle = computed((): string => {
    return props.selectedProjectMembersCount === 1 ? 'user' : 'users';
});

/**
 * Opens add team members modal.
 */
function toggleTeamMembersModal(): void {
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.addTeamMember);
}

function onFirstDeleteClick(): void {
    isDeleteClicked.value = true;
}

/**
 * Clears selection and returns area state to default.
 */
function onClearSelection(): void {
    pmStore.clearProjectMemberSelection();
    isDeleteClicked.value = false;

    emit('onSuccessAction');
}

/**
 * Removes user from selected project.
 */
async function onDelete(): Promise<void> {
    try {
        await pmStore.deleteProjectMembers(store.getters.selectedProject.id);
        await setProjectState();
    } catch (error) {
        await notify.error(`Error while deleting users from projectMembers. ${error.message}`, AnalyticsErrorEventSource.PROJECT_MEMBERS_HEADER);
        isDeleteClicked.value = false;
        return;
    }

    emit('onSuccessAction');
    await notify.success('Members were successfully removed from project');
    isDeleteClicked.value = false;
}

/**
 * Fetches team members of current project depends on search query.
 * @param search
 */
async function processSearchQuery(search: string): Promise<void> {
    pmStore.setSearchQuery(search);
    try {
        await pmStore.getProjectMembers(FIRST_PAGE, store.getters.selectedProject.id);
    } catch (error) {
        await notify.error(`Unable to fetch project members. ${error.message}`, AnalyticsErrorEventSource.PROJECT_MEMBERS_HEADER);
    }
}

async function setProjectState(): Promise<void> {
    const projects: Project[] = await store.dispatch(PROJECTS_ACTIONS.FETCH);
    if (!projects.length) {
        const onboardingPath = RouteConfig.OnboardingTour.with(RouteConfig.FirstOnboardingStep).path;

        analytics.pageVisit(onboardingPath);
        await router.push(onboardingPath);

        return;
    }

    if (!projects.includes(store.getters.selectedProject)) {
        await store.dispatch(PROJECTS_ACTIONS.SELECT, projects[0].id);
    }

    await pmStore.getProjectMembers(FIRST_PAGE, store.getters.selectedProject.id);
    headerComponent.value?.clearSearch();
}

/**
 * Lifecycle hook before component destruction.
 * Clears selection and search query for team members page.
 */
onBeforeUnmount((): void => {
    onClearSelection();
    pmStore.setSearchQuery('');
});
</script>

<style scoped lang="scss">
    .team-header-container {

        &__title-area {
            display: flex;
            align-items: center;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                color: #263549;
                margin: 0;
            }

            &__info-button {
                max-height: 20px;
                cursor: pointer;
                margin-left: 10px;

                &:hover {

                    .team-header-svg-path {
                        fill: #fff;
                    }

                    .team-header-svg-rect {
                        fill: #2683ff;
                    }
                }

                &__message {
                    color: #586c86;
                    font-family: 'font_regular', sans-serif;
                    font-size: 16px;
                    line-height: 18px;
                }
            }
        }
    }

    .header-default-state,
    .header-after-delete-click {
        display: flex;
        flex-direction: column;
        justify-content: center;
        height: 85px;

        &__info-text {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 28px;
        }

        &__delete-confirmation {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            line-height: 28px;
        }

        &__button-area {
            display: flex;

            .deletion {
                margin-top: 2px;
            }
        }
    }

    .header-selected-members {
        display: flex;
        align-items: center;
        height: 85px;
        justify-content: center;

        &__info-text {
            margin-left: 25px;
            line-height: 48px;
        }
    }

    .button {
        margin-right: 12px;
    }

    .team-header-container__wrapper {
        margin-bottom: 4px;
        display: flex;
        align-items: center;
        justify-content: flex-start;
        position: relative;

        .blur-content {
            position: absolute;
            top: 100%;
            left: 0;
            background-color: #f5f6fa;
            width: 100%;
            height: 70vh;
            z-index: 100;
            opacity: 0.3;
        }

        .blur-search {
            position: absolute;
            bottom: 0;
            right: 0;
            width: 540px;
            height: 56px;
            z-index: 100;
            opacity: 0.3;
            background-color: #f5f6fa;
        }
    }

    .container.deletion {
        background-color: #ff4f4d;

        &.label {
            color: #fff;
        }

        &:hover {
            background-color: #de3e3d;
            box-shadow: none;
        }
    }

    :deep(.info__box__message) {
        min-width: 300px;
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-header-container">
        <div class="team-header-container__title-area">
            <div class="team-header-container__title-area__titles">
                <span class="team-header-container__title-area__titles__title" aria-roledescription="title">Team</span>
                <VInfo class="team-header-container__title-area__titles__info-button">
                    <template #icon>
                        <InfoIcon />
                    </template>
                    <template #message>
                        <p class="team-header-container__title-area__info-button__message">
                            The only project role currently available is Admin, which gives full access to the project.
                        </p>
                    </template>
                </VInfo>
                <p class="team-header-container__title-area__titles__subtitle" aria-roledescription="subtitle">
                    Manage the team members of "{{ projectName }}"
                </p>
            </div>
            <VButton
                class="team-header-container__title-area__button"
                label="Add Members"
                width="160px"
                height="40px"
                font-size="13px"
                border-radius="8px"
                :on-press="toggleAddTeamMembersModal"
                icon="add"
                :is-disabled="isAddButtonDisabled"
            />
        </div>

        <div class="team-header-container__divider" />

        <div class="team-header-container__wrapper">
            <VSearchAlternateStyling
                ref="searchInput"
                class="team-header-container__wrapper__search"
                placeholder="members"
                :search="processSearchQuery"
            />
            <div>
                <div v-if="areProjectMembersSelected" class="header-selected-members">
                    <VButton
                        class="button deletion"
                        label="Delete"
                        width="122px"
                        height="40px"
                        :on-press="toggleRemoveTeamMembersModal"
                    />
                    <VButton
                        class="button"
                        label="Cancel"
                        width="122px"
                        height="40px"
                        :is-transparent="true"
                        :on-press="onClearSelection"
                    />
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue';

import { RouteConfig } from '@/router';
import { ProjectMemberHeaderState } from '@/types/projectMembers';
import { Project } from '@/types/projects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify, useRouter } from '@/utils/hooks';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';

import VInfo from '@/components/common/VInfo.vue';
import VButton from '@/components/common/VButton.vue';
import VSearchAlternateStyling from '@/components/common/VSearchAlternateStyling.vue';

import InfoIcon from '@/../static/images/team/infoTooltip.svg';

interface ClearSearch {
    clearSearch(): void;
}

const configStore = useConfigStore();
const appStore = useAppStore();
const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();
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

const FIRST_PAGE = 1;
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const isDeleteClicked = ref<boolean>(false);
const searchInput = ref<typeof VSearchAlternateStyling & ClearSearch>();

/**
 * Returns the name of the selected project from store.
 */
const projectName = computed((): string => {
    return projectsStore.state.selectedProject.name;
});

const isDefaultState = computed((): boolean => {
    return props.headerState === 0;
});

const areProjectMembersSelected = computed((): boolean => {
    return props.headerState === 1 && !isDeleteClicked.value;
});

const userCountTitle = computed((): string => {
    return props.selectedProjectMembersCount === 1 ? 'user' : 'users';
});

/**
 * Opens add team members modal.
 */
function toggleAddTeamMembersModal(): void {
    appStore.updateActiveModal(MODALS.addTeamMember);
}

function toggleRemoveTeamMembersModal(): void {
    appStore.updateActiveModal(MODALS.removeTeamMember);
}

/**
 * Clears selection and returns area state to default.
 */
function onClearSelection(): void {
    pmStore.clearProjectMemberSelection();
    isDeleteClicked.value = false;
}

/**
 * Fetches team members of current project depends on search query.
 * @param search
 */
async function processSearchQuery(search: string): Promise<void> {
    // avoid infinite loop due to listener on pmStore.setSearchQuery('') itself indirectly calling pmStore.setSearchQuery('')
    if (pmStore.getSearchQuery() !== search) {
        pmStore.setSearchQuery(search);
    }
    try {
        await pmStore.getProjectMembers(FIRST_PAGE, projectsStore.state.selectedProject.id);
    } catch (error) {
        await notify.error(`Unable to fetch project members. ${error.message}`, AnalyticsErrorEventSource.PROJECT_MEMBERS_HEADER);
    }
}

/**
 * Lifecycle hook after initial render.
 * Set up listener to clear search bar.
 */
onMounted((): void => {
    if (configStore.state.config.allProjectsDashboard && !projectsStore.state.selectedProject.id) {
        // navigation back to the all projects dashboard is done in ProjectMembersArea.vue
        return;
    }

    pmStore.$onAction(({ name, after, args }) => {
        if (name === 'setSearchQuery' && args[0] === '') {
            after((_) => searchInput.value?.clearSearch());
        }
    });
});

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
            width: 100%;
            display: flex;
            justify-content: space-between;
            align-items: center;

            @media screen and (max-width: 1150px) {
                flex-direction: column;
                align-items: flex-start;
                justify-content: flex-start;
                row-gap: 10px;

                &__button {
                    width: 100% !important;
                }
            }

            &__titles {

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-weight: 600;
                    font-size: 28px;
                    line-height: 34px;
                    color: #232b34;
                    text-align: left;
                    display: inline;
                }

                &__subtitle {
                    font-size: 14px;
                    line-height: 20px;
                    font-weight: bold;
                    margin-top: 10px;
                }

                &__info-button {
                    max-height: 20px;
                    cursor: pointer;
                    margin-left: 10px;
                    display: inline;

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

        &__divider {
            width: 100%;
            height: 1px;
            background: #dadfe7;
            margin: 24px 0;
        }
    }

    .header-default-state,
    .header-after-delete-click {
        display: flex;
        flex-direction: column;
        justify-content: center;

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
        }
    }

    .header-selected-members {
        display: flex;
        align-items: center;
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
        position: relative;
        margin-bottom: 20px;
        display: flex;
        align-items: center;
        justify-content: space-between;

        @media screen and (max-width: 1150px) {
            flex-direction: column;
            align-items: flex-start;
            justify-content: flex-start;
            row-gap: 10px;
        }

        &__search {
            position: static;
        }

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
            left: 0;
            width: 300px;
            height: 40px;
            z-index: 100;
            opacity: 0.3;
            background-color: #f5f6fa;

            @media screen and (max-width: 1150px) {
                bottom: unset;
                right: 0;
                width: unset;
            }
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

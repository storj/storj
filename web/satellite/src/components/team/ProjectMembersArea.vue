// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-area">
        <div class="team-area__header">
            <HeaderArea
                :header-state="headerState"
                :selected-project-members-count="selectedProjectMembersLength"
                :is-add-button-disabled="areMembersFetching"
            />
        </div>
        <VLoader v-if="areMembersFetching" width="100px" height="100px" />

        <div v-if="isEmptySearchResultShown" class="team-area__empty-search-result-area">
            <h1 class="team-area__empty-search-result-area__title">No results found</h1>
            <EmptySearchResultIcon class="team-area__empty-search-result-area__image" />
        </div>

        <v-table
            v-if="!areMembersFetching && !isEmptySearchResultShown"
            class="team-area__table"
            items-label="project members"
            :selectable="true"
            :limit="projectMemberLimit"
            :total-page-count="totalPageCount"
            :total-items-count="projectMembersTotalCount"
            :on-page-change="onPageChange"
        >
            <template #head>
                <th class="align-left">Name</th>
                <th class="align-left date-added">Date Added</th>
                <th class="align-left">Email</th>
            </template>
            <template #body>
                <ProjectMemberListItem
                    v-for="(member, key) in projectMembers"
                    :key="key"
                    :item-data="member"
                    @memberClick="onMemberCheckChange"
                    @selectClicked="(_) => onMemberCheckChange(member)"
                />
            </template>
        </v-table>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import {
    ProjectMember,
    ProjectMemberHeaderState,
} from '@/types/projectMembers';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import VLoader from '@/components/common/VLoader.vue';
import HeaderArea from '@/components/team/HeaderArea.vue';
import ProjectMemberListItem from '@/components/team/ProjectMemberListItem.vue';
import VTable from '@/components/common/VTable.vue';

import EmptySearchResultIcon from '@/../static/images/common/emptySearchResult.svg';

const pmStore = useProjectMembersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const FIRST_PAGE = 1;

const areMembersFetching = ref<boolean>(true);

/**
 * Returns team members of current page from store.
 * With project owner pinned to top
 */
const projectMembers = computed((): ProjectMember[] => {
    const projectMembers = pmStore.state.page.projectMembers;
    const projectOwner = projectMembers.find((member) => member.user.id === projectsStore.state.selectedProject.ownerId);
    const projectMembersToReturn = projectMembers.filter((member) => member.user.id !== projectsStore.state.selectedProject.ownerId);

    // if the project owner exists, place at the front of the members list
    projectOwner && projectMembersToReturn.unshift(projectOwner);

    return projectMembersToReturn;
});

/**
 * Returns team members total page count from store.
 */
const projectMembersTotalCount = computed((): number => {
    return pmStore.state.page.totalCount;
});

/**
 * Returns team members limit from store.
 */
const projectMemberLimit = computed((): number => {
    return pmStore.state.page.limit;
});

/**
 * Returns team members count of current page from store.
 */
const projectMembersCount = computed((): number => {
    return pmStore.state.page.projectMembers.length;
});

const totalPageCount = computed((): number => {
    return pmStore.state.page.pageCount;
});

const selectedProjectMembersLength = computed((): number => {
    return pmStore.state.selectedProjectMembersEmails.length;
});

const headerState = computed((): number => {
    return selectedProjectMembersLength.value > 0 ? ProjectMemberHeaderState.ON_SELECT : ProjectMemberHeaderState.DEFAULT;
});

const isEmptySearchResultShown = computed((): boolean => {
    return projectMembersCount.value === 0 && projectMembersTotalCount.value === 0;
});

/**
 * Selects team member if this user has no owner status.
 * @param member
 */
function onMemberCheckChange(member: ProjectMember): void {
    if (projectsStore.state.selectedProject.ownerId !== member.user.id) {
        pmStore.toggleProjectMemberSelection(member);
    }
}

/**
 * Fetches team member of selected page.
 * @param index
 * @param limit
 */
async function onPageChange(index: number, limit: number): Promise<void> {
    try {
        await pmStore.getProjectMembers(index, projectsStore.state.selectedProject.id, limit);
    } catch (error) {
        notify.error(`Unable to fetch project members. ${error.message}`, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
    }
}

/**
 * Lifecycle hook after initial render.
 * Fetches first page of team members list of current project.
 */
onMounted(async (): Promise<void> => {
    try {
        await pmStore.getProjectMembers(FIRST_PAGE, projectsStore.state.selectedProject.id);

        areMembersFetching.value = false;
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.PROJECT_MEMBERS_PAGE);
    }
});
</script>

<style scoped lang="scss">
    .team-area {
        padding: 40px 30px 55px;
        height: calc(100% - 95px);
        font-family: 'font_regular', sans-serif;

        &__header {
            width: 100%;
            background-color: #f5f6fa;
            top: auto;
        }

        &__empty-search-result-area {
            height: 100%;
            display: flex;
            align-items: center;
            justify-content: center;
            flex-direction: column;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                margin-top: 100px;
            }

            &__image {
                margin-top: 40px;
            }
        }
    }

    @media screen and (max-width: 800px) and (min-width: 500px) {

        .date-added {
            display: none;
        }
    }
</style>

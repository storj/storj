// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-area">
        <div class="team-area__header">
            <HeaderArea
                :header-state="headerState"
                :selected-project-members-count="selectedProjectMembersLength"
                :is-add-button-disabled="areMembersFetching"
                @onSuccessAction="resetPaginator"
            />
        </div>
        <VLoader v-if="areMembersFetching" width="100px" height="100px" />
        <template v-else>
            <div v-if="isTeamAreaShown" id="team-container" class="team-area__container">
                <SortingListHeader :on-header-click-callback="onHeaderSectionClickCallback" />
                <div class="team-area__container__content">
                    <VList
                        :data-set="projectMembers"
                        :item-component="getItemComponent"
                        :on-item-click="onMemberClick"
                    />
                </div>
            </div>
            <VPagination
                v-if="totalPageCount > 1"
                ref="pagination"
                class="pagination-area"
                :total-page-count="totalPageCount"
                :on-page-click-callback="onPageClick"
            />
            <div v-if="isEmptySearchResultShown" class="team-area__empty-search-result-area">
                <h1 class="team-area__empty-search-result-area__title">No results found</h1>
                <EmptySearchResultIcon class="team-area__empty-search-result-area__image" />
            </div>
        </template>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VList from '@/components/common/VList.vue';
import VLoader from '@/components/common/VLoader.vue';
import VPagination from '@/components/common/VPagination.vue';
import HeaderArea from '@/components/team/HeaderArea.vue';
import ProjectMemberListItem from '@/components/team/ProjectMemberListItem.vue';
import SortingListHeader from '@/components/team/SortingListHeader.vue';

import EmptySearchResultIcon from '@/../static/images/common/emptySearchResult.svg';

import { SortDirection } from '@/types/common';
import { ProjectMember, ProjectMemberHeaderState, ProjectMemberOrderBy } from '@/types/projectMembers';
import { PM_ACTIONS } from '@/utils/constants/actionNames';

const {
    FETCH,
    TOGGLE_SELECTION,
    SET_SORT_BY,
    SET_SORT_DIRECTION,
} = PM_ACTIONS;

declare interface ResetPagination {
    resetPageIndex(): void;
}

// @vue/component
@Component({
    components: {
        HeaderArea,
        VList,
        VPagination,
        SortingListHeader,
        EmptySearchResultIcon,
        VLoader,
    },
})
export default class ProjectMembersArea extends Vue {
    private FIRST_PAGE = 1;

    public areMembersFetching = true;

    public $refs!: {
        pagination: HTMLElement & ResetPagination;
    };

    /**
     * Lifecycle hook after initial render.
     * Fetches first page of team members list of current project.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);

            this.areMembersFetching = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Selects team member if this user has no owner status.
     * @param member
     */
    public onMemberClick(member: ProjectMember): void {
        if (this.$store.getters.selectedProject.ownerId !== member.user.id) {
            this.$store.dispatch(TOGGLE_SELECTION, member);
        }
    }

    /**
     * Returns team members of current page from store.
     * With project owner pinned to top
     */
    public get projectMembers(): ProjectMember[] {
        const projectMembers = this.$store.state.projectMembersModule.page.projectMembers;
        const projectOwner = projectMembers.find((member) => member.user.id === this.$store.getters.selectedProject.ownerId);
        const projectMembersToReturn = projectMembers.filter((member) => member.user.id !== this.$store.getters.selectedProject.ownerId);

        // if the project owner exists, place at the front of the members list
        projectOwner && projectMembersToReturn.unshift(projectOwner);

        return projectMembersToReturn;
    }

    public get getItemComponent(): typeof ProjectMemberListItem {
        return ProjectMemberListItem;
    }

    /**
     * Returns team members total page count from store.
     */
    public get projectMembersTotalCount(): number {
        return this.$store.state.projectMembersModule.page.totalCount;
    }

    /**
     * Returns team members count of current page from store.
     */
    public get projectMembersCount(): number {
        return this.$store.state.projectMembersModule.page.projectMembers.length;
    }

    public get totalPageCount(): number {
        return this.$store.state.projectMembersModule.page.pageCount;
    }

    public get selectedProjectMembersLength(): number {
        return this.$store.state.projectMembersModule.selectedProjectMembersEmails.length;
    }

    public get headerState(): number {
        return this.selectedProjectMembersLength > 0 ? ProjectMemberHeaderState.ON_SELECT : ProjectMemberHeaderState.DEFAULT;
    }

    public get isTeamAreaShown(): boolean {
        return this.projectMembersCount > 0 || this.projectMembersTotalCount > 0;
    }

    public get isEmptySearchResultShown(): boolean {
        return this.projectMembersCount === 0 && this.projectMembersTotalCount === 0;
    }

    /**
     * Fetches team member of selected page.
     * @param index
     */
    public async onPageClick(index: number): Promise<void> {
        try {
            await this.$store.dispatch(FETCH, index);
        } catch (error) {
            this.$notify.error(`Unable to fetch project members. ${error.message}`);
        }
    }

    /**
     * Changes sorting parameters and fetches team members.
     * @param sortBy
     * @param sortDirection
     */
    public async onHeaderSectionClickCallback(sortBy: ProjectMemberOrderBy, sortDirection: SortDirection): Promise<void> {
        await this.$store.dispatch(SET_SORT_BY, sortBy);
        await this.$store.dispatch(SET_SORT_DIRECTION, sortDirection);
        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project members. ${error.message}`);
        }

        this.resetPaginator();
    }

    public resetPaginator(): void {
        if (this.totalPageCount > 1) {
            this.$refs.pagination.resetPageIndex();
        }
    }
}
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

        &__container {

            &__content {
                display: flex;
                justify-content: space-between;
                margin-bottom: 20px;
                flex-direction: column;
            }
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

    .pagination-area {
        margin-left: -25px;
        padding-bottom: 15px;
    }
</style>

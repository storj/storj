// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-header-container">
        <div class="team-header-container__title-area">
            <h1 class="team-header-container__title-area__title">Project Members</h1>
            <VInfo
                class="team-header-container__title-area__info-button"
                bold-text="The only project role currently available is Admin, which gives full access to the project.">
                <InfoIcon class="team-header-container__title-area__info-button__image"/>
            </VInfo>
        </div>
	    <div class="team-header-container__wrapper">
            <VHeader
                ref="headerComponent"
                placeholder="Team Members"
                :search="processSearchQuery">
                <div class="header-default-state" v-if="isDefaultState">
                    <VButton
                        class="button"
                        label="+ Add"
                        width="122px"
                        height="48px"
                        :on-press="onAddUsersClick"
                    />
                </div>
                <div class="header-selected-members" v-if="areProjectMembersSelected">
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
                        is-white="true"
                        :on-press="onClearSelection"
                    />
                    <span class="header-selected-members__info-text"><b>{{selectedProjectMembersCount}}</b> users selected</span>
                </div>
                <div class="header-after-delete-click" v-if="areSelectedProjectMembersBeingDeleted">
                    <span class="header-after-delete-click__delete-confirmation">Are you sure you want to delete <b>{{selectedProjectMembersCount}}</b> {{userCountTitle}}?</span>
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
                            is-white="true"
                            :on-press="onClearSelection"
                        />
                    </div>
                </div>
            </VHeader>
            <div class="blur-content" v-if="isDeleteClicked"></div>
            <div class="blur-search" v-if="isDeleteClicked"></div>
	    </div>
        <AddUserPopup v-if="isAddTeamMembersPopupShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';
import VHeader from '@/components/common/VHeader.vue';
import VInfo from '@/components/common/VInfo.vue';
import AddUserPopup from '@/components/team/AddUserPopup.vue';

import InfoIcon from '@/../static/images/team/infoTooltip.svg';

import { RouteConfig } from '@/router';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectMemberHeaderState } from '@/types/projectMembers';
import { APP_STATE_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';

declare interface ClearSearch {
    clearSearch(): void;
}

@Component({
    components: {
        VButton,
        VHeader,
        AddUserPopup,
        VInfo,
        InfoIcon,
    },
})
export default class HeaderArea extends Vue {
    @Prop({default: ProjectMemberHeaderState.DEFAULT})
    private readonly headerState: ProjectMemberHeaderState;
    @Prop({default: 0})
    public readonly selectedProjectMembersCount: number;

    private FIRST_PAGE = 1;

    /**
     * Indicates if state after first delete click is active.
     */
    public isDeleteClicked: boolean = false;

    public $refs!: {
        headerComponent: VHeader & ClearSearch;
    };

    /**
     * Lifecycle hook before component destruction.
     * Clears selection and search query for team members page.
     */
    public beforeDestroy(): void {
        this.onClearSelection();
        this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');
    }

    public get userCountTitle(): string {
        return this.selectedProjectMembersCount === 1 ? 'user' : 'users';
    }

    /**
     * Opens add team members popup.
     */
    public onAddUsersClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);
    }

    public onFirstDeleteClick(): void {
        this.isDeleteClicked = true;
    }

    /**
     * Clears selection and returns area state to default.
     */
    public onClearSelection(): void {
        this.$store.dispatch(PM_ACTIONS.CLEAR_SELECTION);
        this.isDeleteClicked = false;

        this.$emit('onSuccessAction');
    }

    /**
     * Removes user from selected project.
     */
    public async onDelete(): Promise<void> {
        try {
            await this.$store.dispatch(PM_ACTIONS.DELETE);
            await this.setProjectState();
        } catch (error) {
            await this.$notify.error(`Error while deleting users from projectMembers. ${error.message}`);
            this.isDeleteClicked = false;

            return;
        }

        this.$emit('onSuccessAction');
        await this.$notify.success('Members were successfully removed from project');
        this.isDeleteClicked = false;
    }

    /**
     * Fetches team members of current project depends on search query.
     * @param search
     */
    public async processSearchQuery(search: string): Promise<void> {
        await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, search);
        try {
            await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project members. ${error.message}`);
        }
    }

    /**
     * Indicates if add team member popup should be rendered.
     */
    public get isAddTeamMembersPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isAddTeamMembersPopupShown;
    }

    public get isDefaultState(): boolean {
        return this.headerState === 0;
    }

    public get areProjectMembersSelected(): boolean {
        return this.headerState === 1 && !this.isDeleteClicked;
    }

    public get areSelectedProjectMembersBeingDeleted(): boolean {
        return this.headerState === 1 && this.isDeleteClicked;
    }

    private async setProjectState(): Promise<void> {
        const projects = await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
        if (!projects.length) {
            await this.$router.push(RouteConfig.OnboardingTour.path);

            return;
        }

        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projects[0].id);
        await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
        this.$refs.headerComponent.clearSearch();
    }
}
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

    /deep/ .info__message-box {
        background-image: url('../../../static/images/team/MessageBox.png');
        background-repeat: no-repeat;
        min-height: 80px;
        min-width: 200px;
        width: 200px;
        top: 110%;
        left: -205%;
        padding: 0 20px 12px 20px;
        word-break: break-word;

        &__text {
            text-align: left;
            font-size: 13px;
            line-height: 17px;
            margin-top: 20px;

            &__bold-text {
                font-family: 'font_medium', sans-serif;
                color: #354049;
            }
        }
    }
</style>

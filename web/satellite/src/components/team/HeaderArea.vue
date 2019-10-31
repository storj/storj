// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-header-container">
	    <h1 class="team-header-container__title">Project Members</h1>
	    <div class="team-header-container__wrapper">
            <VHeader
                ref="headerComponent"
                placeholder="Team Members"
                :search="processSearchQuery">
                <div class="header-default-state" v-if="isDefaultState">
                    <span class="header-default-state__info-text">The only project role currently available is Admin, which gives <b>full access</b> to the project.</span>
                    <VButton
                        class="button"
                        label="+Add"
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
import AddUserPopup from '@/components/team/AddUserPopup.vue';

import { ProjectMemberHeaderState } from '@/types/projectMembers';
import { APP_STATE_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';

declare interface ClearSearch {
    clearSearch(): void;
}

@Component({
    components: {
        VButton,
        VHeader,
        AddUserPopup,
    },
})
export default class HeaderArea extends Vue {
    @Prop({default: ProjectMemberHeaderState.DEFAULT})
    private readonly headerState: ProjectMemberHeaderState;
    @Prop({default: 0})
    public readonly selectedProjectMembersCount: number;

    private FIRST_PAGE = 1;

    public isDeleteClicked: boolean = false;

    public $refs!: {
        headerComponent: VHeader & ClearSearch;
    };

    public beforeDestroy(): void {
        this.onClearSelection();
    }

    public get userCountTitle(): string {
        if (this.selectedProjectMembersCount === 1) {
            return 'user';
        }

        return 'users';
    }

    public onAddUsersClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);
    }

    public onFirstDeleteClick(): void {
        this.isDeleteClicked = true;
    }

    public onClearSelection(): void {
        this.$store.dispatch(PM_ACTIONS.CLEAR_SELECTION);
        this.isDeleteClicked = false;

        this.$emit('onSuccessAction');
        this.$refs.headerComponent.clearSearch();
    }

    public async onDelete(): Promise<void> {
        try {
            await this.$store.dispatch(PM_ACTIONS.DELETE);
        } catch (error) {
            await this.$notify.error(`Error while deleting users from projectMembers. ${error.message}`);

            return;
        }

        this.$emit('onSuccessAction');
        await this.$notify.success('Members was successfully removed from project');
        this.isDeleteClicked = false;

        this.$refs.headerComponent.clearSearch();
    }

    public async processSearchQuery(search: string): Promise<void> {
        await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, search);
        try {
            await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project members. ${error.message}`);
        }
    }

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
}
</script>

<style scoped lang="scss">
    .team-header-container {

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            margin: 0;
        }
    }

    .header-default-state,
    .header-after-delete-click {
        display: flex;
        flex-direction: column;
        justify-content: space-between;
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
        align-items: flex-end;
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
            width: 602px;
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
</style>

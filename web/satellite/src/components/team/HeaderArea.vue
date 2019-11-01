// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-header-container">
        <div class="team-header-container__title-area">
            <h1 class="team-header-container__title-area__title">Project Members</h1>
            <VInfo
                class="team-header-container__title-area__info-button"
                bold-text="The only project role currently available is Admin, which gives full access to the project.">
                <svg class="team-header-container__title-area__info-button__image" width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect class="team-header-svg-rect" x="0.5" y="0.5" width="19" height="19" rx="9.5" stroke="#AFB7C1"/>
                    <path class="team-header-svg-path" d="M7 7.25177C7.00959 6.23527 7.28777 5.44177 7.83453 4.87129C8.38129 4.29043 9.1199 4 10.0504 4C10.952 4 11.6667 4.22819 12.1942 4.68458C12.7314 5.14097 13 5.79444 13 6.64498C13 7.03913 12.9376 7.38661 12.8129 7.68741C12.6882 7.98821 12.5396 8.24234 12.3669 8.44979C12.1942 8.65724 11.9592 8.90099 11.6619 9.18105C11.2686 9.54408 10.9712 9.876 10.7698 10.1768C10.5779 10.4672 10.482 10.8303 10.482 11.2659H9.04317C9.04317 10.851 9.10072 10.488 9.21583 10.1768C9.33094 9.86563 9.46523 9.6115 9.61871 9.41443C9.78177 9.20698 10.0024 8.96841 10.2806 8.69873C10.6067 8.37718 10.8465 8.09712 11 7.85856C11.1535 7.61999 11.2302 7.31919 11.2302 6.95615C11.2302 6.55163 11.1103 6.25082 10.8705 6.05375C10.6403 5.8463 10.3141 5.74257 9.89209 5.74257C9.45084 5.74257 9.10552 5.87223 8.85611 6.13154C8.60671 6.38048 8.47242 6.75389 8.45324 7.25177H7ZM9.73381 12.7595C10.0216 12.7595 10.2566 12.8633 10.4388 13.0707C10.6307 13.2782 10.7266 13.5427 10.7266 13.8642C10.7266 14.1961 10.6307 14.471 10.4388 14.6888C10.2566 14.8963 10.0216 15 9.73381 15C9.45564 15 9.22062 14.8911 9.02878 14.6733C8.84652 14.4554 8.7554 14.1858 8.7554 13.8642C8.7554 13.5427 8.84652 13.2782 9.02878 13.0707C9.22062 12.8633 9.45564 12.7595 9.73381 12.7595Z" fill="#354049"/>
                </svg>
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
import VInfo from '@/components/common/VInfo.vue';
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
        VInfo,
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

        &__title-area {
            display: flex;
            align-items: center;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
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

    /deep/ .info__message-box {
        background-image: url('../../../static/images/account/billing/MessageBox.png');
        background-repeat: no-repeat;
        min-height: 80px;
        min-width: 220px;
        width: 220px;
        top: 110%;
        left: -224%;
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

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-header-container">
        <HeaderComponent ref="headerComponent" placeHolder="Team Members" :search="processSearchQuery" title="Project Members">
            <div class="header-default-state" v-if="headerState === 0">
                <Button class="button" label="+Add" width="122px" height="48px" :onPress="onAddUsersClick"/>
                <span>The only project role currently available is Admin, which gives <b>full access</b> to the project.</span>
            </div>
            <div class="header-selected-members" v-if="headerState === 1 && !isDeleteClicked">
                <Button class="button deletion" label="Delete" width="122px" height="48px" :onPress="onFirstDeleteClick"/>
                <Button class="button" label="Cancel" width="122px" height="48px" isWhite="true" :onPress="onClearSelection"/>
            </div>
            <div class="header-after-delete-click" v-if="headerState === 1 && isDeleteClicked">
                <Button class="button deletion" label="Delete" width="122px" height="48px" :onPress="onDelete"/>
                <Button class="button" label="Cancel" width="122px" height="48px" isWhite="true" :onPress="onClearSelection"/>
                <span>Are you sure you want to delete {{selectedProjectMembers}} {{customUserCount}}</span>
            </div>
        </HeaderComponent>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';
    import HeaderComponent from '@/components/common/HeaderComponent.vue';
    import { TeamMember } from '@/types/teamMembers';
    import { RequestResponse } from '@/types/response';

    declare interface ClearSearch {
        clearSearch: () => void;
    }

    @Component({
        components: {
            Button,
            HeaderComponent,
        }
    })
    export default class HeaderArea extends Vue {
        @Prop({default: 0})
        private readonly headerState: number;
        @Prop({default: 0})
        private readonly selectedProjectMembers: number;

        private isDeleteClicked: boolean = false;

        public $refs!: {
            headerComponent: HeaderComponent & ClearSearch
        };

        public get customUserCount(): string {
            if (this.selectedProjectMembers === 1) {
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

            this.$refs.headerComponent.clearSearch();
        }

        public async onDelete(): Promise<void> {
            const projectMemberEmails = this.$store.getters.selectedProjectMembers.map((member: TeamMember) => {
                return member.user.email;
            });

            const response = await this.$store.dispatch(PM_ACTIONS.DELETE, projectMemberEmails);

            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Error while deleting users from team');

                return;
            }

            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Members was successfully removed from project');
            this.isDeleteClicked = false;

            this.$refs.headerComponent.clearSearch();
        }

        public async processSearchQuery(search: string) {
            this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, search);
            const response: RequestResponse<object> = await this.$store.dispatch(PM_ACTIONS.FETCH);

            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');
            }
        }
    }
</script>

<style scoped lang="scss">
    .header-default-state,
    .header-selected-members,
    .header-after-delete-click {
        display: flex;
        align-items: center;
    }

    .button {
        margin-right: 12px;
    }

    span {
        font-family: 'font_regular';
        font-size: 14px;
        line-height: 28px;
    }

    .team-header-container {
        margin-bottom: 4px;
        display: flex;
        align-items: center;
        justify-content: flex-start;
    }

    .container.deletion {
        background-color: #FF4F4D;

        &.label {
            color: #FFFFFF;
        }

        &:hover {
            background-color: #DE3E3D;
            box-shadow: none;
        }
    }
</style>

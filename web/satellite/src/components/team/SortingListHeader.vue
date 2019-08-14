// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__name-container" @click="onHeaderItemClick(ProjectMemberSortByEnum.NAME)">
            <p>Name</p>
            <VerticalArrows
                :isActive="sortBy === ProjectMemberSortByEnum.NAME"
                :direction="getSortDirection"/>
        </div>
        <div class="sort-header-container__added-container" @click="onHeaderItemClick(ProjectMemberSortByEnum.CREATED_AT)">
            <p>Added</p>
            <VerticalArrows
                :isActive="sortBy === ProjectMemberSortByEnum.CREATED_AT"
                :direction="getSortDirection"/>
        </div>
        <div class="sort-header-container__email-container" @click="onHeaderItemClick(ProjectMemberSortByEnum.EMAIL)">
            <p>Email</p>
            <VerticalArrows
                :isActive="sortBy === ProjectMemberSortByEnum.EMAIL"
                :direction="getSortDirection"/>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';
    import VerticalArrows from '@/components/common/VerticalArrows.vue';
    import { ProjectMemberSortByEnum, ProjectMemberSortDirectionEnum } from '@/utils/constants/ProjectMemberSortEnum';
    import { OnHeaderClickCallback } from '@/types/projectMembers';

    @Component({
        components: {
            VerticalArrows
        },
    })
    export default class SortingListHeader extends Vue {
        @Prop({default: () => { return new Promise(() => false); }})
        private readonly onHeaderClickCallback: OnHeaderClickCallback;

        public ProjectMemberSortByEnum = ProjectMemberSortByEnum;

        public sortBy: ProjectMemberSortByEnum = ProjectMemberSortByEnum.NAME;
        public sortDirection: ProjectMemberSortDirectionEnum = ProjectMemberSortDirectionEnum.ASCENDING;

        public get getSortDirection() {
            if (this.sortDirection === ProjectMemberSortDirectionEnum.DESCENDING) {
                return ProjectMemberSortDirectionEnum.ASCENDING;
            }

            return ProjectMemberSortDirectionEnum.DESCENDING;
        }

        public async onHeaderItemClick(sortBy: ProjectMemberSortByEnum) {
            if (this.sortBy != sortBy) {
                this.sortBy = sortBy;
                this.sortDirection = ProjectMemberSortDirectionEnum.ASCENDING;

                await this.notifyDatasetChanged();

                return;
            }

            if (this.sortDirection === ProjectMemberSortDirectionEnum.DESCENDING) {
                this.sortDirection = ProjectMemberSortDirectionEnum.ASCENDING;
            } else {
                this.sortDirection = ProjectMemberSortDirectionEnum.DESCENDING;
            }

            await this.notifyDatasetChanged();
        }

        public async notifyDatasetChanged() {
            await this.onHeaderClickCallback(this.sortBy, this.sortDirection);
            this.$forceUpdate();
        }
    }
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        flex-direction: row;
        height: 36px;
        margin-top: 200px;

        p {
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 23px;
            color: #AFB7C1;
            margin: 0;
        }

        &__name-container {
            display: flex;
            width: calc(50% - 30px);
            cursor: pointer;
            text-align: left;
            margin-left: 30px;
            align-items: center;
            justify-content: flex-start;
        }

        &__added-container {
             width: 25%;
             cursor: pointer;
             text-align: left;
             margin-left: 30px;
            display: flex;
            align-items: center;
            justify-content: flex-start;
        }

        &__email-container {
            width: 25%;
             cursor: pointer;
             text-align: left;
             display: flex;
             align-items: center;
             justify-content: flex-start;
        }
    }
</style>

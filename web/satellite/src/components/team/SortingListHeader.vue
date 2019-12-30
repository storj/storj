// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__name-container" @click="onHeaderItemClick(ProjectMemberOrderBy.NAME)">
            <p class="sort-header-container__name-container__title">Name</p>
            <VerticalArrows
                :is-active="areProjectMembersSortedByName"
                :direction="getSortDirection"
            />
        </div>
        <div class="sort-header-container__added-container" @click="onHeaderItemClick(ProjectMemberOrderBy.CREATED_AT)">
            <p class="sort-header-container__added-container__title">Added</p>
            <VerticalArrows
                :is-active="areProjectMembersSortedByDate"
                :direction="getSortDirection"
            />
        </div>
        <div class="sort-header-container__email-container" @click="onHeaderItemClick(ProjectMemberOrderBy.EMAIL)">
            <p class="sort-header-container__email-container__title">Email</p>
            <VerticalArrows
                :is-active="areProjectMembersSortedByEmail"
                :direction="getSortDirection"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VerticalArrows from '@/components/common/VerticalArrows.vue';

import { SortDirection } from '@/types/common';
import { OnHeaderClickCallback, ProjectMemberOrderBy } from '@/types/projectMembers';

@Component({
    components: {
        VerticalArrows,
    },
})
export default class SortingListHeader extends Vue {
    @Prop({default: () => new Promise(() => false)})
    private readonly onHeaderClickCallback: OnHeaderClickCallback;

    public ProjectMemberOrderBy = ProjectMemberOrderBy;

    public sortBy: ProjectMemberOrderBy = ProjectMemberOrderBy.NAME;
    public sortDirection: SortDirection = SortDirection.ASCENDING;

    public get getSortDirection(): SortDirection {
        if (this.sortDirection === SortDirection.DESCENDING) {
            return SortDirection.ASCENDING;
        }

        return SortDirection.DESCENDING;
    }

    public get areProjectMembersSortedByName(): boolean {
        return this.sortBy === ProjectMemberOrderBy.NAME;
    }

    public get areProjectMembersSortedByDate(): boolean {
        return this.sortBy === ProjectMemberOrderBy.CREATED_AT;
    }

    public get areProjectMembersSortedByEmail(): boolean {
        return this.sortBy === ProjectMemberOrderBy.EMAIL;
    }

    public async onHeaderItemClick(sortBy: ProjectMemberOrderBy): Promise<void> {
        if (this.sortBy !== sortBy) {
            this.sortBy = sortBy;
            this.sortDirection = SortDirection.ASCENDING;

            await this.onHeaderClickCallback(this.sortBy, this.sortDirection);

            return;
        }

        if (this.sortDirection === SortDirection.DESCENDING) {
            this.sortDirection = SortDirection.ASCENDING;
        } else {
            this.sortDirection = SortDirection.DESCENDING;
        }

        await this.onHeaderClickCallback(this.sortBy, this.sortDirection);
    }
}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        flex-direction: row;
        height: 40px;
        background-color: rgba(255, 255, 255, 0.3);
        margin-top: 31px;
        user-select: none;

        &__name-container,
        &__added-container,
        &__email-container {

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: #2a2a32;
                margin: 0;
            }
        }

        &__name-container {
            display: flex;
            width: calc(50% - 70px);
            cursor: pointer;
            text-align: left;
            margin-left: 70px;
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

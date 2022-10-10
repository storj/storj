// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__name-item" @click="onHeaderItemClick(AccessGrantsOrderBy.NAME)">
            <p class="sort-header-container__name-item__title">NAME</p>
            <VerticalArrows
                :is-active="areAccessGrantsSortedByName"
                :direction="getSortDirection"
            />
        </div>
        <div class="sort-header-container__date-item" @click="onHeaderItemClick(AccessGrantsOrderBy.CREATED_AT)">
            <p class="sort-header-container__date-item__title creation-date">DATE CREATED</p>
            <VerticalArrows
                :is-active="!areAccessGrantsSortedByName"
                :direction="getSortDirection"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { AccessGrantsOrderBy, OnHeaderClickCallback } from '@/types/accessGrants';
import { SortDirection } from '@/types/common';

import VerticalArrows from '@/components/common/VerticalArrows.vue';

// @vue/component
@Component({
    components: {
        VerticalArrows,
    },
})
export default class SortAccessGrantsHeader extends Vue {
    @Prop({ default: () => new Promise(() => false) })
    private readonly onHeaderClickCallback: OnHeaderClickCallback;

    public AccessGrantsOrderBy = AccessGrantsOrderBy;

    public sortBy: AccessGrantsOrderBy = AccessGrantsOrderBy.NAME;
    public sortDirection: SortDirection = SortDirection.ASCENDING;

    /**
     * Used for arrow styling.
     */
    public get getSortDirection(): SortDirection {
        return this.sortDirection === SortDirection.DESCENDING ? SortDirection.ASCENDING : SortDirection.DESCENDING;
    }

    public get areAccessGrantsSortedByName(): boolean {
        return this.sortBy === AccessGrantsOrderBy.NAME;
    }

    /**
     * Sets sorting kind if different from current.
     * If same, changes sort direction.
     * @param sortBy
     */
    public async onHeaderItemClick(sortBy: AccessGrantsOrderBy): Promise<void> {
        if (this.sortBy !== sortBy) {
            this.sortBy = sortBy;
            this.sortDirection = SortDirection.ASCENDING;

            await this.onHeaderClickCallback(this.sortBy, this.sortDirection);

            return;
        }

        this.sortDirection = this.sortDirection === SortDirection.DESCENDING ?
            SortDirection.ASCENDING
            : SortDirection.DESCENDING;

        await this.onHeaderClickCallback(this.sortBy, this.sortDirection);
    }
}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        width: calc(100% - 32px);
        height: 40px;
        background-color: #fff;
        margin-top: 31px;
        padding: 16px 16px 0;
        border-radius: 8px 8px 0 0;
        border: 1px solid #e5e7eb;
        border-bottom: 0;

        &__name-item,
        &__date-item {
            width: 50%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                margin: 0 0 0 23px;
                color: #2a2a32;
            }

            .creation-date {
                margin-left: 0;
            }
        }

        &__date-item {

            &__title {
                margin: 0;
            }
        }
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__name-item" @click="onHeaderItemClick(ApiKeyOrderBy.NAME)">
            <p class="sort-header-container__name-item__title">Key Name</p>
            <VerticalArrows
                :is-active="areApiKeysSortedByName"
                :direction="getSortDirection"
            />
        </div>
        <div class="sort-header-container__date-item" @click="onHeaderItemClick(ApiKeyOrderBy.CREATED_AT)">
            <p class="sort-header-container__date-item__title creation-date">Created</p>
            <VerticalArrows
                :is-active="!areApiKeysSortedByName"
                :direction="getSortDirection"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VerticalArrows from '@/components/common/VerticalArrows.vue';

import { ApiKeyOrderBy, OnHeaderClickCallback } from '@/types/apiKeys';
import { SortDirection } from '@/types/common';

@Component({
    components: {
        VerticalArrows,
    },
})
export default class SortApiKeysHeader extends Vue {
    @Prop({default: () => new Promise(() => false)})
    private readonly onHeaderClickCallback: OnHeaderClickCallback;

    public ApiKeyOrderBy = ApiKeyOrderBy;

    public sortBy: ApiKeyOrderBy = ApiKeyOrderBy.NAME;
    public sortDirection: SortDirection = SortDirection.ASCENDING;

    /**
     * Used for arrow styling.
     */
    public get getSortDirection(): SortDirection {
        return this.sortDirection === SortDirection.DESCENDING ? SortDirection.ASCENDING : SortDirection.DESCENDING;
    }

    public get areApiKeysSortedByName(): boolean {
        return this.sortBy === ApiKeyOrderBy.NAME;
    }

    /**
     * Sets sorting kind if different from current.
     * If same, changes sort direction.
     * @param sortBy
     */
    public async onHeaderItemClick(sortBy: ApiKeyOrderBy): Promise<void> {
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
        width: 100%;
        height: 40px;
        background-color: rgba(255, 255, 255, 0.3);
        margin-top: 31px;

        &__name-item,
        &__date-item {
            width: 60%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                margin: 0 0 0 80px;
                color: #2a2a32;
            }

            .creation-date {
                margin-left: 0;
            }
        }

        &__date-item {
            width: 40%;

            &__title {
                margin: 0;
            }
        }
    }
</style>

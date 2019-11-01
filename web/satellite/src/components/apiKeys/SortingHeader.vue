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
                :is-active="areApiKeysSortedByDate"
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

    public get getSortDirection(): SortDirection {
        if (this.sortDirection === SortDirection.DESCENDING) {
            return SortDirection.ASCENDING;
        }

        return SortDirection.DESCENDING;
    }

    public get areApiKeysSortedByName(): boolean {
        return this.sortBy === ApiKeyOrderBy.NAME;
    }

    public get areApiKeysSortedByDate(): boolean {
        return this.sortBy === ApiKeyOrderBy.CREATED_AT;
    }

    public async onHeaderItemClick(sortBy: ApiKeyOrderBy): Promise<void> {
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
        width: 100%;
        height: 40px;
        background-color: rgba(255, 255, 255, 0.3);
        margin-top: 31px;
        user-select: none;

        &__date-item {
            width: 60%;

            &__title {
                margin: 0;
            }
        }

        &__name-item,
        &__date-item {
            width: 40%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                margin: 0 0 0 26px;
                color: #2a2a32;
            }

            .creation-date {
                margin-left: 0;
            }
        }
    }
</style>

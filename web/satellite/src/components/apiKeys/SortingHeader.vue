// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__name-item" @click="onHeaderItemClick(ApiKeyOrderBy.NAME)">
            <p>Key Name</p>
            <VerticalArrows
                :isActive="getSortBy === ApiKeyOrderBy.NAME"
                :direction="getSortDirection"></VerticalArrows>
        </div>
        <div class="sort-header-container__date-item" @click="onHeaderItemClick(ApiKeyOrderBy.CREATED_AT)">
            <p>Created</p>
            <VerticalArrows
                :isActive="getSortBy === ApiKeyOrderBy.CREATED_AT"
                :direction="getSortDirection"></VerticalArrows>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VerticalArrows from '@/components/common/VerticalArrows.vue';

import { ApiKeyOrderBy, OnHeaderClickCallback } from '@/types/apiKeys';
import { SortDirection } from '@/types/common';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

@Component({
    components:{
        VerticalArrows,
    }
})
export default class SortApiKeysHeader extends Vue {
    public arrowUp: string = EMPTY_STATE_IMAGES.ARROW_UP;
    public arrowDown: string = EMPTY_STATE_IMAGES.ARROW_DOWN;

    @Prop({default: () => new Promise(() => false)})
    private readonly onHeaderClickCallback: OnHeaderClickCallback;

    public ApiKeyOrderBy = ApiKeyOrderBy;

    public sortBy: ApiKeyOrderBy = ApiKeyOrderBy.NAME;
    public sortDirection: SortDirection = SortDirection.ASCENDING;

    public get getSortDirection() {
        if (this.sortDirection === SortDirection.DESCENDING) {
            return SortDirection.ASCENDING;
        }

        return SortDirection.DESCENDING;
    }

    public get getSortBy() {
        return this.sortBy;
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

        &__name-item,
        &__date-item {
            width: 40%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;

            p {
                font-family: 'font_medium';
                font-size: 16px;
                margin-left: 26px;
                color: #AFB7C1;
            }

            &__arrows {
                display: flex;
                flex-direction: column;
                justify-content: flex-start;
                padding-bottom: 12px;
                margin-left: 10px;

                span.selected {

                    svg {

                        path {
                            fill: #2683FF !important;
                        }
                    }
                }

                span {
                    height: 10px;
                }
            }

            &:nth-child(1) {
                margin-left: 0px;
            }
        }

        &__date-item {
            width: 60%;

            P {
                margin: 0;
            }
        }
    }
</style>
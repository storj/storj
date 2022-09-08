// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pagination-container">
        <div class="pagination-container__pages">
            <span class="pagination-container__pages__label">{{ label }}</span>
            <div class="pagination-container__button" @click="prevPage">
                <PaginationRightIcon class="pagination-container__button__image reversed" />
            </div>
            <div class="pagination-container__button" @click="nextPage">
                <PaginationRightIcon class="pagination-container__button__image" />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { OnPageClickCallback } from '@/types/pagination';

import PaginationRightIcon from '@/../static/images/common/tablePaginationArrowRight.svg';

// @vue/component
@Component({
    components: {
        PaginationRightIcon,
    },
})
export default class TablePagination extends Vue {
    private currentPageNumber = 1;
    public isLoading = false;

    @Prop({ default: 0 })
    private readonly totalPageCount: number;
    @Prop({ default: 0 })
    private readonly limit: number;
    @Prop({ default: 0 })
    private readonly totalItemsCount: number;
    @Prop({ default: () => () => new Promise(() => false) })
    private readonly onPageClickCallback: OnPageClickCallback;

    public get label(): string {
        const currentMaxPage = this.currentPageNumber * this.limit > this.totalItemsCount ?
            this.totalItemsCount
            : this.currentPageNumber * this.limit;
        return `${this.currentPageNumber * this.limit - this.limit + 1} - ${currentMaxPage} of ${this.totalItemsCount}`;
    }

    /**
     * Indicates if current page is first.
     */
    public get isFirstPage(): boolean {
        return this.currentPageNumber === 1;
    }

    /**
     * Indicates if current page is last.
     */
    public get isLastPage(): boolean {
        return this.currentPageNumber === this.totalPageCount;
    }

    /**
     * nextPage fires after 'next' arrow click.
     */
    public async nextPage(): Promise<void> {
        if (this.isLastPage || this.isLoading) {
            return;
        }

        this.isLoading = true;
        await this.onPageClickCallback(this.currentPageNumber + 1);
        this.incrementCurrentPage();
        this.isLoading = false;
    }

    /**
     * prevPage fires after 'previous' arrow click.
     */
    public async prevPage(): Promise<void> {
        if (this.isFirstPage || this.isLoading) {
            return;
        }

        this.isLoading = true;
        await this.onPageClickCallback(this.currentPageNumber - 1);
        this.decrementCurrentPage();
        this.isLoading = false;
    }

    private incrementCurrentPage(): void {
        this.currentPageNumber++;
    }

    private decrementCurrentPage(): void {
        this.currentPageNumber--;
    }
}
</script>

<style scoped lang="scss">
.pagination-container {
    display: flex;
    align-items: center;
    justify-content: flex-end;

    &__pages {
        display: flex;
        align-items: center;

        &__label {
            margin-right: 25px;
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            line-height: 24px;
            color: rgb(44 53 58 / 60%);
        }
    }

    &__button {
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        width: 15px;
        height: 15px;
        max-width: 15px;
        max-height: 15px;
    }
}

.reversed {
    transform: rotate(180deg);
}
</style>

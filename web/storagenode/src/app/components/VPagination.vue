// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pagination-container">
        <div class="pagination-container__pages">
            <div
                class="pagination-container__button"
                @click="prevPage"
                :class="{ active: !isFirstPage }"
            >
                <p class="pagination-container__button__label">Prev</p>
            </div>
            <div class="pagination-container__items">
                <PagesBlock
                    :pages="firstBlockPages"
                    :is-selected="isSelected"
                />
                <span class="pages-divider" v-if="isFirstDotsShown">...</span>
                <PagesBlock
                    :pages="middleBlockPages"
                    :is-selected="isSelected"
                />
                <span class="pages-divider" v-if="isSecondDotsShown">...</span>
                <PagesBlock
                    :pages="lastBlockPages"
                    :is-selected="isSelected"
                />
            </div>
            <div
                class="pagination-container__button"
                @click="nextPage"
                :class="{ active: !isLastPage }"
            >
                <p class="pagination-container__button__label">Next</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import PagesBlock from '@/app/components/PagesBlock.vue';

import { OnPageClickCallback, Page } from '@/app/types/pagination';

@Component({
    components: {
        PagesBlock,
    },
})
export default class VPagination extends Vue {
    private readonly MAX_PAGES_PER_BLOCK: number = 3;
    private readonly MAX_PAGES_OFF_BLOCKS: number = 6;
    private currentPageNumber: number = 1;
    public isLoading = false;
    public pagesArray: Page[] = [];
    public firstBlockPages: Page[] = [];
    public middleBlockPages: Page[] = [];
    public lastBlockPages: Page[] = [];

    @Prop({default: 0})
    private readonly totalPageCount: number;
    @Prop({default: () => new Promise(() => false)})
    private readonly onPageClickCallback: OnPageClickCallback;

    /**
     * Component initialization.
     */
    public async mounted() {
        await this.populatePagesArray();
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
     * Indicates if dots after first pages block should appear.
     */
    public get isFirstDotsShown(): boolean {
        return this.middleBlockPages.length <= this.MAX_PAGES_PER_BLOCK
            && this.pagesArray.length > this.MAX_PAGES_OFF_BLOCKS;
    }

    /**
     * Indicates if dots after middle pages block should appear.
     */
    public get isSecondDotsShown(): boolean {
        return !!this.middleBlockPages.length;
    }

    /**
     * Indicates page is current and should appear in different styling.
     */
    public isSelected(page: number): boolean {
        return page === this.currentPageNumber;
    }

    /**
     * Method after total page count change.
     */
    @Watch('totalPageCount')
    public onPageCountChange() {
        this.resetPageIndex();
    }

    /**
     * onPageClick fires after concrete page click.
     */
    public async onPageClick(page: number): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;
        try {
            await this.onPageClickCallback(page);
        } catch (error) {
            // TODO: add notification here
            console.error(error.message);
            this.isLoading = false;

            return;
        }

        this.setCurrentPage(page);
        this.reorganizePageBlocks();
        this.isLoading = false;
    }

    /**
     * nextPage fires after 'next' arrow click.
     */
    public async nextPage(): Promise<void> {
        if (this.isLastPage || this.isLoading) {
            return;
        }

        this.isLoading = true;

        try {
            await this.onPageClickCallback(this.currentPageNumber + 1);
        } catch (error) {
            // TODO: add notification here
            console.error(error.message);
            this.isLoading = false;

            return;
        }

        this.incrementCurrentPage();
        this.reorganizePageBlocks();
        this.isLoading = false;
    }

    /**
     * nextPage fires after 'previous' arrow click.
     */
    public async prevPage(): Promise<void> {
        if (this.isFirstPage || this.isLoading) {
            return;
        }

        this.isLoading = true;

        try {
            await this.onPageClickCallback(this.currentPageNumber - 1);
        } catch (error) {
            // TODO: add notification here
            console.error(error.message);
            this.isLoading = false;

            return;
        }

        this.decrementCurrentPage();
        this.reorganizePageBlocks();
        this.isLoading = false;
    }

    /**
     * resetPageIndex sets current selected page as first and rebuilds page blocks after.
     */
    public resetPageIndex(): void {
        this.pagesArray = [];
        this.firstBlockPages = [];
        this.setCurrentPage(1);

        this.populatePagesArray();
    }

    /**
     * creates pages blocks and pages depends of total page count.
     */
    private populatePagesArray(): void {
        if (!this.totalPageCount) {
            return;
        }

        for (let i = 1; i <= this.totalPageCount; i++) {
            this.pagesArray.push(new Page(i, this.onPageClick));
        }

        if (this.isPagesTotalOffBlocks()) {
            this.firstBlockPages = this.pagesArray.slice();
            this.middleBlockPages = [];
            this.lastBlockPages = [];

            return;
        }

        this.reorganizePageBlocks();
    }

    /**
     * reorganizePageBlocks changes pages blocks organization depends of
     * current selected page index.
     */
    private reorganizePageBlocks(): void {
        if (this.isPagesTotalOffBlocks()) {
            return;
        }

        if (this.isCurrentInFirstBlock()) {
            this.setBlocksIfCurrentInFirstBlock();

            return;
        }

        if (!this.isCurrentInFirstBlock() && !this.isCurrentInLastBlock()) {
            this.setBlocksIfCurrentInMiddleBlock();

            return;
        }

        if (this.isCurrentInLastBlock()) {
            this.setBlocksIfCurrentInLastBlock();
        }
    }

    private setBlocksIfCurrentInFirstBlock(): void {
        this.firstBlockPages = this.pagesArray.slice(0, 3);
        this.middleBlockPages = [];
        this.lastBlockPages = this.pagesArray.slice(-1);
    }

    private setBlocksIfCurrentInMiddleBlock(): void {
        this.firstBlockPages = this.pagesArray.slice(0, 1);
        this.middleBlockPages = this.pagesArray.slice(this.currentPageNumber - 2, this.currentPageNumber + 1);
        this.lastBlockPages = this.pagesArray.slice(-1);
    }

    private setBlocksIfCurrentInLastBlock(): void {
        this.firstBlockPages = this.pagesArray.slice(0, 1);
        this.middleBlockPages = [];
        this.lastBlockPages = this.pagesArray.slice(-3);
    }

    private isCurrentInFirstBlock(): boolean {
        return this.currentPageNumber < this.MAX_PAGES_PER_BLOCK;
    }

    private isCurrentInLastBlock(): boolean {
        return this.totalPageCount - this.currentPageNumber < this.MAX_PAGES_PER_BLOCK - 1;
    }

    private isPagesTotalOffBlocks(): boolean {
        return this.totalPageCount <= this.MAX_PAGES_OFF_BLOCKS;
    }

    private incrementCurrentPage(): void {
        this.currentPageNumber++;
    }

    private decrementCurrentPage(): void {
        this.currentPageNumber--;
    }

    private setCurrentPage(pageNumber: number): void {
        this.currentPageNumber = pageNumber;
    }
}
</script>

<style scoped lang="scss">
    .pagination-container {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: 25px;

        &__pages {
            display: flex;
            align-items: center;
        }

        &__button {
            display: flex;
            align-items: center;
            justify-content: center;
            border: 1px solid #e8e8e8;
            border-radius: 6px;
            width: 30px;
            height: 30px;
            font-family: 'font_bold', sans-serif;
            font-size: 14px;
            color: var(--regular-text-color);
            padding: 5px 37px;

            &__label {
                margin: 0;
            }

            &.active {
                background-color: var(--block-background-color);
                cursor: pointer;
            }
        }

        &__items {
            margin: 0 20px;
            display: flex;

            .pages-divider {
                margin: 0 20px;
                color: var(--page-number-color);
            }
        }
    }
</style>

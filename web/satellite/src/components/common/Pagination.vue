// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pagination-container">
        <div class="pagination-container__pages">
            <div v-html="arrowLeft" @click="prevPage" class="pagination-container__button"></div>
            <div class="pagination-container__items">
                <PagesBlock :pages="firstBlockPages" :checkSelected="isSelected"/>
                <span v-if="isFirstDotsShown">...</span>
                <PagesBlock :pages="middleBlockPages" :checkSelected="isSelected"/>
                <span v-if="isSecondDotsShown">...</span>
                <PagesBlock :pages="lastBlockPages" :checkSelected="isSelected"/>
            </div>
            <div v-html="arrowRight" @click="nextPage" class="pagination-container__button"></div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue, Watch } from 'vue-property-decorator';

import PagesBlock from '@/components/common/PagesBlock.vue';

import { Page } from '@/types/pagination';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

@Component({
    components: {
        PagesBlock,
    }
})
export default class Pagination extends Vue {
    // TODO: use svg loader
    public readonly arrowLeft: string = EMPTY_STATE_IMAGES.ARROW_LEFT;
    public readonly arrowRight: string = EMPTY_STATE_IMAGES.ARROW_RIGHT;
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

    public mounted() {
        this.populatePagesArray();
    }

    public get isFirstPage(): boolean {
        return this.currentPageNumber === 1;
    }

    public get isLastPage(): boolean {
        return this.currentPageNumber === this.totalPageCount;
    }

    public get isFirstDotsShown(): boolean {
        return this.middleBlockPages.length <= this.MAX_PAGES_PER_BLOCK
            && this.pagesArray.length > this.MAX_PAGES_OFF_BLOCKS;
    }

    public get isSecondDotsShown(): boolean {
        return !!this.middleBlockPages.length;
    }

    public isSelected(page: number): boolean {
        return page === this.currentPageNumber;
    }

    @Watch('totalPageCount')
    public onPageCountChange(val: number, oldVal: number) {
        this.resetPageIndex();
    }

    public async onPageClick(page: number): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;
        await this.onPageClickCallback(page);
        this.setCurrentPage(page);
        this.reorganizePageBlocks();
        this.isLoading = false;
    }

    public async nextPage(): Promise<void> {
        if (this.isLastPage || this.isLoading) {
            return;
        }

        this.isLoading = true;
        await this.onPageClickCallback(this.currentPageNumber + 1);
        this.incrementCurrentPage();
        this.reorganizePageBlocks();
        this.isLoading = false;
    }

    public async prevPage(): Promise<void> {
        if (this.isFirstPage || this.isLoading) {
            return;
        }

        this.isLoading = true;
        await this.onPageClickCallback(this.currentPageNumber - 1);
        this.decrementCurrentPage();
        this.reorganizePageBlocks();
        this.isLoading = false;
    }

    public resetPageIndex(): void {
        this.pagesArray = [];
        this.firstBlockPages = [];
        this.setCurrentPage(1);

        this.populatePagesArray();
    }

    private populatePagesArray(): void {
        if (!this.totalPageCount) {
            return;
        }

        if (this.$route.query.pageNumber) {
            const pageNumber = parseInt(this.$route.query.pageNumber as string);
            this.setCurrentPage(pageNumber);

            // Here we need to set short timeout to let router to set up after page
            // hard reload before we can replace query with current page number
            setTimeout(this.updateRouterPathWithPageNumber, 1);
        }

        for (let i = 1; i <= this.totalPageCount; i++) {
            this.pagesArray.push(new Page(i, this.onPageClick));
        }

        if (this.isPagesTotalOffBlocks()) {
            this.firstBlockPages = this.pagesArray.slice();

            return;
        }

        this.reorganizePageBlocks();
    }

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
        this.updateRouterPathWithPageNumber();
    }

    private decrementCurrentPage(): void {
        this.currentPageNumber--;
        this.updateRouterPathWithPageNumber();
    }

    private setCurrentPage(pageNumber: number): void {
        this.currentPageNumber = pageNumber;
        this.updateRouterPathWithPageNumber();
    }

    private updateRouterPathWithPageNumber() {
        this.$router.replace({ query: { pageNumber: this.currentPageNumber.toString() } });
    }
}
</script>

<style scoped lang="scss">
    .pagination-container {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding-left: 25px;
        margin-top: 25px;

        &__pages {
            display: flex;
            align-items: center;
        }

        &__counter {

            p {
                font-family: 'font_medium';
                font-size: 16px;
                color: #AFB7C1;
            }
        }

        &__button {
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            border: 1px solid #AFB7C1;
            border-radius: 6px;
            width: 30px;
            height: 30px;

            &:hover {

                svg {

                    path {
                        fill: #fff !important;
                    }
                }
            }
        }

        &__items {
            margin: 0 20px;
            display: flex;

            span {
                margin: 0 20px;
            }
        }
    }
</style>

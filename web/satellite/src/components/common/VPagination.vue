// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pagination-container">
        <div class="pagination-container__pages">
            <div class="pagination-container__button" @click="prevPage">
                <PaginationLeftIcon class="pagination-container__button__image" />
            </div>
            <div class="pagination-container__items">
                <PagesBlock
                    :pages="firstBlockPages"
                    :is-selected="isSelected"
                />
                <span v-if="isFirstDotsShown" class="pages-divider">...</span>
                <PagesBlock
                    :pages="middleBlockPages"
                    :is-selected="isSelected"
                />
                <span v-if="isSecondDotsShown" class="pages-divider">...</span>
                <PagesBlock
                    :pages="lastBlockPages"
                    :is-selected="isSelected"
                />
            </div>
            <div class="pagination-container__button" @click="nextPage">
                <PaginationRightIcon class="pagination-container__button__image" />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { OnPageClickCallback, Page } from '@/types/pagination';
import { useLoading } from '@/composables/useLoading';

import PagesBlock from '@/components/common/PagesBlock.vue';

import PaginationLeftIcon from '@/../static/images/common/paginationLeft.svg';
import PaginationRightIcon from '@/../static/images/common/paginationRight.svg';

const MAX_PAGES_PER_BLOCK = 3;
const MAX_PAGES_OFF_BLOCKS = 6;

const props = withDefaults(defineProps<{
    onPageClickCallback?: OnPageClickCallback;
    totalPageCount?: number;
}>(), {
    totalPageCount: 0,
    onPageClickCallback: () => (_: number) => Promise.resolve(),
});

const { withLoading } = useLoading();

const currentPageNumber = ref<number>(1);
const pagesArray = ref<Page[]>([]);
const firstBlockPages = ref<Page[]>([]);
const middleBlockPages = ref<Page[]>([]);
const lastBlockPages = ref<Page[]>([]);

/**
 * Indicates if current page is first.
 */
const isFirstPage = computed((): boolean => {
    return currentPageNumber.value === 1;
});

/**
 * Indicates if current page is last.
 */
const isLastPage = computed((): boolean => {
    return currentPageNumber.value === props.totalPageCount;
});

/**
 * Indicates if dots after first pages block should appear.
 */
const isFirstDotsShown = computed((): boolean => {
    return middleBlockPages.value.length <= MAX_PAGES_PER_BLOCK
        && pagesArray.value.length > MAX_PAGES_OFF_BLOCKS;
});

/**
 * Indicates if dots after middle pages block should appear.
 */
const isSecondDotsShown = computed((): boolean => {
    return !!middleBlockPages.value.length;
});

const isCurrentInFirstBlock = computed((): boolean => {
    return currentPageNumber.value < MAX_PAGES_PER_BLOCK;
});

const isCurrentInLastBlock = computed((): boolean => {
    return props.totalPageCount - currentPageNumber.value < MAX_PAGES_PER_BLOCK - 1;
});

const isPagesTotalOffBlocks = computed((): boolean => {
    return props.totalPageCount <= MAX_PAGES_OFF_BLOCKS;
});

/**
 * Indicates page is current and should appear in different styling.
 */
function isSelected(page: number): boolean {
    return page === currentPageNumber.value;
}

/**
 * Creates pages blocks and pages depends of total page count.
 */
function populatePagesArray(): void {
    if (!props.totalPageCount) {
        return;
    }

    for (let i = 1; i <= props.totalPageCount; i++) {
        pagesArray.value.push(new Page(i, onPageClick));
    }

    if (isPagesTotalOffBlocks.value) {
        firstBlockPages.value = pagesArray.value.slice();
        middleBlockPages.value = [];
        lastBlockPages.value = [];

        return;
    }

    reorganizePageBlocks();
}

function setBlocksIfCurrentInFirstBlock(): void {
    firstBlockPages.value = pagesArray.value.slice(0, 3);
    middleBlockPages.value = [];
    lastBlockPages.value = pagesArray.value.slice(-1);
}

function setBlocksIfCurrentInMiddleBlock(): void {
    firstBlockPages.value = pagesArray.value.slice(0, 1);
    middleBlockPages.value = pagesArray.value.slice(currentPageNumber.value - 2, currentPageNumber.value + 1);
    lastBlockPages.value = pagesArray.value.slice(-1);
}

function setBlocksIfCurrentInLastBlock(): void {
    firstBlockPages.value = pagesArray.value.slice(0, 1);
    middleBlockPages.value = [];
    lastBlockPages.value = pagesArray.value.slice(-3);
}

function incrementCurrentPage(): void {
    currentPageNumber.value++;
}

function decrementCurrentPage(): void {
    currentPageNumber.value--;
}

function setCurrentPage(pageNumber: number): void {
    currentPageNumber.value = pageNumber;
}

/**
 * onPageClick fires after concrete page click.
 */
async function onPageClick(page: number): Promise<void> {
    await withLoading(async () => {
        await props.onPageClickCallback(page);
        setCurrentPage(page);
        reorganizePageBlocks();
    });
}

/**
 * nextPage fires after 'next' arrow click.
 */
async function nextPage(): Promise<void> {
    await withLoading(async () => {
        if (isLastPage.value) {
            return;
        }

        await props.onPageClickCallback(currentPageNumber.value + 1);
        incrementCurrentPage();
        reorganizePageBlocks();
    });
}

/**
 * prevPage fires after 'previous' arrow click.
 */
async function prevPage(): Promise<void> {
    await withLoading(async () => {
        if (isFirstPage.value) {
            return;
        }

        await props.onPageClickCallback(currentPageNumber.value - 1);
        decrementCurrentPage();
        reorganizePageBlocks();
    });
}

/**
 * reorganizePageBlocks changes pages blocks organization depends of
 * current selected page index.
 */
function reorganizePageBlocks(): void {
    if (isPagesTotalOffBlocks.value) {
        return;
    }

    if (isCurrentInFirstBlock.value) {
        setBlocksIfCurrentInFirstBlock();

        return;
    }

    if (!isCurrentInFirstBlock.value && !isCurrentInLastBlock.value) {
        setBlocksIfCurrentInMiddleBlock();

        return;
    }

    if (isCurrentInLastBlock.value) {
        setBlocksIfCurrentInLastBlock();
    }
}

/**
 * Component initialization.
 */
onMounted((): void => {
    populatePagesArray();
});
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

        &__button {
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            border: 1px solid #afb7c1;
            border-radius: 6px;
            width: 30px;
            height: 30px;

            &:hover {

                .pagination-svg-path {
                    fill: #fff !important;
                }
            }
        }

        &__items {
            margin: 0 20px;
            display: flex;

            .pages-divider {
                margin: 0 20px;
            }
        }
    }
</style>

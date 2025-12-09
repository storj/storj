// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pagination-container">
        <div class="pagination-container__pages">
            <div class="pagination-container__button" @click="prevPage">
                <svg width="30" height="30" viewBox="0 0 30 30" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M9.80078 9.2625L15.5258 15L9.80078 20.7375L11.5633 22.5L19.0633 15L11.5633 7.5L9.80078 9.2625Z" fill="currentColor" />
                </svg>
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
                <svg width="30" height="30" viewBox="0 0 30 30" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M9.80078 9.2625L15.5258 15L9.80078 20.7375L11.5633 22.5L19.0633 15L11.5633 7.5L9.80078 9.2625Z" fill="currentColor" />
                </svg>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';

import { OnPageClickCallback, Page } from '@/app/types/pagination';

import PagesBlock from '@/app/components/common/PagesBlock.vue';

const props = withDefaults(defineProps<{
    totalPageCount?: number;
    preselectedCurrentPageNumber?: number;
    onPageClickCallback?: OnPageClickCallback;
}>(), {
    totalPageCount: 0,
    preselectedCurrentPageNumber: 1,
    onPageClickCallback: () => Promise.resolve,
});

const MAX_PAGES_PER_BLOCK = 3;
const MAX_PAGES_OFF_BLOCKS = 6;

const currentPageNumber = ref<number>(1);
const isLoading = ref<boolean>(false);
const pagesArray = ref<Page[]>([]);
const firstBlockPages = ref<Page[]>([]);
const middleBlockPages = ref<Page[]>([]);
const lastBlockPages = ref<Page[]>([]);

const isFirstPage = computed<boolean>(() => currentPageNumber.value === 1);
const isLastPage = computed<boolean>(() => currentPageNumber.value === props.totalPageCount);
const isFirstDotsShown = computed<boolean>(() => {
    return middleBlockPages.value.length <= MAX_PAGES_PER_BLOCK
        && pagesArray.value.length > MAX_PAGES_OFF_BLOCKS;
});
const isSecondDotsShown = computed<boolean>(() => !!middleBlockPages.value.length);
const isOneBlockRequired = computed<boolean>(() => props.totalPageCount <= MAX_PAGES_OFF_BLOCKS);
const isCurrentInFirstBlock = computed<boolean>(() => currentPageNumber.value < MAX_PAGES_PER_BLOCK);
const isCurrentInLastBlock = computed<boolean>(() => props.totalPageCount - currentPageNumber.value < MAX_PAGES_PER_BLOCK - 1);

function isSelected(page: number): boolean {
    return page === currentPageNumber.value;
}

async function onPageClick(page: number): Promise<void> {
    if (isLoading.value) return;

    isLoading.value = true;

    await props.onPageClickCallback(page);
    currentPageNumber.value = page;
    reorganizePageBlocks();

    isLoading.value = false;
}

async function nextPage(): Promise<void> {
    if (isLastPage.value || isLoading.value) return;

    isLoading.value = true;

    await props.onPageClickCallback(currentPageNumber.value + 1);
    incrementCurrentPage();
    reorganizePageBlocks();

    isLoading.value = false;
}

async function prevPage(): Promise<void> {
    if (isFirstPage.value || isLoading.value) return;

    isLoading.value = true;

    await props.onPageClickCallback(currentPageNumber.value - 1);
    decrementCurrentPage();
    reorganizePageBlocks();

    isLoading.value = false;
}

function resetPageIndex(): void {
    pagesArray.value = [];
    firstBlockPages.value = [];
    setCurrentPage(1);

    populatePages();
}

function populatePages(): void {
    if (!props.totalPageCount) return;

    for (let i = 1; i <= props.totalPageCount; i++) {
        pagesArray.value.push(new Page(i, onPageClick));
    }

    if (isOneBlockRequired.value) {
        firstBlockPages.value = pagesArray.value.slice();
        middleBlockPages.value = [];
        lastBlockPages.value = [];

        return;
    }

    reorganizePageBlocks();
}

function reorganizePageBlocks(): void {
    if (isOneBlockRequired.value) {
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

onMounted(() => {
    populatePages();
    currentPageNumber.value = props.preselectedCurrentPageNumber;
});

watch(() => props.totalPageCount, () => {
    resetPageIndex();
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
            border: 1px solid #e1e3e6;
            border-radius: 6px;
            width: 40px;
            height: 40px;

            &:first-of-type {

                svg {
                    transform: rotate(180deg);
                }
            }

            &:hover {
                color: #2379ec;
                border-color: #2379ec;
            }
        }

        &__items {
            margin: 0 20px;
            display: flex;

            .pages-divider {
                border: 1px solid #e1e3e6;
                border-radius: 6px;
                width: 40px;
                height: 40px;
                display: flex;
                align-items: center;
                justify-content: center;
                margin: 0 20px;
            }
        }
    }
</style>

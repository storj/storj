// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pagination-container">
        <div class="pagination-container__pages">
            <div
                class="pagination-container__button"
                :class="{ active: !isFirstPage }"
                @click="prevPage"
            >
                <p class="pagination-container__button__label">Prev</p>
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
            <div
                class="pagination-container__button"
                :class="{ active: !isLastPage }"
                @click="nextPage"
            >
                <p class="pagination-container__button__label">Next</p>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';

import { OnPageClickCallback, Page } from '@/app/types/pagination';

import PagesBlock from '@/app/components/PagesBlock.vue';

const props = withDefaults(defineProps<{
    totalPageCount?: number;
    onPageClickCallback?: OnPageClickCallback;
}>(), {
    totalPageCount: 0,
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

const isFirstPage = computed<boolean>(() => {
    return currentPageNumber.value === 1;
});

const isLastPage = computed<boolean>(() => {
    return currentPageNumber.value === props.totalPageCount;
});

const isFirstDotsShown = computed<boolean>(() => {
    return middleBlockPages.value.length <= MAX_PAGES_PER_BLOCK
        && pagesArray.value.length > MAX_PAGES_OFF_BLOCKS;
});

const isSecondDotsShown = computed<boolean>(() => {
    return !!middleBlockPages.value.length;
});

function isSelected(page: number): boolean {
    return page === currentPageNumber.value;
}

async function onPageClick(page: number): Promise<void> {
    if (isLoading.value) {
        return;
    }

    isLoading.value = true;
    try {
        await props.onPageClickCallback(page);
    } catch (error) {
        // TODO: add notification here
        console.error(error);
        isLoading.value = false;

        return;
    }

    setCurrentPage(page);
    reorganizePageBlocks();
    isLoading.value = false;
}

async function nextPage(): Promise<void> {
    if (isLastPage.value || isLoading.value) {
        return;
    }

    isLoading.value = true;

    try {
        await props.onPageClickCallback(currentPageNumber.value + 1);
    } catch (error) {
        // TODO: add notification here
        console.error(error);
        isLoading.value = false;

        return;
    }

    incrementCurrentPage();
    reorganizePageBlocks();
    isLoading.value = false;
}

async function prevPage(): Promise<void> {
    if (isFirstPage.value || isLoading.value) {
        return;
    }

    isLoading.value = true;

    try {
        await props.onPageClickCallback(currentPageNumber.value - 1);
    } catch (error) {
        // TODO: add notification here
        console.error(error);
        isLoading.value = false;

        return;
    }

    decrementCurrentPage();
    reorganizePageBlocks();
    isLoading.value = false;
}

function resetPageIndex(): void {
    pagesArray.value = [];
    firstBlockPages.value = [];
    setCurrentPage(1);

    populatePagesArray();
}

function populatePagesArray(): void {
    if (!props.totalPageCount) {
        return;
    }

    for (let i = 1; i <= props.totalPageCount; i++) {
        pagesArray.value.push(new Page(i, onPageClick));
    }

    if (isPagesTotalOffBlocks()) {
        firstBlockPages.value = pagesArray.value.slice();
        middleBlockPages.value = [];
        lastBlockPages.value = [];

        return;
    }

    reorganizePageBlocks();
}

function reorganizePageBlocks(): void {
    if (isPagesTotalOffBlocks()) {
        return;
    }

    if (isCurrentInFirstBlock()) {
        setBlocksIfCurrentInFirstBlock();

        return;
    }

    if (!isCurrentInFirstBlock() && !isCurrentInLastBlock()) {
        setBlocksIfCurrentInMiddleBlock();

        return;
    }

    if (isCurrentInLastBlock()) {
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

function isCurrentInFirstBlock(): boolean {
    return currentPageNumber.value < MAX_PAGES_PER_BLOCK;
}

function isCurrentInLastBlock(): boolean {
    return props.totalPageCount - currentPageNumber.value < MAX_PAGES_PER_BLOCK - 1;
}

function isPagesTotalOffBlocks(): boolean {
    return props.totalPageCount <= MAX_PAGES_OFF_BLOCKS;
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

watch(() => props.totalPageCount, () => {
    resetPageIndex();
});

onMounted(() => {
    populatePagesArray();
});
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

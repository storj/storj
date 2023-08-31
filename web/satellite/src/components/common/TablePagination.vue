// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pagination-container">
        <span v-if="totalItemsCount > 0" class="pagination-container__label">{{ totalItemsCount }} {{ itemsLabel }}</span>
        <span v-else class="pagination-container__label">No {{ itemsLabel }}</span>

        <div v-if="simplePagination" class="pagination-container__pages">
            <span
                tabindex="0"
                class="pagination-container__pages__button"
                @click="prevPage"
                @keyup.enter="prevPage"
            >
                <PaginationRightIcon class="pagination-container__pages__button__image reversed" />
            </span>

            <span
                tabindex="0"
                class="pagination-container__pages__button"
                @click="nextPage"
                @keyup.enter="nextPage"
            >
                <PaginationRightIcon class="pagination-container__pages__button__image" />
            </span>
        </div>
        <div v-else-if="totalPageCount > 1" class="pagination-container__pages">
            <template v-for="page of pageItems">
                <span
                    v-if="page.type === 'prev'"
                    :key="page.type"
                    tabindex="0"
                    class="pagination-container__pages__button"
                    @click="prevPage"
                    @keyup.enter="prevPage"
                >
                    <PaginationRightIcon class="pagination-container__pages__button__image reversed" />
                </span>
                <span
                    v-if="page.type === 'prev_3'"
                    :key="page.type"
                    tabindex="0"
                    class="pagination-container__pages__label jumper"
                    @click="() => goToPage(currentPageNumber - 3)"
                    @keyup.enter="() => goToPage(currentPageNumber - 3)"
                >...</span>
                <span
                    v-if="page.type === 'page'"
                    :key="page.type + page.index"
                    tabindex="0"
                    class="pagination-container__pages__label index"
                    :class="{selected: page.index === currentPageNumber}"
                    @click="() => goToPage(page.index)"
                    @keyup.enter="() => goToPage(page.index)"
                >{{ page.index }}</span>
                <span
                    v-if="page.type === 'next_3'"
                    :key="page.type"
                    tabindex="0"
                    class="pagination-container__pages__label jumper"
                    @click="() => goToPage(currentPageNumber + 3)"
                    @keyup.enter="() => goToPage(currentPageNumber + 3)"
                >...</span>
                <span
                    v-if="page.type === 'next'"
                    :key="page.type"
                    tabindex="0"
                    class="pagination-container__pages__button"
                    @click="nextPage"
                    @keyup.enter="nextPage"
                >
                    <PaginationRightIcon class="pagination-container__pages__button__image" />
                </span>
            </template>
        </div>
        <div v-else class="pagination-container__pages-placeholder" />

        <table-size-changer
            v-if="(limit && totalPageCount && totalItemsCount > 10) || simplePagination"
            simple-pagination
            :item-count="totalItemsCount"
            :selected="pageSize"
            @change="sizeChanged"
        />
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { PageChangeCallback } from '@/types/pagination';
import { useLoading } from '@/composables/useLoading';

import TableSizeChanger from '@/components/common/TableSizeChanger.vue';

import PaginationRightIcon from '@/../static/images/common/tablePaginationArrowRight.svg';

// Represents each type of item in the pagination component.
interface PaginationControlItem {
  index?: number;
  type: 'page' | 'prev' | 'next' | 'next_3' | 'prev_3';
}

const { withLoading } = useLoading();

const props = withDefaults(defineProps<{
    itemsLabel?: string,
    totalPageCount?: number;
    limit?: number;
    totalItemsCount?: number;
    simplePagination?: boolean;
    onPageChange?: PageChangeCallback | null;
    onNextClicked?: (() => Promise<void>) | null;
    onPreviousClicked?: (() => Promise<void>) | null;
    onPageSizeChanged?: ((size: number) => Promise<void> | void) | null;
}>(), {
    itemsLabel: 'items',
    totalPageCount: 0,
    limit: 0,
    totalItemsCount: 0,
    simplePagination: false,
    onPageChange: null,
    onNextClicked: null,
    onPreviousClicked: null,
    onPageSizeChanged: null,
});

const currentPageNumber = ref<number>(1);
const pageSize = ref<number>(props.limit || 10);

/**
 * a list of pagination items in the form prev, jumper, page, page, page, jumper, next
 * ie: (< 1...4 5 6...10 >), (< 1 2 3...10 >) or (< ...8 9 10 >)
 * inspired by https://github.com/NG-ZORRO/ng-zorro-antd/blob/59143d241858a0a72e395f28bdaee55d1b4f6c6e/components/pagination/pagination-default.component.ts#L161-L213
 */
const pageItems = computed((): PaginationControlItem[] => {
    const pageIndex = currentPageNumber.value;
    const lastIndex = props.totalPageCount ?? 10;
    const prevItem: PaginationControlItem = { type: 'prev' };
    const nextItem: PaginationControlItem = { type: 'next' };
    const generatePages = (start: number, end: number): PaginationControlItem[] => {
        const list: PaginationControlItem[] = [];
        for (let i = start; i <= end; i++) {
            list.push({ index: i, type: 'page' });
        }
        return list;
    };
    if (lastIndex > 4) {
        // We try to show a max of 3 pages on the left/right apart from the first and last pages, separated by ellipses.
        let rangedItems: PaginationControlItem[];
        const prev3Item: PaginationControlItem = { type: 'prev_3' }; // ellipses
        const next3Item: PaginationControlItem = { type: 'next_3' }; // ellipses
        const firstPageItem = generatePages(1, 1);
        const lastPageItem = generatePages(lastIndex, lastIndex);

        if (pageIndex < 4) {
            // If the 3rd is selected, a 4th page will be displayed. Without this, the 4th page will only be accessible by
            // clicking next.
            const maxLeft = pageIndex === 3 ? 4 : 3;
            rangedItems = [...generatePages(2, maxLeft), next3Item];
        } else if (pageIndex < lastIndex - 2) {
            rangedItems = [prev3Item, ...generatePages(pageIndex - 1, pageIndex + 1), next3Item];
        } else {
            // If the 3rd from last is selected, one more page will be displayed.
            const minRight = pageIndex === lastIndex - 2 ? lastIndex - 3 : lastIndex - 2;
            rangedItems = [prev3Item, ...generatePages(minRight, lastIndex - 1)];
        }
        return [prevItem, ...firstPageItem, ...rangedItems, ...lastPageItem, nextItem];
    }
    // If the page count is 4 or less, we just display all of them.
    return [prevItem, ...generatePages(1, lastIndex), nextItem];
});

const label = computed((): string => {
    const currentMaxPage = currentPageNumber.value * props.limit > props.totalItemsCount ?
        props.totalItemsCount
        : currentPageNumber.value * props.limit;
    return `${currentPageNumber.value * props.limit - props.limit + 1} - ${currentMaxPage} of ${props.totalItemsCount}`;
});

const isFirstPage = computed((): boolean => {
    return currentPageNumber.value === 1;
});

const isLastPage = computed((): boolean => {
    return currentPageNumber.value === props.totalPageCount;
});

function sizeChanged(size: number) {
    withLoading(async () => {
        if (props.simplePagination) {
            if (!props.onPageSizeChanged) {
                return;
            }
            await props.onPageSizeChanged(size);
            pageSize.value = size;
        }
        // if the new size is large enough to cause the page index to be out of range
        // we calculate an appropriate new page index.
        const maxPage = Math.ceil(Math.ceil(props.totalItemsCount / size));
        const page = currentPageNumber.value > maxPage ? maxPage : currentPageNumber.value;
        if (!props.onPageChange) {
            return;
        }
        await props.onPageChange(page, size);
        pageSize.value = size;
        currentPageNumber.value = page;
    });
}

async function goToPage(index?: number): Promise<void> {
    if (index === undefined || index === currentPageNumber.value) {
        return;
    }
    if (index < 1) {
        index = 1;
    } else if (index > (props.totalPageCount ?? 1)) {
        index = props.totalPageCount ?? 1;
    }
    await withLoading(async () => {
        if (!props.onPageChange || index === undefined) {
            return;
        }
        await props.onPageChange(index, pageSize.value);
        currentPageNumber.value = index;
    });
}

/**
 * nextPage fires after 'next' arrow click.
 */
async function nextPage(): Promise<void> {
    await withLoading(async () => {
        if (props.simplePagination && props.onNextClicked) {
            await props.onNextClicked();
            return;
        }
        if (isLastPage.value || !props.onPageChange) {
            return;
        }
        await props.onPageChange(currentPageNumber.value + 1, pageSize.value);
        currentPageNumber.value++;
    });
}

/**
 * prevPage fires after 'previous' arrow click.
 */
async function prevPage(): Promise<void> {
    await withLoading(async () => {
        if (props.simplePagination && props.onPreviousClicked) {
            await props.onPreviousClicked();
            return;
        }
        if (isFirstPage.value || !props.onPageChange) {
            return;
        }
        await props.onPageChange(currentPageNumber.value - 1, pageSize.value);
        currentPageNumber.value--;
    });
}
</script>

<style scoped lang="scss">
.pagination-container {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr;
    align-items: center;

    &__label {
        font-family: 'font_regular', sans-serif;
        color: var(--c-grey-5);
    }

    &__pages {
        display: flex;
        align-items: center;
        justify-content: center;
        user-select: none;

        &__button {
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            width: 24px;
            height: 24px;
            box-sizing: border-box;

            &:last-of-type {
                margin-left: 2px;
            }

            &:hover {
                border: 1px solid var(--c-grey-3);
                border-radius: 5px;
            }
        }

        &__label {
            font-family: 'font_regular', sans-serif;
            color: var(--c-grey-6);
            text-align: center;
            line-height: 24px;
            padding: 2px 5px;
            margin-left: 2px;
            box-sizing: border-box;
            border: 1px solid transparent;
            cursor: pointer;

            &:hover {
                border-color: var(--c-grey-3);
                border-radius: 5px;
                line-height: 22px;
            }

            &.selected {
                background: var(--c-grey-2);
                border-color: var(--c-grey-3);
                border-radius: 5px;
                line-height: 22px;
            }

            @media only screen and (width <= 550px) {

                &.index:not(.selected),
                &.jumper {
                    display: none;
                }
            }
        }
    }
}

.reversed {
    transform: rotate(180deg);
}
</style>

// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <fragment>
        <th
            class="align-left"
            @mouseover="mouseOver(AccessGrantsOrderBy.NAME)"
            @mouseleave="mouseLeave"
            @click="sortBy(AccessGrantsOrderBy.NAME)"
        >
            <span class="header__item">
                <span>Name</span>
                <span :class="{ invisible: nameSortData.isHidden }">
                    <a v-if="nameSortData.isDesc" class="arrow">
                        <desc-icon />
                    </a>
                    <a v-else class="arrow">
                        <asc-icon />
                    </a>
                </span>
            </span>
        </th>
        <th
            class="align-left"
            @mouseover="mouseOver(AccessGrantsOrderBy.CREATED_AT)"
            @mouseleave="mouseLeave"
            @click="sortBy(AccessGrantsOrderBy.CREATED_AT)"
        >
            <span class="header__item">
                <span>Date Created</span>
                <span :class="{ invisible: dateSortData.isHidden }">
                    <a v-if="dateSortData.isDesc" class="arrow">
                        <desc-icon />
                    </a>
                    <a v-else class="arrow">
                        <asc-icon />
                    </a>
                </span>
            </span>
        </th>
    </fragment>
</template>

<script setup lang="ts">
import { Fragment } from 'vue-fragment';
import { computed, ref } from 'vue';

import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { SortDirection } from '@/types/common';
import { AccessGrantsOrderBy } from '@/types/accessGrants';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import AscIcon from '@/../static/images/objects/asc.svg';
import DescIcon from '@/../static/images/objects/desc.svg';

const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const hover = ref<AccessGrantsOrderBy>();

const nameSortData = computed((): { isHidden: boolean, isDesc: boolean } => {
    return {
        isHidden: !showArrow(AccessGrantsOrderBy.NAME),
        isDesc: isDesc(AccessGrantsOrderBy.NAME),
    };
});

const dateSortData = computed((): { isHidden: boolean, isDesc: boolean } => {
    return {
        isHidden: !showArrow(AccessGrantsOrderBy.CREATED_AT),
        isDesc: isDesc(AccessGrantsOrderBy.CREATED_AT),
    };
});

/**
 * Check if a heading is sorted in descending order.
 */
function isDesc(sortOrder: AccessGrantsOrderBy): boolean {
    return agStore.state.cursor.order === sortOrder && agStore.state.cursor.orderDirection === SortDirection.DESCENDING;
}

/**
 * Check if sorting arrow should be displayed.
 */
function showArrow(heading: AccessGrantsOrderBy): boolean {
    return agStore.state.cursor.order === heading || hover.value === heading;
}

/**
 * Set the heading of the current heading being hovered over.
 */
function mouseOver(heading: AccessGrantsOrderBy): void {
    hover.value = heading;
}

/**
 * Changes sorting parameters and fetches access grants.
 * @param sortBy
 */
async function sortBy(sortBy: AccessGrantsOrderBy): Promise<void> {
    if (sortBy === agStore.state.cursor.order) {
        agStore.toggleSortingDirection();
    } else {
        agStore.setSortingBy(sortBy);
        agStore.setSortingDirection(SortDirection.ASCENDING);
    }

    try {
        await agStore.getAccessGrants(agStore.state.page.currentPage, projectsStore.state.selectedProject.id);
    } catch (error) {
        await notify.error(`Unable to fetch accesses. ${error.message}`, AnalyticsErrorEventSource.ACCESS_GRANTS_PAGE);
    }
}

/**
 * Change the hover property to nothing on mouse leave.
 */
function mouseLeave(): void {
    hover.value = undefined;
}
</script>

<style scoped lang="scss">
.header {

    &__item {
        display: flex;
        align-items: center;
        gap: 5px;

        & > .invisible {
            visibility: hidden;
        }
    }

    &__functional {
        padding: 0 10px;
        position: relative;
        cursor: pointer;

        &__dropdown {
            position: absolute;
            top: 25px;
            right: 15px;
            background: #fff;
            box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
            border-radius: 6px;
            width: 255px;
            z-index: 100;

            &__item {
                display: flex;
                align-items: center;
                padding: 20px 25px;
                width: calc(100% - 50px);

                &:hover {
                    background-color: #f4f5f7;
                }
            }
        }
    }
}

.delete-confirmation {
    display: flex;
    flex-direction: column;
    gap: 5px;
    align-items: flex-start;
    width: 100%;

    &__options {
        display: flex;
        gap: 20px;
        align-items: center;

        &__item {
            display: flex;
            gap: 5px;
            align-items: center;

            &.yes:hover {
                color: var(--c-red-2);

                svg :deep(path) {
                    fill: var(--c-red-2);
                    stroke: var(--c-red-2);
                }
            }

            &.no:hover {
                color: var(--c-blue-3);

                svg :deep(path) {
                    fill: var(--c-blue-3);
                    stroke: var(--c-blue-3);
                }
            }
        }
    }
}

</style>

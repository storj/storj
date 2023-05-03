// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="table-wrapper">
        <table class="base-table" border="0" cellpadding="0" cellspacing="0">
            <thead>
                <tr>
                    <th v-if="selectable" class="icon select" @click.stop="() => emit('selectAllClicked')">
                        <v-table-checkbox v-if="showSelect" :value="selected" @selectClicked="() => emit('selectAllClicked')" />
                    </th>
                    <slot name="head" />
                </tr>
            </thead>
            <tbody>
                <slot name="body" />
            </tbody>
        </table>
        <div class="table-footer">
            <table-pagination
                class="table-footer__pagination"
                :total-page-count="totalPageCount"
                :total-items-count="totalItemsCount"
                :items-label="itemsLabel"
                :limit="limit"
                :on-page-change="onPageChange"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { PageChangeCallback } from '@/types/pagination';

import TablePagination from '@/components/common/TablePagination.vue';
import VTableCheckbox from '@/components/common/VTableCheckbox.vue';

const props = withDefaults(defineProps<{
    itemsLabel?: string,
    limit?: number,
    totalItemsCount?: number,
    onPageChange?: PageChangeCallback | null;
    totalPageCount?: number,
    selectable?: boolean,
    selected?: boolean,
    showSelect?: boolean,
}>(), {
    selectable: false,
    selected: false,
    showSelect: false,
    totalPageCount: 0,
    itemsLabel: 'items',
    limit: 0,
    totalItemsCount: 0,
    onPageChange: null,
});

const emit = defineEmits(['selectAllClicked']);
</script>

<style lang="scss">
.table-wrapper {
    background: #fff;
    border-radius: 12px;
}

.base-table {
    display: table;
    width: 100%;
    z-index: 997;
    border: 1px solid var(--c-grey-2);
    border-bottom: none;
    border-top-left-radius: 12px;
    border-top-right-radius: 12px;

    th {
        box-sizing: border-box;
        padding: 1.125rem;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
        font-family: 'font_regular', sans-serif;
    }

    thead {
        background: var(--c-block-gray);
        text-transform: uppercase;

        @media screen and (max-width: 550px) {
            display: none;
        }

        tr {
            height: 52px;
            font-size: 0.875rem;
            color: #6b7280;

            th.icon {
                border-top-left-radius: 12px;
            }
        }
    }

    tbody {

        th {
            font-family: 'font_regular', sans-serif;
            color: #111827;
            font-size: 1rem;
            border-top: solid 1px var(--c-grey-2);

            @media screen and (max-width: 550px) {
                border-top: none;
                border-bottom: solid 1px var(--c-grey-2);
            }
        }

        .data {
            font-family: 'font_bold', sans-serif;
        }

        .data ~ .data {
            font-family: 'font_regular', sans-serif;
        }
    }
}

.align-left {
    text-align: left;
}

.overflow-visible {
    overflow: visible !important;
}

.icon {
    width: 50px;
    overflow: visible !important;
    border-right: 1px solid var(--c-grey-2);
}

.table-footer {
    padding: 10px 20px;
    background: var(--c-grey-1);
    border: 1px solid var(--c-grey-2);
    border-bottom-left-radius: 12px;
    border-bottom-right-radius: 12px;

    @media screen and (max-width: 550px) {
        border-top: none;
    }
}

@media screen and (max-width: 970px) {

    tbody tr > .data p {
        max-width: 25rem;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }
}

@media screen and (max-width: 870px) {

    tbody tr > .data p {
        max-width: 20rem;
    }
}

@media screen and (max-width: 550px) {

    .select {
        display: none;
    }

    tbody tr > .data p {
        max-width: 25rem;
    }
}

@media screen and (max-width: 660px) {

    tbody tr > .data p {
        max-width: 15rem;
    }
}

@media screen and (max-width: 550px) {

    tbody tr > .data p {
        max-width: 15rem;
    }
}

@media screen and (max-width: 440px) {

    tbody tr > .data p {
        max-width: 10rem;
    }
}

@media screen and (max-width: 350px) {

    tbody tr > .data p {
        max-width: 8rem;
    }
}
</style>

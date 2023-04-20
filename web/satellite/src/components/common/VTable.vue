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
        <div v-if="totalPageCount > 0" class="table-footer">
            <span class="table-footer__label">{{ totalItemsCount }} {{ itemsLabel }}</span>
            <table-pagination
                v-if="totalPageCount > 1"
                :total-page-count="totalPageCount"
                :total-items-count="totalItemsCount"
                :limit="limit"
                :on-page-click-callback="onPageClickCallback"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { OnPageClickCallback } from '@/types/pagination';

import TablePagination from '@/components/common/TablePagination.vue';
import VTableCheckbox from '@/components/common/VTableCheckbox.vue';

const props = withDefaults(defineProps<{
    itemsLabel?: string,
    limit?: number,
    totalItemsCount?: number,
    onPageClickCallback?: OnPageClickCallback,
    totalPageCount?: number,
    selectable?: boolean,
    selected?: boolean,
    showSelect?: boolean,
}>(), {
    selectable: false,
    selected: false,
    showSelect: false,
    totalPageCount: 0,
    itemsLabel: '',
    limit: 0,
    totalItemsCount: 0,
    onPageClickCallback: () => () => Promise.resolve(),
});

const emit = defineEmits(['selectAllClicked']);
</script>

<style lang="scss">
.table-wrapper {
    background: #fff;
    box-shadow: 0 4px 2rem rgb(0 0 0 / 4%);
    border-radius: 12px;
}

.base-table {
    display: table;
    width: 100%;
    z-index: 997;

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
            border-top: solid 1px #e5e7eb;

            @media screen and (max-width: 550px) {
                border-top: none;
                border-bottom: solid 1px #e5e7eb;
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
    background: var(--c-grey-1);
}

.table-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 15px 20px;
    font-size: 1rem;
    line-height: 1.7rem;
    color: rgb(44 53 58 / 60%);
    border-top: solid 1px #e5e7eb;
    font-family: 'font-medium', sans-serif;

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

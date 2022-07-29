// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="table-wrapper">
        <table class="base-table" border="0" cellpadding="0" cellspacing="0">
            <thead>
                <tr>
                    <th v-if="selectable" class="icon" />
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
                :total-page-count="totalPageCount"
                :total-items-count="totalItemsCount"
                :limit="limit"
                :on-page-click-callback="onPageClickCallback"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import TablePagination from "@/components/common/TablePagination.vue";
import { OnPageClickCallback } from "@/types/pagination";

export type SelectableItem<T> = T & {
    isSelected: boolean;
}

// @vue/component
@Component({
    components: {
        TablePagination,
    }
})
export default class VTable extends Vue {
    @Prop({ default: false })
    public readonly selectable: boolean;
    @Prop({default: 0})
    private readonly totalPageCount: number;
    @Prop({default: "items"})
    private readonly itemsLabel: string;
    @Prop({default: 0})
    private readonly limit: number;
    @Prop({default: () => []})
    private readonly items: object[];
    @Prop({default: 0})
    private readonly totalItemsCount: number;
    @Prop({default: () => () => new Promise(() => false)})
    private readonly onPageClickCallback: OnPageClickCallback;
}
</script>

<style lang="scss">
.table-wrapper {
    background: #fff;
    box-shadow: 0 4px 32px rgb(0 0 0 / 4%);
    border-radius: 12px;
}

.base-table {
    display: table;
    width: 100%;
    z-index: 997;

    th {
        box-sizing: border-box;
        padding: 16px;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
        font-family: 'font_regular', sans-serif;
    }

    thead {
        background: var(--c-block-gray);
        text-transform: uppercase;

        tr {
            height: 52px;
            font-size: 12px;
            color: #6b7280;
        }
    }

    tbody {

        tr:hover {
            background: #e6edf7;
        }

        th {
            font-family: 'font_regular', sans-serif;
            color: #111827;
            font-size: 14px;
            border-top: solid 1px #e5e7eb;
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
    width: 5%;
    overflow: visible !important;
}

.table-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 15px 20px;
    font-size: 14px;
    line-height: 24px;
    color: rgb(44 53 58 / 60%);
    border-top: solid 1px #e5e7eb;
}
</style>

// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__date-item">
            <p class="sort-header-container__date-item__title">DATE</p>
        </div>
        <div class="sort-header-container__status-item">
            <p class="sort-header-container__status-item__title creation-date">STATUS</p>
        </div>
        <div class="sort-header-container__amount-item">
            <p class="sort-header-container__amount-item__title creation-date">AMOUNT</p>
        </div>
        <div class="sort-header-container__download-item">
            <p class="sort-header-container__download-item__title creation-date">DOWNLOAD</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { AccessGrantsOrderBy, OnHeaderClickCallback } from '@/types/accessGrants';
import { SortDirection } from '@/types/common';

// @vue/component
@Component({
    components: {
    },
})
export default class BillingHistoryHeader extends Vue {
    @Prop({default: () => () => new Promise(() => false)})
    private readonly onHeaderClickCallback: OnHeaderClickCallback;

    public AccessGrantsOrderBy = AccessGrantsOrderBy;

    public sortBy: AccessGrantsOrderBy = AccessGrantsOrderBy.NAME;
    public sortDirection: SortDirection = SortDirection.ASCENDING;

    /**
     * Used for arrow styling.
     */
    public get getSortDirection(): SortDirection {
        return this.sortDirection === SortDirection.DESCENDING ? SortDirection.ASCENDING : SortDirection.DESCENDING;
    }

    /**
    * Sets sorting kind if different from current.
    * If same, changes sort direction.
    * @param sortBy
    */

    public get areAccessGrantsSortedByName(): boolean {
        return this.sortBy === AccessGrantsOrderBy.NAME;
    }

   
}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        height: 40px;
        background-color: #fff;
        margin-top: 31px;
        padding: 16px 16px 0;
        border-radius: 8px 8px 0 0;

        &__name-item,
        &__status-item {
            padding-bottom: 13px;
            font-family: 'font_medium', sans-serif;
            font-style: normal;
            font-weight: 500;
            font-size: 12px;
            line-height: 16px;
            letter-spacing: 0.05em;
            text-transform: uppercase;
            color: #6b7280;
            width: 20%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;
        }

        &__amount-item {
            padding-bottom: 13px;
            font-family: 'font_medium', sans-serif;
            font-style: normal;
            font-weight: 500;
            font-size: 12px;
            line-height: 16px;
            letter-spacing: 0.05em;
            text-transform: uppercase;
            color: #6b7280;
            width: 20%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;
        }

        &__download-item {
            padding-bottom: 13px;
            font-family: 'font_medium', sans-serif;
            font-style: normal;
            font-weight: 500;
            font-size: 12px;
            line-height: 16px;
            margin-left: 0 0 0 53px;
            letter-spacing: 0.05em;
            text-transform: uppercase;
            color: #6b7280;
            width: 24%;
            display: flex;
            align-items: center;
            cursor: pointer;

            &__title {
                margin-left: 46px;
            }
        }

        &__date-item {
            padding-bottom: 13px;
            font-family: 'font_medium', sans-serif;
            font-style: normal;
            font-weight: 500;
            font-size: 12px;
            line-height: 16px;
            letter-spacing: 0.05em;
            text-transform: uppercase;
            color: #6b7280;
            width: 30%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;
        }

        @media only screen and (max-width: 768px) {

            .sort-header-container__status-item__title,
            .sort-header-container__status-item {
                display: none;
            }
        }
    }
</style>

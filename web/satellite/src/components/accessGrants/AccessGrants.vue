// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="access-grants">
        <div class="access-grants__title-area">
            <h2 class="access-grants__title-area__title">Access Grants</h2>
            <div class="access-grants__title-area__right" v-if="accessGrantsList.length">
                <VButton
                    v-if="selectedAccessGrantsAmount"
                    :label="deleteButtonLabel"
                    width="203px"
                    height="40px"
                    :on-press="onDeleteClick"
                    is-deletion="true"
                />
                <VButton
                    v-else
                    label="Create Access Grant +"
                    width="203px"
                    height="44px"
                    :on-press="onCreateClick"
                />
            </div>
        </div>
        <div v-if="accessGrantsList.length" class="access-grants-items">
            <SortAccessGrantsHeader :on-header-click-callback="onHeaderSectionClickCallback"/>
            <div class="access-grants-items__content">
                <VList
                    :data-set="accessGrantsList"
                    :item-component="itemComponent"
                    :on-item-click="toggleSelection"
                />
            </div>
            <VPagination
                v-if="totalPageCount > 1"
                class="pagination-area"
                ref="pagination"
                :total-page-count="totalPageCount"
                :on-page-click-callback="onPageClick"
            />
        </div>
        <EmptyState v-else />
        <ConfirmDeletePopup
            v-if="isDeleteClicked"
            @close="onClearSelection"
            @reset-pagination="resetPagination"
        />
        <router-view/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import AccessGrantsItem from '@/components/accessGrants/AccessGrantsItem.vue';
import ConfirmDeletePopup from '@/components/accessGrants/ConfirmDeletePopup.vue';
import EmptyState from '@/components/accessGrants/EmptyState.vue';
import SortAccessGrantsHeader from '@/components/accessGrants/SortingHeader.vue';
import VButton from '@/components/common/VButton.vue';
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant, AccessGrantsOrderBy } from '@/types/accessGrants';
import { SortDirection } from '@/types/common';

const {
    FETCH,
    TOGGLE_SELECTION,
    CLEAR,
    CLEAR_SELECTION,
    SET_SEARCH_QUERY,
    SET_SORT_BY,
    SET_SORT_DIRECTION,
} = ACCESS_GRANTS_ACTIONS;

declare interface ResetPagination {
    resetPageIndex(): void;
}

@Component({
    components: {
        EmptyState,
        SortAccessGrantsHeader,
        VList,
        VPagination,
        VButton,
        ConfirmDeletePopup,
    },
})
export default class AccessGrants extends Vue {
    private FIRST_PAGE = 1;

    /**
     * Indicates if delete confirmation state should appear.
     */
    private isDeleteClicked: boolean = false;

    public $refs!: {
        pagination: HTMLElement & ResetPagination;
    };

    /**
     * Lifecycle hook after initial render where list of existing access grants is fetched.
     */
    public async mounted(): Promise<void> {
        await this.$store.dispatch(FETCH, 1);
    }

    /**
     * Lifecycle hook before component destruction.
     * Clears existing access grants selection.
     */
    public beforeDestroy(): void {
        this.onClearSelection();
    }

    /**
     * Toggles access grant selection.
     * @param accessGrant
     */
    public async toggleSelection(accessGrant: AccessGrant): Promise<void> {
        await this.$store.dispatch(TOGGLE_SELECTION, accessGrant);
    }

    /**
     * Fetches access grants page by clicked index.
     * @param index
     */
    public async onPageClick(index: number): Promise<void> {
        try {
            await this.$store.dispatch(FETCH, index);
        } catch (error) {
            await this.$notify.error(`Unable to fetch Access Grants. ${error.message}`);
        }
    }

    /**
     * Used for sorting.
     * @param sortBy
     * @param sortDirection
     */
    public async onHeaderSectionClickCallback(sortBy: AccessGrantsOrderBy, sortDirection: SortDirection): Promise<void> {
        await this.$store.dispatch(SET_SORT_BY, sortBy);
        await this.$store.dispatch(SET_SORT_DIRECTION, sortDirection);
        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch Access Grants. ${error.message}`);
        }

        if (this.totalPageCount > 1) {
            this.resetPagination();
        }
    }

    /**
     * Resets pagination to default state.
     */
    public resetPagination(): void {
        if (this.totalPageCount > 1) {
            this.$refs.pagination.resetPageIndex();
        }
    }

    /**
     * Starts create access grant flow.
     */
    public onCreateClick(): void {
        this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).with(RouteConfig.NameStep).path);
    }

    /**
     * Holds on button click login for deleting access grant process.
     */
    public onDeleteClick(): void {
        this.isDeleteClicked = true;
    }

    /**
     * Clears access grants selection.
     */
    public async onClearSelection(): Promise<void> {
        await this.$store.dispatch(CLEAR_SELECTION);
        this.isDeleteClicked = false;
    }

    /**
     * Returns delete access grants button label.
     */
    public get deleteButtonLabel(): string {
        return `Remove Selected (${this.selectedAccessGrantsAmount})`;
    }

    /**
     * Returns access grants pages count from store.
     */
    public get totalPageCount(): number {
        return this.$store.state.accessGrantsModule.page.pageCount;
    }

    /**
     * Returns AccessGrant item component.
     */
    public get itemComponent() {
        return AccessGrantsItem;
    }

    /**
     * Returns access grants from store.
     */
    public get accessGrantsList(): AccessGrant[] {
        return this.$store.state.accessGrantsModule.page.accessGrants;
    }

    /**
     * Returns selected access grants IDs amount from store.
     */
    public get selectedAccessGrantsAmount(): number {
        return this.$store.state.accessGrantsModule.selectedAccessGrantsIds.length;
    }
}
</script>

<style scoped lang="scss">
    .access-grants {
        position: relative;
        padding: 40px 30px 55px 30px;
        font-family: 'font_regular', sans-serif;

        &__title-area {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #263549;
                margin: 0;
            }
        }

        .access-grants-items {
            position: relative;

            &__content {
                background-color: #fff;
                display: flex;
                flex-direction: column;
                width: calc(100% - 32px);
                justify-content: flex-start;
                padding: 16px;
                border-radius: 0 0 8px 8px;
            }
        }
    }
</style>

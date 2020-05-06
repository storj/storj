// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="api-keys-area">
        <h1 class="api-keys-area__title" v-if="!isEmpty">API Keys</h1>
        <div class="api-keys-area__container">
            <ApiKeysCreationPopup
                @closePopup="closeNewApiKeyPopup"
                @showCopyPopup="showCopyApiKeyPopup"
                :is-popup-shown="isNewApiKeyPopupShown"
            />
            <ApiKeysCopyPopup
                :is-popup-shown="isCopyApiKeyPopupShown"
                :api-key-secret="apiKeySecret"
                @closePopup="closeCopyNewApiKeyPopup"
            />
            <div v-if="isHeaderShown" class="api-keys-header">
                <VHeader
                    ref="headerComponent"
                    placeholder="API Key"
                    :search="onSearchQueryCallback">
                    <div class="header-default-state" v-if="isDefaultHeaderState">
                        <VButton
                            class="button"
                            label="+ Create API Key"
                            width="180px"
                            height="48px"
                            :on-press="onCreateApiKeyClick"
                        />
                    </div>
                    <div class="header-selected-api-keys" v-if="areApiKeysSelected">
                        <VButton
                            class="button deletion"
                            label="Delete"
                            width="122px"
                            height="48px"
                            :on-press="onFirstDeleteClick"
                        />
                        <VButton
                            class="button"
                            label="Cancel"
                            width="122px"
                            height="48px"
                            is-white="true"
                            :on-press="onClearSelection"
                        />
                        <span class="header-selected-api-keys__info-text"><b>{{selectedAPIKeysCount}}</b> API Keys selected</span>
                    </div>
                    <div class="header-after-delete-click" v-if="areSelectedApiKeysBeingDeleted">
                        <span class="header-after-delete-click__confirmation-label">Are you sure you want to delete <b>{{selectedAPIKeysCount}}</b> {{apiKeyCountTitle}} ?</span>
                        <div class="header-after-delete-click__button-area">
                            <VButton
                                class="button deletion"
                                label="Delete"
                                width="122px"
                                height="48px"
                                :on-press="onDelete"
                            />
                            <VButton
                                class="button"
                                label="Cancel"
                                width="122px"
                                height="48px"
                                is-white="true"
                                :on-press="onClearSelection"
                            />
                        </div>
                    </div>
                </VHeader>
                <div class="blur-content" v-if="isDeleteClicked"></div>
                <div class="blur-search" v-if="isDeleteClicked"></div>
            </div>
            <div v-if="!isEmpty" class="api-keys-items">
                <SortingHeader :on-header-click-callback="onHeaderSectionClickCallback"/>
                <div class="api-keys-items__content">
                    <VList
                        :data-set="apiKeyList"
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
            <div class="empty-search-result-area" v-if="isEmptySearchResultShown">
                <h1 class="empty-search-result-area__title">No results found</h1>
                <EmptySearchResultIcon class="empty-search-result-area__image"/>
            </div>
            <NoApiKeysArea
                :on-button-click="onCreateApiKeyClick"
                v-if="isEmptyStateShown"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';
import NoApiKeysArea from '@/components/apiKeys/NoApiKeysArea.vue';
import SortingHeader from '@/components/apiKeys/SortingHeader.vue';
import VButton from '@/components/common/VButton.vue';
import VHeader from '@/components/common/VHeader.vue';
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';

import EmptySearchResultIcon from '@/../static/images/common/emptySearchResult.svg';

import { ApiKey, ApiKeyOrderBy } from '@/types/apiKeys';
import { SortDirection } from '@/types/common';
import { API_KEYS_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

import ApiKeysCopyPopup from './ApiKeysCopyPopup.vue';
import ApiKeysCreationPopup from './ApiKeysCreationPopup.vue';

// header state depends on api key selection state
enum HeaderState {
    DEFAULT = 0,
    ON_SELECT,
}

const {
    FETCH,
    DELETE,
    TOGGLE_SELECTION,
    CLEAR,
    CLEAR_SELECTION,
    SET_SEARCH_QUERY,
    SET_SORT_BY,
    SET_SORT_DIRECTION,
} = API_KEYS_ACTIONS;

declare interface ResetPagination {
    resetPageIndex(): void;
}

@Component({
    components: {
        NoApiKeysArea,
        VList,
        VHeader,
        ApiKeysItem,
        VButton,
        ApiKeysCreationPopup,
        ApiKeysCopyPopup,
        VPagination,
        SortingHeader,
        EmptySearchResultIcon,
    },
})
export default class ApiKeysArea extends Vue {
    private FIRST_PAGE = 1;
    /**
     * Indicates if delete confirmation state should appear.
     */
    private isDeleteClicked: boolean = false;
    /**
     * Indicates if api key name input state should appear.
     */
    private isNewApiKeyPopupShown: boolean = false;
    /**
     * Indicates if copy api key state should appear.
     * Should only appear once
     */
    private isCopyApiKeyPopupShown: boolean = false;
    private apiKeySecret: string = '';

    public $refs!: {
        pagination: HTMLElement & ResetPagination;
    };

    /**
     * Lifecycle hook after initial render where list of existing api keys is fetched.
     */
    public async mounted(): Promise<void> {
        await this.$store.dispatch(FETCH, 1);
        this.$segment.track(SegmentEvent.API_KEYS_VIEWED, {
            project_id: this.$store.getters.selectedProject.id,
            api_keys_count: this.selectedAPIKeysCount,
        });
    }

    /**
     * Lifecycle hook before component destruction.
     * Clears existing api keys selection and search.
     */
    public async beforeDestroy(): Promise<void> {
        this.onClearSelection();
        await this.$store.dispatch(SET_SEARCH_QUERY, '');
    }

    /**
     * toggles api key selection.
     * @param apiKey
     */
    public async toggleSelection(apiKey: ApiKey): Promise<void> {
        await this.$store.dispatch(TOGGLE_SELECTION, apiKey);
    }

    public onCreateApiKeyClick(): void {
        this.isNewApiKeyPopupShown = true;
    }

    public onFirstDeleteClick(): void {
        this.isDeleteClicked = true;
    }

    public onClearSelection(): void {
        this.$store.dispatch(CLEAR_SELECTION);
        this.isDeleteClicked = false;
    }

    public closeNewApiKeyPopup(): void {
        this.isNewApiKeyPopupShown = false;
    }

    public showCopyApiKeyPopup(secret: string): void {
        this.isCopyApiKeyPopupShown = true;
        this.apiKeySecret = secret;
    }

    public closeCopyNewApiKeyPopup(): void {
        this.isCopyApiKeyPopupShown = false;
    }

    /**
     * Deletes selected api keys, fetches updated list and changes area state to default.
     */
    public async onDelete(): Promise<void> {
        try {
            await this.$store.dispatch(DELETE);
            await this.$notify.success(`API keys deleted successfully`);
            this.$segment.track(SegmentEvent.API_KEY_DELETED, {
                project_id: this.$store.getters.selectedProject.id,
            });
        } catch (error) {
            await this.$notify.error(error.message);
        }

        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch API keys. ${error.message}`);
        }

        this.isDeleteClicked = false;

        if (this.totalPageCount > 1) {
            this.$refs.pagination.resetPageIndex();
        }
    }

    public get itemComponent() {
        return ApiKeysItem;
    }

    /**
     * Returns api keys from store.
     */
    public get apiKeyList(): ApiKey[] {
        return this.$store.getters.apiKeys;
    }

    /**
     * Returns api keys pages count from store.
     */
    public get totalPageCount(): number {
        return this.$store.state.apiKeysModule.page.pageCount;
    }

    /**
     * Returns api keys label depends on api keys count.
     */
    public get apiKeyCountTitle(): string {
        return this.selectedAPIKeysCount === 1 ? 'api key' : 'api keys';
    }

    /**
     * Indicates if no api keys in store.
     */
    public get isEmpty(): boolean {
        return this.$store.getters.apiKeys.length === 0;
    }

    public get hasSearchQuery(): boolean {
        return this.$store.state.apiKeysModule.cursor.search;
    }

    public get selectedAPIKeysCount(): number {
        return this.$store.state.apiKeysModule.selectedApiKeysIds.length;
    }

    public get headerState(): number {
        return this.selectedAPIKeysCount > 0 ? HeaderState.ON_SELECT : HeaderState.DEFAULT;
    }

    public get isHeaderShown(): boolean {
        return !this.isEmpty || this.hasSearchQuery;
    }

    public get isDefaultHeaderState(): boolean {
        return this.headerState === HeaderState.DEFAULT;
    }

    public get areApiKeysSelected(): boolean {
        return this.headerState === HeaderState.ON_SELECT && !this.isDeleteClicked;
    }

    public get areSelectedApiKeysBeingDeleted(): boolean {
        return this.headerState === HeaderState.ON_SELECT && this.isDeleteClicked;
    }

    public get isEmptySearchResultShown(): boolean {
        return this.isEmpty && this.hasSearchQuery;
    }

    public get isEmptyStateShown(): boolean {
        return this.isEmpty && !this.isNewApiKeyPopupShown && !this.hasSearchQuery;
    }

    /**
     * Fetches api keys page by clicked index.
     * @param index
     */
    public async onPageClick(index: number): Promise<void> {
        try {
            await this.$store.dispatch(FETCH, index);
        } catch (error) {
            await this.$notify.error(`Unable to fetch API keys. ${error.message}`);
        }
    }

    /**
     * Used for sorting.
     * @param sortBy
     * @param sortDirection
     */
    public async onHeaderSectionClickCallback(sortBy: ApiKeyOrderBy, sortDirection: SortDirection): Promise<void> {
        await this.$store.dispatch(SET_SORT_BY, sortBy);
        await this.$store.dispatch(SET_SORT_DIRECTION, sortDirection);
        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch API keys. ${error.message}`);
        }

        if (this.totalPageCount > 1) {
            this.$refs.pagination.resetPageIndex();
        }
    }

    /**
     * Sets api keys search query and then fetches depends on it.
     * @param query
     */
    public async onSearchQueryCallback(query: string): Promise<void> {
        await this.$store.dispatch(SET_SEARCH_QUERY, query);
        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch API keys. ${error.message}`);
        }

        if (this.totalPageCount > 1) {
            this.$refs.pagination.resetPageIndex();
        }
    }
}
</script>

<style scoped lang="scss">
    .api-keys-area {
        position: relative;
        padding: 40px 65px 55px 65px;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            color: #263549;
            margin: 0;
        }

        .api-keys-header {
            width: 100%;
            position: relative;

            .blur-content {
                position: absolute;
                top: 100%;
                left: 0;
                background-color: #f5f6fa;
                width: 100%;
                height: 70vh;
                z-index: 100;
                opacity: 0.3;
            }

            .blur-search {
                position: absolute;
                bottom: 0;
                right: 0;
                width: 540px;
                height: 56px;
                z-index: 100;
                opacity: 0.3;
                background-color: #f5f6fa;
            }
        }

        .api-keys-items {
            position: relative;

            &__content {
                display: flex;
                flex-direction: column;
                width: 100%;
                justify-content: flex-start;
            }
        }
    }

    .empty-search-result-area {
        display: flex;
        align-items: center;
        justify-content: center;
        flex-direction: column;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            margin-top: 104px;
        }

        &__image {
            margin-top: 40px;
        }
    }

    .pagination-area {
        margin-left: -25px;
        padding-bottom: 15px;
    }

    .header-default-state,
    .header-selected-api-keys {
        display: flex;
        align-items: center;

        &__info-text {
            margin-left: 25px;
            line-height: 48px;
        }
    }

    .header-selected-api-keys {

        .deletion {
            margin-right: 12px;
        }
    }

    .header-after-delete-click {
        display: flex;
        flex-direction: column;
        margin-top: 2px;

        &__confirmation-label {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 28px;
        }

        &__button-area {
            display: flex;
            margin-top: 4px;

            .button {
                margin-top: 2px;
            }

            .deletion {
                margin: 3px 12px 0 0;
            }
        }
    }

    .container.deletion {
        background-color: #ff4f4d;

        &.label {
            color: #fff;
        }

        &:hover {
            background-color: #de3e3d;
            box-shadow: none;
        }
    }

    .collapsed {
        margin-top: 0 !important;
        padding-top: 0 !important;
    }

    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        width: 0;
    }

    @media screen and (max-width: 1024px) {

        .api-keys-area {
            padding: 40px 40px 55px 40px;
        }
    }
</style>

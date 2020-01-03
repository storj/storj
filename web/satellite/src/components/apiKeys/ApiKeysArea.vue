// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="api-keys-area">
        <h1 class="api-keys-area__title">API Keys</h1>
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
                            label="+Create API Key"
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
                <p class="api-keys-items__additional-info">Want to give limited access? <b>Use API Keys.</b></p>
            </div>
            <div class="empty-search-result-area" v-if="isEmptySearchResultShown">
                <h1 class="empty-search-result-area__title">No results found</h1>
                <EmptySearchResultIcon class="empty-search-result-area__image"/>
            </div>
            <EmptyState
                :on-button-click="onCreateApiKeyClick"
                v-if="isEmptyStateShown"
                main-title="Let's create your first API Key"
                additional-text="<p>API keys give access to the project allowing you to create buckets, upload files, and read them. Once you’ve created an API key, you’re ready to interact with the network through our Uplink CLI.</p>"
                :image-source="emptyImage"
                button-label="Create an API Key"
                is-button-shown="true"
            />
        </div>
    </div>
</template>

<script lang="ts">
import VueClipboards from 'vue-clipboards';
import { Component, Vue } from 'vue-property-decorator';

import ApiKeysItem from '@/components/apiKeys/ApiKeysItem.vue';
import SortingHeader from '@/components/apiKeys/SortingHeader.vue';
import EmptyState from '@/components/common/EmptyStateArea.vue';
import VButton from '@/components/common/VButton.vue';
import VHeader from '@/components/common/VHeader.vue';
import VList from '@/components/common/VList.vue';
import VPagination from '@/components/common/VPagination.vue';

import EmptySearchResultIcon from '@/../static/images/common/emptySearchResult.svg';

import { ApiKey, ApiKeyOrderBy } from '@/types/apiKeys';
import { SortDirection } from '@/types/common';
import { API_KEYS_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

import ApiKeysCopyPopup from './ApiKeysCopyPopup.vue';
import ApiKeysCreationPopup from './ApiKeysCreationPopup.vue';

Vue.use(VueClipboards);

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
        VList,
        EmptyState,
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
    public emptyImage: string = EMPTY_STATE_IMAGES.API_KEY;
    private FIRST_PAGE = 1;
    private isDeleteClicked: boolean = false;
    private isNewApiKeyPopupShown: boolean = false;
    private isCopyApiKeyPopupShown: boolean = false;
    private apiKeySecret: string = '';

    public $refs!: {
        pagination: HTMLElement & ResetPagination;
    };

    public async mounted(): Promise<void> {
        await this.$store.dispatch(FETCH, 1);
        this.$segment.track(SegmentEvent.API_KEYS_VIEWED, {
            project_id: this.$store.getters.selectedProject.id,
            api_keys_count: this.selectedAPIKeysCount,
        });
    }

    public async beforeDestroy(): Promise<void> {
        this.onClearSelection();
        await this.$store.dispatch(SET_SEARCH_QUERY, '');
    }

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
            await this.notifyFetchError(error);
        }

        this.isDeleteClicked = false;

        if (this.totalPageCount > 1) {
            this.$refs.pagination.resetPageIndex();
        }
    }

    public get itemComponent() {
        return ApiKeysItem;
    }

    public get apiKeyList(): ApiKey[] {
        return this.$store.getters.apiKeys;
    }

    public get totalPageCount(): number {
        return this.$store.state.apiKeysModule.page.pageCount;
    }

    public get apiKeyCountTitle(): string {
        if (this.selectedAPIKeysCount === 1) {
            return 'api key';
        }

        return 'api keys';
    }

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
        if (this.selectedAPIKeysCount > 0) {
            return HeaderState.ON_SELECT;
        }

        return HeaderState.DEFAULT;
    }

    public get isHeaderShown(): boolean {
        return !this.isEmpty || this.hasSearchQuery;
    }

    public get isDefaultHeaderState(): boolean {
        return this.headerState === 0;
    }

    public get areApiKeysSelected(): boolean {
        return this.headerState === 1 && !this.isDeleteClicked;
    }

    public get areSelectedApiKeysBeingDeleted(): boolean {
        return this.headerState === 1 && this.isDeleteClicked;
    }

    public get isEmptySearchResultShown(): boolean {
        return this.isEmpty && this.hasSearchQuery;
    }

    public get isEmptyStateShown(): boolean {
        return this.isEmpty && !this.isNewApiKeyPopupShown && !this.hasSearchQuery;
    }

    public async onPageClick(index: number): Promise<void> {
        try {
            await this.$store.dispatch(FETCH, index);
        } catch (error) {
            await this.notifyFetchError(error);
        }
    }

    public async onHeaderSectionClickCallback(sortBy: ApiKeyOrderBy, sortDirection: SortDirection): Promise<void> {
        await this.$store.dispatch(SET_SORT_BY, sortBy);
        await this.$store.dispatch(SET_SORT_DIRECTION, sortDirection);
        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.notifyFetchError(error);
        }

        if (this.totalPageCount > 1) {
            this.$refs.pagination.resetPageIndex();
        }
    }

    public async onSearchQueryCallback(query: string): Promise<void> {
        await this.$store.dispatch(SET_SEARCH_QUERY, query);
        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.notifyFetchError(error);
        }

        if (this.totalPageCount > 1) {
            this.$refs.pagination.resetPageIndex();
        }
    }

    public async notifyFetchError(error: Error): Promise<void> {
        await this.$notify.error(`Unable to fetch API keys. ${error.message}`);
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
            user-select: none;
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
                width: 602px;
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

            &__additional-info {
                font-size: 16px;
                color: #afb7c1;
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

// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.
<template>
    <div class="access-grants">
        <div v-if="!isNewAccessGrantFlow" class="access-grants__title-area">
            <h2 class="access-grants__title-area__title" aria-roledescription="title">Access Grants</h2>
            <div v-if="accessGrantsList.length" class="access-grants__title-area__right">
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
                    :is-disabled="areGrantsFetching"
                />
            </div>
        </div>
        <div v-if="isNewAccessGrantFlow" class="access-grants__new-title-area">
            <h2 class="access-grants__title-area__title" aria-roledescription="title">Access Management</h2>
            <div class="access-grants__title-area__title-subtext" aria-roledescription="title">Create encryption keys to setup permissions to access your objects.</div>
        </div>
        <div v-if="isNewAccessGrantFlow" class="access-grants__flows-area">
            <div class="access-grants__flows-area__access-grant">
                <div class="access-grants__flows-area__icon-container">
                    <AccessGrantsIcon />
                </div>
                <div class="access-grants__flows-area__title">Access Grant</div>
                <div class="access-grants__flows-area__summary">Gives access through native clients such as uplink, libuplink, associate libraries, and bindings. </div>
                <div class="access-grants__flows-area__button-container">
                    <VButton
                        label="Learn More"
                        width="auto"
                        height="30px"
                        is-transparent="true"
                        font-size="13px"
                        class="access-grants__flows-area__learn-button"
                    />
                    <VButton
                        label="Create Access Grant"
                        font-size="13px"
                        width="auto"
                        height="30px"
                        class="access-grants__flows-area__create-button"
                    />
                </div>
            </div>
            <div class="access-grants__flows-area__s3-credentials">
                <div class="access-grants__flows-area__icon-container">
                    <S3Icon />
                </div>
                <div class="access-grants__flows-area__title">S3 Credentials</div>
                <div class="access-grants__flows-area__summary">Gives access through S3 compatible tools and services via our hosted Gateway MT.</div>
                <br>
                <div class="access-grants__flows-area__button-container">
                    <VButton
                        label="Learn More"
                        width="auto"
                        height="30px"
                        is-transparent="true"
                        font-size="13px"
                        class="access-grants__flows-area__learn-button"
                    />
                    <VButton
                        label="Create Access Grant"
                        font-size="13px"
                        width="auto"
                        height="30px"
                        class="access-grants__flows-area__create-button"
                    />
                </div>
            </div>
            <div class="access-grants__flows-area__cli-credentials">
                <div class="access-grants__flows-area__icon-container">
                    <CLIIcon />
                </div>
                <div class="access-grants__flows-area__title">CLI Access</div>
                <div class="access-grants__flows-area__summary">Creates Satellite Adress and API Key to run the “setup” in Command Line Interface. </div>
                <br>
                <div class="access-grants__flows-area__button-container">
                    <VButton
                        label="Learn More"
                        width="auto"
                        height="30px"
                        is-transparent="true"
                        font-size="13px"
                        class="access-grants__flows-area__learn-button"
                    />
                    <VButton
                        label="Create Access Grant"
                        font-size="13px"
                        width="auto"
                        height="30px"
                        class="access-grants__flows-area__create-button"
                    />
                </div>
            </div>
        </div>
        <div v-if="isNewAccessGrantFlow"> 
            <div class="access-grants__header-container">
                <h3 class="access-grants__header-container__title">My Accesses</h3>
                <div class="access-grants__header-container__divider" />
                <VHeader 
                    class="access-header-component"
                    placeholder="Accesses"
                    :search="fetch"
                    style-type="access"
                />
            </div>
            <VLoader v-if="areGrantsFetching" width="100px" height="100px" class="grants-loader" />
            <div v-if="accessGrantsList.length && !areGrantsFetching" class="access-grants-items2">
                <SortAccessGrantsHeader2
                    :on-header-click-callback="onHeaderSectionClickCallback"
                />
                <div class="access-grants-items2__content">
                    <VList
                        :data-set="accessGrantsList"
                        :item-component="itemComponent2"
                    />
                </div>
                <div class="access-grants-items2__footer">
                    <span class="access-grants-items2__footer__total-accesses">
                        {{ accessGrantsList.length }} Access Grants
                    </span>
                    <VPagination
                        v-if="totalPageCount > 1"
                        ref="pagination"
                        class="access-grants-items2__footer__pagination-area"
                        :total-page-count="totalPageCount"
                        :on-page-click-callback="onPageClick"
                    />
                </div>
            </div>
            <div
                v-if="!accessGrantsList.length && !areGrantsFetching"
                class="access-grants-items2__empty-state"
            > 
                <span class="access-grants-items2__empty-state__text">
                    No Results Found
                </span>
            </div>
            <ConfirmDeletePopup
                v-if="isDeleteClicked"
                @close="onClearSelection"
                @reset-pagination="resetPagination"
            />
        </div>
        <div v-if="!isNewAccessGrantFlow">
            <VLoader v-if="areGrantsFetching" width="100px" height="100px" class="grants-loader" />
            <div v-if="accessGrantsList.length && !areGrantsFetching" class="access-grants-items">
                <SortAccessGrantsHeader :on-header-click-callback="onHeaderSectionClickCallback" />
                <div class="access-grants-items__content">
                    <VList
                        :data-set="accessGrantsList"
                        :item-component="itemComponent"
                        :on-item-click="toggleSelection"
                    />
                </div>
                <VPagination
                    v-if="totalPageCount > 1"
                    ref="pagination"
                    class="pagination-area"
                    :total-page-count="totalPageCount"
                    :on-page-click-callback="onPageClick"
                />
            </div>
            <EmptyState v-if="!accessGrantsList.length && !areGrantsFetching" />
            <ConfirmDeletePopup
                v-if="isDeleteClicked"
                @close="onClearSelection"
                @reset-pagination="resetPagination"
            />
        </div>
        <router-view />
    </div>
</template>
<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { MetaUtils } from '@/utils/meta';

import AccessGrantsItem from '@/components/accessGrants/AccessGrantsItem.vue';
import AccessGrantsItem2 from '@/components/accessGrants/AccessGrantsItem2.vue';
import ConfirmDeletePopup from '@/components/accessGrants/ConfirmDeletePopup.vue';
import EmptyState from '@/components/accessGrants/EmptyState.vue';
import SortAccessGrantsHeader from '@/components/accessGrants/SortingHeader.vue';
import SortAccessGrantsHeader2 from '@/components/accessGrants/SortingHeader2.vue';
import VButton from '@/components/common/VButton.vue';
import VList from '@/components/common/VList.vue';
import VLoader from '@/components/common/VLoader.vue';
import VHeader from '@/components/common/VHeader.vue';
import VPagination from '@/components/common/VPagination.vue';
import AccessGrantsIcon from '@/../static/images/accessGrants/accessGrantsIcon.svg';
import CLIIcon from '@/../static/images/accessGrants/cli.svg';
import S3Icon from '@/../static/images/accessGrants/s3.svg';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant, AccessGrantsOrderBy } from '@/types/accessGrants';
import { SortDirection } from '@/types/common';

const {
    FETCH,
    TOGGLE_SELECTION,
    CLEAR_SELECTION,
    SET_SORT_BY,
    SET_SORT_DIRECTION,
    SET_SEARCH_QUERY,
} = ACCESS_GRANTS_ACTIONS;
declare interface ResetPagination {
    resetPageIndex(): void;
}
// @vue/component
@Component({
    components: {
        AccessGrantsIcon,
        CLIIcon,
        EmptyState,
        S3Icon,
        SortAccessGrantsHeader,
        SortAccessGrantsHeader2,
        VList,
        VPagination,
        VButton,
        ConfirmDeletePopup,
        VLoader,
        VHeader,
    },
})
export default class AccessGrants extends Vue {
    private FIRST_PAGE = 1;
    private isDeleteClicked = false;
    public areGrantsFetching = true;
    public $refs!: {
        pagination: HTMLElement & ResetPagination;
    };

    /**
     * Indicates if navigation side bar is hidden.
     */
    public get isNewAccessGrantFlow(): boolean {
        const isNewAccessGrantFlow = MetaUtils.getMetaContent('new-access-grant-flow');
        return isNewAccessGrantFlow === "true";
    }
    /**
     * Lifecycle hook after initial render where list of existing access grants is fetched.
     */
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch(FETCH, this.FIRST_PAGE);
            this.areGrantsFetching = false;
        } catch (error) {
            await this.$notify.error(`Unable to fetch Access Grants. ${error.message}`);
        }
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
     * Fetches Access records by name depending on search query.
     */
    public async fetch(searchQuery: string): Promise<void> {
        await this.$store.dispatch(SET_SEARCH_QUERY, searchQuery);

        try {
            await this.$store.dispatch(FETCH, 1);
        } catch (error) {
            await this.$notify.error(`Unable to fetch accesses: ${error.message}`);
        }
    }
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
    public get itemComponent(): typeof AccessGrantsItem {
        return AccessGrantsItem;
    }
    /**
     * Returns AccessGrant2 item component.
     */
    public get itemComponent2(): typeof AccessGrantsItem2 {
        return AccessGrantsItem2;
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
    @mixin grant-flow-card {
        display: inline-block;
        padding: 28px;
        width: 26%;
        height: 167px;
        background: #fff;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        border-radius: 10px;
    }

    .access-grants {
        position: relative;
        height: calc(100% - 95px);
        padding: 40px 30px 55px;
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

            &__title-subtext {
                margin-top: 10px;
                font-style: normal;
                font-weight: 400;
                font-size: 16px;
                line-height: 24px;
            }
        }

        .access-grants__flows-area {
            text-align: center;
            display: flex;
            -webkit-box-align: center;
            align-items: center;
            -webkit-box-pack: justify;
            justify-content: space-between;
            margin-top: 20px;

            &__access-grant,
            &__s3-credentials,
            &__cli-credentials {
                @include grant-flow-card;
            }

            &__learn-button {
                margin-right: 2%;
                padding: 0 10px;
            }

            &__create-button {
                padding: 0 10px;
            }

            &__button-container {
                display: flex;
                margin-top: 10px;
            }

            &__summary {
                font-style: normal;
                font-weight: 400;
                font-size: 14px;
                line-height: 20px;
                overflow-wrap: break-word;
                text-align: left;
                margin-top: 5px;
            }

            &__title {
                text-align: left;
                margin-top: 15px;
                font-family: 'font_bold', sans-serif;
            }

            &__icon-container {
                text-align: left;
                height: 38px;
                margin-top: -10px;
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

        .access-grants-items2 {
            position: relative;

            &__content {
                background-color: #fff;
                display: flex;
                flex-direction: column;
                justify-content: flex-start;
            }

            &__footer {
                background-color: #fff;
                display: flex;
                justify-content: space-between;
                align-items: center;
                height: 80px;
                width: 100%;
                border: 1px solid #e5e7eb;
                border-radius: 0 0 8px 8px;

                &__total-accesses {
                    height: 20px;
                    padding-left: 20px;
                    margin-left: 18px;
                    color: #2c353a;
                }

                &__pagination-area {
                    padding-right: 20px;
                    margin-bottom: 25px;
                }
            }

            &__empty-state {
                height: 75px;
                width: auto;
                background: white;
                border-radius: 6px;
                margin-top: 10px;
                border: 1px solid #dadfe7;
                display: flex;
                justify-content: center;

                &__text {
                    font-family: sans-serif;
                    font-size: 40px;
                    font-weight: 700;
                    margin: auto 0;
                }
            }
        }

        .access-grants__header-container {

            &__header-container {
                height: 90px;
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                margin-top: 20px;
            }

            &__divider {
                height: 1px;
                width: auto;
                background-color: #dadfe7;
                margin-top: 10px;
            }

            &__access-header-component {
                height: 55px !important;
                margin-top: 15px;
            }
        }
    }

    .grants-loader {
        margin-top: 50px;
    }
</style>
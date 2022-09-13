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
                    <a
                        href="https://docs.storj.io/dcs/concepts/access/access-grants"
                        target="_blank"
                        rel="noopener noreferrer"
                        @click="trackPageVisit('https://docs.storj.io/dcs/concepts/access/access-grants')"
                    >
                        <VButton
                            label="Learn More"
                            width="auto"
                            height="30px"
                            is-transparent="true"
                            font-size="13px"
                            class="access-grants__flows-area__learn-button"
                        />
                    </a>
                    <VButton
                        label="Create Access Grant"
                        font-size="13px"
                        width="auto"
                        height="30px"
                        class="access-grants__flows-area__create-button"
                        :on-press="accessGrantClick"
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
                    <a
                        href="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway"
                        target="_blank"
                        rel="noopener noreferrer"
                        @click="trackPageVisit('https://docs.storj.io/dcs/api-reference/s3-compatible-gateway')"
                    >
                        <VButton
                            label="Learn More"
                            width="auto"
                            height="30px"
                            is-transparent="true"
                            font-size="13px"
                            class="access-grants__flows-area__learn-button"
                        />
                    </a>
                    <VButton
                        label="Create S3 Credentials"
                        font-size="13px"
                        width="auto"
                        height="30px"
                        class="access-grants__flows-area__create-button"
                        :on-press="s3Click"
                    />
                </div>
            </div>
            <div class="access-grants__flows-area__cli-credentials">
                <div class="access-grants__flows-area__icon-container">
                    <CLIIcon />
                </div>
                <div class="access-grants__flows-area__title">API Key</div>
                <div class="access-grants__flows-area__summary">Use it for generating S3 credentials and access grants programatically. </div>
                <br>
                <div class="access-grants__flows-area__button-container">
                    <a
                        href="https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token"
                        target="_blank"
                        rel="noopener noreferrer"
                        @click="trackPageVisit('https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/generate-access-grants-and-tokens/generate-a-token')"
                    >
                        <VButton
                            label="Learn More"
                            width="auto"
                            height="30px"
                            is-transparent="true"
                            font-size="13px"
                            class="access-grants__flows-area__learn-button"
                        />
                    </a>
                    <VButton
                        label="Create Keys for CLI"
                        font-size="13px"
                        width="auto"
                        height="30px"
                        class="access-grants__flows-area__create-button"
                        :on-press="cliClick"
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
            <div class="access-grants-items2">
                <v-table
                    v-if="accessGrantsList.length && !areGrantsFetching"
                    class="access-grants-items2__content"
                    items-label="access grants"
                    :limit="accessGrantLimit"
                    :total-page-count="totalPageCount"
                    :items="accessGrantsList"
                    :total-items-count="accessGrantsTotalCount"
                    :on-page-click-callback="onPageClick"
                >
                    <template #head>
                        <th class="align-left">Name</th>
                        <th class="align-left">Date Created</th>
                    </template>
                    <template #body>
                        <AccessGrantsItem2
                            v-for="(grant, key) in accessGrantsList"
                            :key="key"
                            :item-data="grant"
                            :dropdown-key="key"
                            :is-dropdown-open="activeDropdown === key"
                            @openDropdown="openDropdown"
                            @deleteClick="onDeleteClick"
                        />
                    </template>
                </v-table>
                <div
                    v-if="!accessGrantsList.length && !areGrantsFetching"
                    class="access-grants-items2__empty-state"
                >
                    <span class="access-grants-items2__empty-state__text">
                        {{ emptyStateLabel }}
                    </span>
                </div>
            </div>
        </div>
        <div v-if="!isNewAccessGrantFlow">
            <VLoader v-if="areGrantsFetching" width="100px" height="100px" class="grants-loader" />
            <div v-if="accessGrantsList.length && !areGrantsFetching" class="access-grants-items">
                <v-table
                    v-if="accessGrantsList.length && !areGrantsFetching"
                    class="access-grants-items__content"
                    items-label="access grants"
                    :selectable="true"
                    :limit="accessGrantLimit"
                    :total-page-count="totalPageCount"
                    :items="accessGrantsList"
                    :total-items-count="accessGrantsTotalCount"
                    :on-page-click-callback="onPageClick"
                >
                    <template #head>
                        <th class="align-left">Name</th>
                        <th class="align-left">Date Created</th>
                    </template>
                    <template #body>
                        <AccessGrantsItem
                            v-for="(grant, key) in accessGrantsList"
                            :key="key"
                            :item-data="grant"
                            @accessGrantClick="toggleSelection"
                            @selectChange="(_) => toggleSelection(grant)"
                        />
                    </template>
                </v-table>
            </div>
        </div>
        <div v-if="!isNewAccessGrantFlow">
            <ConfirmDeletePopup
                v-if="isDeleteClicked"
                @close="onClearSelection"
            />
            <EmptyState v-if="!accessGrantsList.length && !areGrantsFetching" />
        </div>
        <div v-if="isNewAccessGrantFlow">
            <ConfirmDeletePopup2
                v-if="isDeleteClicked"
                @close="onClearSelection"
            />
        </div>
        <router-view />
    </div>
</template>
<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { MetaUtils } from '@/utils/meta';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant, AccessGrantsOrderBy } from '@/types/accessGrants';
import { SortDirection } from '@/types/common';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import AccessGrantsItem from '@/components/accessGrants/AccessGrantsItem.vue';
import AccessGrantsItem2 from '@/components/accessGrants/AccessGrantsItem2.vue';
import ConfirmDeletePopup from '@/components/accessGrants/ConfirmDeletePopup.vue';
import ConfirmDeletePopup2 from '@/components/accessGrants/ConfirmDeletePopup2.vue';
import EmptyState from '@/components/accessGrants/EmptyState.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';
import VHeader from '@/components/common/VHeader.vue';
import VTable from '@/components/common/VTable.vue';

import AccessGrantsIcon from '@/../static/images/accessGrants/accessGrantsIcon.svg';
import CLIIcon from '@/../static/images/accessGrants/cli.svg';
import S3Icon from '@/../static/images/accessGrants/s3.svg';

const {
    FETCH,
    TOGGLE_SELECTION,
    CLEAR_SELECTION,
    SET_SORT_BY,
    SET_SORT_DIRECTION,
    SET_SEARCH_QUERY,
} = ACCESS_GRANTS_ACTIONS;

// @vue/component
@Component({
    components: {
        AccessGrantsItem2,
        AccessGrantsItem,
        AccessGrantsIcon,
        CLIIcon,
        EmptyState,
        S3Icon,
        VButton,
        ConfirmDeletePopup,
        ConfirmDeletePopup2,
        VLoader,
        VHeader,
        VTable,
    },
})
export default class AccessGrants extends Vue {
    private FIRST_PAGE = 1;
    private isDeleteClicked = false;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Indicates if the access modal should be shown and what the defaulted type of access should be defaulted.
     */
    private showAccessModal = false;
    private modalAccessType = '';
    public activeDropdown = -1;

    public areGrantsFetching = true;

    /**
     * Indicates if navigation side bar is hidden.
     */
    public get isNewAccessGrantFlow(): boolean {
        const isNewAccessGrantFlow = MetaUtils.getMetaContent('new-access-grant-flow');
        return isNewAccessGrantFlow === 'true';
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
    }

    /**
     * Starts create access grant flow.
     */
    public onCreateClick(): void {
        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).with(RouteConfig.NameStep).path);
        this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).with(RouteConfig.NameStep).path);
    }

    /**
     * Opens AccessGrantItem2 dropdown.
     */
    public openDropdown(key: number): void {
        if (this.activeDropdown === key) {
            this.activeDropdown = -1;

            return;
        }

        this.activeDropdown = key;
    }

    /**
     * Holds on button click login for deleting access grant process.
     */
    public async onDeleteClick(grant: AccessGrant): Promise<void> {
        await this.$store.dispatch(TOGGLE_SELECTION, grant);
        this.isDeleteClicked = true;
    }
    /**
     * Clears access grants selection.
     */
    public async onClearSelection(): Promise<void> {
        this.isDeleteClicked = false;
        await this.$store.dispatch(CLEAR_SELECTION);
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
     * Returns access grants total page count from store.
     */
    public get accessGrantsTotalCount(): number {
        return this.$store.state.accessGrantsModule.page.totalCount;
    }

    /**
     * Returns access grants limit from store.
     */
    public get accessGrantLimit(): number {
        return this.$store.state.accessGrantsModule.page.limit;
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

    /**
     * Returns search query from store.
     */
    private get searchQuery(): string {
        return this.$store.state.accessGrantsModule.cursor.search;
    }

    /**
     * Returns correct empty state label.
     */
    private get emptyStateLabel(): string {
        const noGrants = 'No accesses were created yet.';
        const noSearchResults = 'No results found.';

        return this.searchQuery ? noSearchResults : noGrants;
    }

    /**
     * Access grant button click.
     */
    public accessGrantClick(): void {
        this.analytics.eventTriggered(AnalyticsEvent.CREATE_ACCESS_GRANT_CLICKED);
        this.trackPageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).name,
            params: { accessType: 'access' },
        });
    }

    /**
     * S3 Access button click..
     */
    public s3Click(): void {
        this.analytics.eventTriggered(AnalyticsEvent.CREATE_S3_CREDENTIALS_CLICKED);
        this.trackPageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).name,
            params: { accessType: 's3' },
        });
    }

    /**
     * CLI Access button click.
     */
    public cliClick(): void {
        this.analytics.eventTriggered(AnalyticsEvent.CREATE_KEYS_FOR_CLI_CLICKED);
        this.trackPageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).name,
            params: { accessType: 'api' },
        });
    }

    /**
     * Sends "trackPageVisit" event to segment and opens link.
     */
    public trackPageVisit(link: string): void {
        this.analytics.pageVisit(link);
    }
}
</script>
<style scoped lang="scss">
    @mixin grant-flow-card {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        justify-content: center;
        padding: 10px 28px;
        width: 300px;
        height: 220px;
        background: #fff;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        border-radius: 10px;
        min-width: 175px;
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
            flex-wrap: wrap;
            -webkit-box-align: center;
            align-items: center;
            -webkit-box-pack: justify;
            margin-top: 20px;
            column-gap: 16px;
            row-gap: 16px;

            &__access-grant,
            &__s3-credentials,
            &__cli-credentials {
                @include grant-flow-card;

                @media screen and (max-width: 448px) {
                    height: auto;

                    .access-grants__flows-area__create-button {
                        padding: 20px 10px;
                        margin: 8px 0 0;
                    }
                }
            }

            &__learn-button,
            &__create-button {
                box-sizing: border-box;
                padding: 0 10px;
                height: 30px;
            }

            &__create-button {
                margin-left: 8px;
            }

            &__button-container {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                margin-top: 8px;
                flex-wrap: wrap;
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

            &__content {
                margin-top: 20px;
            }
        }

        .access-grants-items2 {

            &__content {
                margin-top: 20px;
            }

            &__empty-state {
                padding: 48px 0;
                background: #fff;
                border-radius: 6px;
                margin-top: 10px;
                border: 1px solid #dadfe7;
                display: flex;
                justify-content: center;

                &__text {
                    font-size: 14px;
                    line-height: 20px;
                    color: #56606d;
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

// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="access-grants">
        <div class="access-grants__new-title-area">
            <h2 class="access-grants__title-area__title" aria-roledescription="title">Access Management</h2>
            <div class="access-grants__title-area__title-subtext" aria-roledescription="title">Create encryption keys to setup permissions to access your objects.</div>
        </div>
        <div class="access-grants__flows-area">
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
                            :is-transparent="true"
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
                            :is-transparent="true"
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
                            :is-transparent="true"
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
        <div class="access-grants-items">
            <v-table
                v-if="accessGrantsList.length && !areGrantsFetching"
                class="access-grants-items__content"
                items-label="access grants"
                :limit="accessGrantLimit"
                :total-page-count="totalPageCount"
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
                        :dropdown-key="key"
                        :is-dropdown-open="activeDropdown === key"
                        @openDropdown="openDropdown"
                        @deleteClick="onDeleteClick"
                    />
                </template>
            </v-table>
            <div
                v-if="!accessGrantsList.length && !areGrantsFetching"
                class="access-grants-items__empty-state"
            >
                <span class="access-grants-items__empty-state__text">
                    {{ emptyStateLabel }}
                </span>
            </div>
        </div>
        <ConfirmDeletePopup
            v-if="isDeleteClicked"
            @close="onClearSelection"
        />
        <router-view />
    </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant } from '@/types/accessGrants';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AccessType } from '@/types/createAccessGrant';
import { useNotify, useRouter, useStore } from '@/utils/hooks';

import AccessGrantsItem from '@/components/accessGrants/AccessGrantsItem.vue';
import ConfirmDeletePopup from '@/components/accessGrants/ConfirmDeletePopup.vue';
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
    SET_SEARCH_QUERY,
} = ACCESS_GRANTS_ACTIONS;

const FIRST_PAGE = 1;

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
const store = useStore();
const notify = useNotify();
const router = useRouter();

const isDeleteClicked = ref<boolean>(false);
const activeDropdown = ref<number>(-1);
const areGrantsFetching = ref<boolean>(true);

/**
 * Returns access grants pages count from store.
 */
const totalPageCount = computed((): number => {
    return store.state.accessGrantsModule.page.pageCount;
});

/**
 * Returns access grants total page count from store.
 */
const accessGrantsTotalCount = computed((): number => {
    return store.state.accessGrantsModule.page.totalCount;
});

/**
 * Returns access grants limit from store.
 */
const accessGrantLimit = computed((): number => {
    return store.state.accessGrantsModule.page.limit;
});

/**
 * Returns access grants from store.
 */
const accessGrantsList = computed((): AccessGrant[] => {
    return store.state.accessGrantsModule.page.accessGrants;
});

/**
 * Returns search query from store.
 */
const searchQuery = computed((): string => {
    return store.state.accessGrantsModule.cursor.search;
});

/**
 * Returns correct empty state label.
 */
const emptyStateLabel = computed((): string => {
    const noGrants = 'No accesses were created yet.';
    const noSearchResults = 'No results found.';

    return searchQuery.value ? noSearchResults : noGrants;
});

/**
 * Indicates if new access grant flow should be used.
 */
const isNewAccessGrantFlow = computed((): boolean => {
    return store.state.appStateModule.isNewAccessGrantFlow;
});

/**
 * Fetches access grants page by clicked index.
 * @param index
 */
async function onPageClick(index: number): Promise<void> {
    try {
        await store.dispatch(FETCH, index);
    } catch (error) {
        await notify.error(`Unable to fetch Access Grants. ${error.message}`, AnalyticsErrorEventSource.ACCESS_GRANTS_PAGE);
    }
}

/**
 * Opens AccessGrantItem2 dropdown.
 */
function openDropdown(key: number): void {
    if (activeDropdown.value === key) {
        activeDropdown.value = -1;

        return;
    }

    activeDropdown.value = key;
}

/**
 * Holds on button click login for deleting access grant process.
 */
async function onDeleteClick(grant: AccessGrant): Promise<void> {
    await store.dispatch(TOGGLE_SELECTION, grant);
    isDeleteClicked.value = true;
}

/**
 * Clears access grants selection.
 */
async function onClearSelection(): Promise<void> {
    isDeleteClicked.value = false;
    await store.dispatch(CLEAR_SELECTION);
}

/**
 * Fetches Access records by name depending on search query.
 */
async function fetch(searchQuery: string): Promise<void> {
    await store.dispatch(SET_SEARCH_QUERY, searchQuery);

    try {
        await store.dispatch(FETCH, 1);
    } catch (error) {
        await notify.error(`Unable to fetch accesses: ${error.message}`, AnalyticsErrorEventSource.ACCESS_GRANTS_PAGE);
    }
}

/**
 * Access grant button click.
 */
function accessGrantClick(): void {
    analytics.eventTriggered(AnalyticsEvent.CREATE_ACCESS_GRANT_CLICKED);

    if (isNewAccessGrantFlow.value) {
        trackPageVisit(RouteConfig.AccessGrants.with(RouteConfig.NewCreateAccessModal).path);
        router.push({
            name: RouteConfig.NewCreateAccessModal.name,
            params: { accessType: AccessType.AccessGrant },
        });
        return;
    }

    trackPageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
    router.push({
        name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).name,
        params: { accessType: 'access' },
    });
}

/**
 * S3 Access button click..
 */
function s3Click(): void {
    analytics.eventTriggered(AnalyticsEvent.CREATE_S3_CREDENTIALS_CLICKED);

    if (isNewAccessGrantFlow.value) {
        trackPageVisit(RouteConfig.AccessGrants.with(RouteConfig.NewCreateAccessModal).path);
        router.push({
            name: RouteConfig.NewCreateAccessModal.name,
            params: { accessType: AccessType.S3 },
        });
        return;
    }

    trackPageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
    router.push({
        name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).name,
        params: { accessType: 's3' },
    });
}

/**
 * CLI Access button click.
 */
function cliClick(): void {
    analytics.eventTriggered(AnalyticsEvent.CREATE_KEYS_FOR_CLI_CLICKED);

    if (isNewAccessGrantFlow.value) {
        trackPageVisit(RouteConfig.AccessGrants.with(RouteConfig.NewCreateAccessModal).path);
        router.push({
            name: RouteConfig.NewCreateAccessModal.name,
            params: { accessType: AccessType.APIKey },
        });
        return;
    }

    trackPageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).path);
    router.push({
        name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessModal).name,
        params: { accessType: 'api' },
    });
}

/**
 * Sends "trackPageVisit" event to segment and opens link.
 */
function trackPageVisit(link: string): void {
    analytics.pageVisit(link);
}

onMounted(async () => {
    try {
        await store.dispatch(FETCH, FIRST_PAGE);
        areGrantsFetching.value = false;
    } catch (error) {
        await notify.error(`Unable to fetch Access Grants. ${error.message}`, AnalyticsErrorEventSource.ACCESS_GRANTS_PAGE);
    }
});

onBeforeUnmount(() => {
    onClearSelection();
});
</script>
<style scoped lang="scss">
    @mixin grant-flow-card {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        justify-content: center;
        padding: 25px 28px;
        width: 300px;
        background: #fff;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        border-radius: 10px;
        min-width: 175px;

        @media screen and (max-width: 930px) {
            width: 100%;
        }
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

                @media screen and (max-width: 370px) {

                    .access-grants__flows-area__button-container {
                        flex-direction: column;
                        align-items: flex-start;
                    }

                    .access-grants__flows-area__create-button {
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
            padding-bottom: 55px;

            @media screen and (max-width: 1150px) {
                margin-top: -45px;
            }

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
                    color: var(--c-grey-6);
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

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header">
        <div class="header__content-holder">
            <div class="header__content-holder__logo-area">
                <button name="Logo Button" type="button" @click.prevent="onHeaderLogoClick">
                    <StorjIconDark
                        v-if="isDarkMode"
                        class="header__content-holder__logo"
                        alt="storj logo"
                    />
                    <StorjIconLight
                        v-else
                        class="header__content-holder__logo"
                        alt="storj logo"
                    />
                </button>
                <StorjIconWithoutText
                    alt="storj logo"
                    class="header__content-holder__logo--small"
                />
                <button name="Refresh" type="button" class="header__content-holder__logo-area__refresh-button" @click.prevent="onRefresh">
                    <RefreshIcon alt="refresh image" />
                </button>
            </div>
            <div class="header__content-holder__right-area">
                <div role="button" tabindex="0" class="header__content-holder__right-area__node-id-container" @click="copyNodeId">
                    <b class="header__content-holder__right-area__node-id-container__title">Node ID:</b>
                    <p class="header__content-holder__right-area__node-id-container__id">{{ nodeId }}</p>
                    <CopyIcon />
                </div>
                <button name="Settings" aria-pressed="false" class="options-button" type="button" @click.prevent.stop="openOptionsDropdown">
                    <SettingsIcon />
                </button>
                <OptionsDropdown
                    v-show="isOptionsShown"
                    class="options-dropdown"
                    @close-dropdown="closeOptionsDropdown"
                />
                <button name="Notifications" aria-pressed="false" class="header__content-holder__right-area__bell-area" type="button" @click.stop.prevent="toggleNotificationsPopup">
                    <BellIcon />
                    <span
                        v-if="hasNewNotifications"
                        class="header__content-holder__right-area__bell-area__new-circle"
                    />
                </button>
            </div>
            <NotificationsPopup
                v-if="isNotificationPopupShown"
                v-click-outside.stop="closeNotificationPopup"
                class="header__content-holder__right-area__bell-area__popup"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { RouteConfig } from '@/app/router';
import { usePayoutStore } from '@/app/store/modules/payoutStore';
import { useNodeStore } from '@/app/store/modules/nodeStore';
import { useAppStore } from '@/app/store/modules/appStore';
import { useNotificationsStore } from '@/app/store/modules/notificationsStore';

import OptionsDropdown from '@/app/components/OptionsDropdown.vue';
import NotificationsPopup from '@/app/components/notifications/NotificationsPopup.vue';

import CopyIcon from '@/../static/images/Copy.svg';
import StorjIconWithoutText from '@/../static/images/LogoWithoutText.svg';
import BellIcon from '@/../static/images/notifications/bell.svg';
import RefreshIcon from '@/../static/images/refresh.svg';
import SettingsIcon from '@/../static/images/SettingsDots.svg';
import StorjIconLight from '@/../static/images/storjIcon.svg';
import StorjIconDark from '@/../static/images/storjIconDark.svg';

const route = useRoute();
const router = useRouter();

const payoutStore = usePayoutStore();
const nodeStore = useNodeStore();
const appStore = useAppStore();
const notificationsStore = useNotificationsStore();

const FIRST_PAGE = 1;

const isNotificationPopupShown = ref<boolean>(false);
const isOptionsShown = ref<boolean>(false);

const nodeId = computed<string>(() => nodeStore.state.info.id);
const hasNewNotifications = computed<boolean>(() => notificationsStore.state.unreadCount > 0);
const isDarkMode = computed<boolean>(() => appStore.state.isDarkMode);

function openOptionsDropdown(): void {
    setTimeout(() => isOptionsShown.value = true, 0);
}

function closeOptionsDropdown(): void {
    isOptionsShown.value = false;
}

function toggleNotificationsPopup(): void {
    if (route.name === RouteConfig.Notifications.name) {
        return;
    }

    isNotificationPopupShown.value = !isNotificationPopupShown.value;
}

function closeNotificationPopup(): void {
    isNotificationPopupShown.value = false;
}

function copyNodeId(): void {
    navigator.clipboard.writeText(nodeId.value);
}

async function onHeaderLogoClick(): Promise<void> {
    const isCurrentLocationIsHomePage = route.name === RouteConfig.Root.name;

    if (isCurrentLocationIsHomePage) {
        location.reload();
    }

    await router.replace('/');
}

async function onRefresh(): Promise<void> {
    appStore.setLoading(true);

    const selectedSatelliteId = nodeStore.state.selectedSatellite.id;

    appStore.setNoPayoutData(false);

    try {
        await nodeStore.fetchNodeInfo();
        await nodeStore.selectSatellite(selectedSatelliteId);
    } catch (error) {
        console.error('fetching satellite data', error);
    }

    try {
        await payoutStore.fetchPayoutHistory();
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchEstimation(selectedSatelliteId);
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchPricingModel(selectedSatelliteId);
    } catch (error) {
        console.error(error);
    }

    appStore.setLoading(false);

    try {
        await payoutStore.fetchPayoutInfo(selectedSatelliteId);
        await payoutStore.fetchTotalPayments(selectedSatelliteId);
    } catch (error) {
        console.error(error);
    }

    try {
        await notificationsStore.fetchNotifications(FIRST_PAGE);
    } catch (error) {
        console.error(error);
    }

    try {
        await payoutStore.fetchHeldHistory();
    } catch (error) {
        console.error(error);
    }
}

onBeforeMount(async () => {
    appStore.setLoading(true);

    try {
        await nodeStore.fetchNodeInfo();
        await notificationsStore.fetchNotifications(FIRST_PAGE);
    } catch (error) {
        console.error(error);
    }

    appStore.setLoading(false);
});
</script>

<style scoped lang="scss">
    .svg :deep(path) {
        fill: var(--node-id-copy-icon-color);
    }

    .storj-logo :deep(path) {
        fill: var(--icon-color) !important;
    }

    .settings-icon {

        circle {
            fill: var(--regular-icon-color) !important;
        }
    }

    .notifications-bell-icon :deep(path) {
        fill: var(--regular-icon-color) !important;
    }

    .header {
        padding: 0 36px;
        width: calc(100% - 72px);
        height: 89px;
        display: flex;
        justify-content: center;
        background-color: var(--block-background-color);
        position: fixed;
        top: 0;
        z-index: 9999;

        &__content-holder {
            width: 822px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            position: relative;

            &__logo-area {
                display: flex;
                align-items: center;

                &__refresh-button {
                    margin-left: 25px;
                    max-height: 42px;
                    cursor: pointer;

                    .refresh-button-svg-rect {
                        fill: var(--refresh-button-background-color);
                        stroke: var(--refresh-button-border-color);
                    }

                    .refresh-button-svg-path {
                        fill: var(--icon-color);
                    }

                    &:hover {

                        .refresh-button-svg-rect {
                            fill: var(--icon-color);
                        }

                        .refresh-button-svg-path {
                            fill: var(--block-background-color);
                        }
                    }
                }
            }

            &__logo {
                cursor: pointer;

                &--small {
                    display: none;
                }
            }

            &__right-area {
                display: flex;
                align-items: center;
                justify-content: flex-end;

                &__node-id-container {
                    color: var(--node-id-text-color);
                    height: 44px;
                    padding: 0 14px;
                    display: flex;
                    align-items: center;
                    border: 1px solid var(--node-id-border-color);
                    border-radius: 12px;
                    font-size: 14px;
                    margin-right: 30px;
                    cursor: pointer;

                    &__title {
                        font-family: 'font_bold', sans-serif;
                        min-width: 55px;
                        margin-right: 5px;
                    }

                    &__id {
                        font-size: 11px;
                        padding-right: 20px;
                    }

                    &:hover {
                        border-color: var(--node-id-border-hover-color);
                        color: var(--node-id-hover-text-color);

                        .svg :deep(path) {
                            fill: var(--node-id-border-hover-color) !important;
                        }
                    }
                }

                &__bell-area {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    position: relative;
                    width: 26px;
                    height: 32px;
                    cursor: pointer;
                    margin-left: 35px;

                    &__new-circle {
                        position: absolute;
                        top: 0;
                        right: 0;
                        display: inline-block;
                        width: 6px;
                        height: 6px;
                        border-radius: 50%;
                        background-color: #eb001b;
                    }

                    &__popup {
                        position: absolute;
                        top: 105px;
                        right: 0;
                    }
                }
            }
        }
    }

    .options-button {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 30px;
        width: 30px;
        cursor: pointer;
    }

    .options-dropdown {
        position: absolute;
        top: 89px;
        right: 55px;
    }

    @media screen and (width <= 780px) {

        .header__content-holder {

            &__logo {
                order: 2;
            }

            &__logo-area {
                width: calc(50% + 63px);
                justify-content: space-between;

                &__refresh-button {
                    order: 1;
                    margin-left: 0;
                }
            }

            &__right-area {

                &__node-id-container {
                    display: none;
                }
            }
        }
    }

    @media screen and (width <= 600px) {

        .header__content-holder {

            &__logo-area {
                width: calc(50% + 20px);
            }

            &__logo {
                display: none;

                &--small {
                    display: block;
                    order: 2;
                }
            }
        }
    }

    @media screen and (width <= 600px) {

        .header__content-holder__right-area__bell-area__popup {
            position: fixed;
            top: 89px;
            right: 0;
            left: 0;
        }
    }
</style>

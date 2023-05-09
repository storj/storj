// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header">
        <div class="header__content-holder">
            <div class="header__content-holder__logo-area">
                <button name="Logo Button" type="button" @click.prevent="onHeaderLogoClick">
                    <StorjIcon
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
                <div v-clipboard="nodeId" role="button" tabindex="0" class="header__content-holder__right-area__node-id-container">
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
                    @closeDropdown="closeOptionsDropdown"
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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/app/router';
import { APPSTATE_ACTIONS } from '@/app/store/modules/appState';
import { NODE_ACTIONS } from '@/app/store/modules/node';
import { NOTIFICATIONS_ACTIONS } from '@/app/store/modules/notifications';
import { PAYOUT_ACTIONS } from '@/app/store/modules/payout';

import OptionsDropdown from '@/app/components/OptionsDropdown.vue';
import NotificationsPopup from '@/app/components/notifications/NotificationsPopup.vue';

import CopyIcon from '@/../static/images/Copy.svg';
import StorjIconWithoutText from '@/../static/images/LogoWithoutText.svg';
import BellIcon from '@/../static/images/notifications/bell.svg';
import RefreshIcon from '@/../static/images/refresh.svg';
import SettingsIcon from '@/../static/images/SettingsDots.svg';
import StorjIcon from '@/../static/images/storjIcon.svg';

const {
    GET_NODE_INFO,
    SELECT_SATELLITE,
} = NODE_ACTIONS;

// @vue/component
@Component({
    components: {
        OptionsDropdown,
        NotificationsPopup,
        SettingsIcon,
        StorjIcon,
        RefreshIcon,
        BellIcon,
        StorjIconWithoutText,
        CopyIcon,
    },
})
export default class SNOHeader extends Vue {
    public isNotificationPopupShown = false;
    public isOptionsShown = false;
    private readonly FIRST_PAGE: number = 1;

    /**
     * Lifecycle hook before render.
     * Fetches first page of notifications.
     */
    public async beforeMount(): Promise<void> {
        await this.$store.dispatch(APPSTATE_ACTIONS.SET_LOADING, true);

        try {
            await this.$store.dispatch(NODE_ACTIONS.GET_NODE_INFO);
            await this.$store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, this.FIRST_PAGE);
        } catch (error) {
            console.error(error);
        }

        await this.$store.dispatch(APPSTATE_ACTIONS.SET_LOADING, false);
    }

    public get nodeId(): string {
        return this.$store.state.node.info.id;
    }

    public get hasNewNotifications(): boolean {
        return this.$store.state.notificationsModule.unreadCount > 0;
    }

    public openOptionsDropdown(): void {
        setTimeout(() => this.isOptionsShown = true, 0);
    }

    public closeOptionsDropdown(): void {
        this.isOptionsShown = false;
    }

    /**
     * toggleNotificationPopup toggles NotificationPopup visibility.
     */
    public toggleNotificationsPopup(): void {
        /**
         * Blocks opening popup in current route is /notifications.
         */
        if (this.$route.name === RouteConfig.Notifications.name) {
            return;
        }

        this.isNotificationPopupShown = !this.isNotificationPopupShown;
    }

    /**
     * closeNotificationPopup when clicking outside popup.
     */
    public closeNotificationPopup(): void {
        this.isNotificationPopupShown = false;
    }

    /**
     * Refreshes page when on home page or relocates to home page from other location.
     */
    public async onHeaderLogoClick(): Promise<void> {
        const isCurrentLocationIsHomePage = this.$route.name === RouteConfig.Root.name;

        if (isCurrentLocationIsHomePage) {
            location.reload();
        }

        await this.$router.replace('/');
    }

    /**
     * Refreshes all needed data from server.
     */
    public async onRefresh(): Promise<void> {
        await this.$store.dispatch(APPSTATE_ACTIONS.SET_LOADING, true);

        const selectedSatelliteId = this.$store.state.node.selectedSatellite.id;
        await this.$store.dispatch(APPSTATE_ACTIONS.SET_NO_PAYOUT_DATA, false);

        try {
            await this.$store.dispatch(GET_NODE_INFO);
            await this.$store.dispatch(SELECT_SATELLITE, selectedSatelliteId);
        } catch (error) {
            console.error('fetching satellite data', error);
        }

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_PAYOUT_HISTORY);
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_ESTIMATION, selectedSatelliteId);
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_PRICING_MODEL, selectedSatelliteId);
        } catch (error) {
            console.error(error);
        }

        await this.$store.dispatch(APPSTATE_ACTIONS.SET_LOADING, false);

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_PAYOUT_INFO, selectedSatelliteId);
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_TOTAL, selectedSatelliteId);
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, this.FIRST_PAGE);
        } catch (error) {
            console.error(error);
        }

        try {
            await this.$store.dispatch(PAYOUT_ACTIONS.GET_HELD_HISTORY);
        } catch (error) {
            console.error(error);
        }
    }
}
</script>

<style scoped lang="scss">
    .svg ::v-deep path {
        fill: var(--node-id-copy-icon-color);
    }

    .storj-logo ::v-deep path {
        fill: var(--icon-color) !important;
    }

    .settings-icon {

        circle {
            fill: var(--regular-icon-color) !important;
        }
    }

    .notifications-bell-icon ::v-deep path {
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

                        .svg ::v-deep path {
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

    @media screen and (max-width: 780px) {

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

    @media screen and (max-width: 600px) {

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

    @media screen and (max-width: 600px) {

        .header__content-holder__right-area__bell-area__popup {
            position: fixed;
            top: 89px;
            right: 0;
            left: 0;
        }
    }
</style>

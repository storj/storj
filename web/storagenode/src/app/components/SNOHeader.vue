// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header">
        <div class="header__content-holder">
            <div class="header__content-holder__logo-area">
                <StorjIcon
                    class="header__content-holder__logo"
                    alt="storj logo"
                    @click="onHeaderLogoClick"
                />
                <img
                    src="@/../static/images/LogoWithoutText.png"
                    alt="storj logo"
                    class="header__content-holder__logo--small"
                >
                <div class="header__content-holder__logo-area__refresh-button" @click="onRefresh">
                    <RefreshIcon alt="refresh image"/>
                </div>
            </div>
            <div class="header__content-holder__right-area">
                <div class="header__content-holder__right-area__node-id-container">
                    <b class="header__content-holder__right-area__node-id-container__title">Node ID:</b>
                    <p class="header__content-holder__right-area__node-id-container__id">{{this.nodeId}}</p>
                </div>
                <div class="header__content-holder__right-area__bell-area" @click.stop="toggleNotificationsPopup">
                    <BellIcon />
                    <span
                        class="header__content-holder__right-area__bell-area__new-circle"
                        v-if="hasNewNotifications"
                    />
                </div>
            </div>
            <NotificationsPopup
                v-if="isNotificationPopupShown"
                class="header__content-holder__right-area__bell-area__popup"
                v-click-outside="closeNotificationPopup"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NotificationsPopup from '@/app/components/notifications/NotificationsPopup.vue';

import BellIcon from '@/../static/images/notifications/bell.svg';
import RefreshIcon from '@/../static/images/refresh.svg';
import StorjIcon from '@/../static/images/storjIcon.svg';

import { RouteConfig } from '@/app/router';
import { NODE_ACTIONS } from '@/app/store/modules/node';
import { NOTIFICATIONS_ACTIONS } from '@/app/store/modules/notifications';
import { NotificationsCursor } from '@/app/types/notifications';

const {
    GET_NODE_INFO,
    SELECT_SATELLITE,
} = NODE_ACTIONS;

@Component({
    components: {
        NotificationsPopup,
        StorjIcon,
        RefreshIcon,
        BellIcon,
    },
})
export default class SNOHeader extends Vue {
    public isNotificationPopupShown: boolean = false;

    /**
     * Lifecycle hook before render.
     * Fetches first page of notifications.
     */
    public beforeMount(): void {
        try {
            this.$store.dispatch(NODE_ACTIONS.GET_NODE_INFO);
            this.$store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, new NotificationsCursor(1));
        } catch (error) {
            console.error(error.message);
        }
    }

    public get nodeId(): string {
        return this.$store.state.node.info.id;
    }

    public get hasNewNotifications(): boolean {
        return this.$store.state.notificationsModule.unreadCount > 0;
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

    public async onRefresh(): Promise<void> {
        const selectedSatellite = this.$store.state.node.selectedSatellite.id;

        try {
            await this.$store.dispatch(GET_NODE_INFO);
            await this.$store.dispatch(SELECT_SATELLITE, selectedSatellite);
        } catch (error) {
            console.error(`${error.message} satellite data.`);
        }
    }
}
</script>

<style scoped lang="scss">
    .header {
        padding: 0 36px;
        width: calc(100% - 72px);
        height: 89px;
        display: flex;
        justify-content: center;
        background-color: #fff;
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

                    &:hover {

                        .refresh-button-svg-rect {
                            fill: #133e9c;
                        }

                        .refresh-button-svg-path {
                            fill: #fff;
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
                    color: #535f77;
                    height: 44px;
                    padding: 0 14px 0 14px;
                    display: flex;
                    align-items: center;
                    border: 1px solid #e8e8e8;
                    border-radius: 12px;
                    font-size: 14px;
                    margin-right: 30px;

                    &__title {
                        min-width: 55px;
                        margin-right: 5px;
                    }

                    &__id {
                        font-size: 11px;
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

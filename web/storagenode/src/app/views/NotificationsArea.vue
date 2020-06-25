// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notifications-container">
        <div class="notifications-container__header">
            <div class="notifications-container__header__left-area">
                <router-link to="/" class="notifications-container__header__back-link">
                    <BackArrowIcon />
                </router-link>
                <p class="notifications-container__header__text">Notifications</p>
            </div>
            <div
                class="notifications-container__header__button"
                :class="{ disabled: isMarkAllAsReadButtonDisabled }"
                @click="markAllAsRead"
            >
                <p class="notifications-container__header__button__label">Mark all as read</p>
            </div>
        </div>
        <div class="notifications-container__content-area" v-if="notifications.length">
            <SNONotification
                class="notification"
                v-for="notification in notifications"
                :key="notification.id"
                :notification="notification"
            />
        </div>
        <div class="notifications-container__empty-state" v-else>
            <img
                class="notifications-container__empty-state__image"
                src="@/../static/images/notifications/EmptyStateLarge.png"
                alt="Empty state image"
            >
            <p class="notifications-container__empty-state__label">No notifications yet</p>
        </div>
        <VPagination
            v-if="totalPageCount > 1"
            class="pagination-area"
            ref="pagination"
            :total-page-count="totalPageCount"
            :on-page-click-callback="onPageClick"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import SNONotification from '@/app/components/notifications/SNONotification.vue';
import VPagination from '@/app/components/VPagination.vue';

import BackArrowIcon from '@/../static/images/notifications/backArrow.svg';

import { NOTIFICATIONS_ACTIONS } from '@/app/store/modules/notifications';
import { Notification, NotificationsCursor } from '@/app/types/notifications';

@Component ({
    components: {
        VPagination,
        SNONotification,
        BackArrowIcon,
    },
})
export default class NotificationsArea extends Vue {
    /**
     * Returns notification of current page.
     */
    public get notifications(): Notification[] {
        return this.$store.state.notificationsModule.notifications;
    }

    /**
     * Indicates if mark all as read button should be disabled.
     */
    public get isMarkAllAsReadButtonDisabled(): boolean {
        return this.$store.state.notificationsModule.unreadCount === 0;
    }

    /**
     * Returns total number of notification pages.
     */
    public get totalPageCount(): number {
        return this.$store.state.notificationsModule.pageCount;
    }

    /**
     * onPageClick fetches notifications on selected page.
     *
     * @param index number of page
     */
    public async onPageClick(index: number): Promise<void> {
        try {
            await this.$store.dispatch(NOTIFICATIONS_ACTIONS.GET_NOTIFICATIONS, new NotificationsCursor(index));
        } catch (error) {
            console.error(error.message);
        }
    }

    /**
     * markAllAsRead marks all notifications as read and disables button.
     */
    public async markAllAsRead(): Promise<void> {
        try {
            await this.$store.dispatch(NOTIFICATIONS_ACTIONS.READ_ALL);
        } catch (error) {
            console.error(error.message);
        }
    }
}
</script>

<style scoped lang="scss">
    .notifications-container {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: flex-start;
        width: 822px;
        min-height: calc(100vh - 89px - 89px);
        overflow-y: scroll;
        height: calc(100vh - 89px - 50px);
        padding-bottom: 50px;

        &__header {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 17px;

            &__left-area {
                display: flex;
                align-items: center;
                justify-content: flex-start;
                width: auto;
            }

            &__back-link {
                width: 25px;
                height: 25px;
                display: flex;
                align-items: center;
                justify-content: center;
            }

            &__text {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 57px;
                color: var(--regular-text-color);
                margin-left: 29px;
                text-align: center;
            }

            &__button {
                width: 140px;
                height: 35px;
                display: flex;
                align-items: center;
                justify-content: center;
                border: 1px solid var(--read-button-border-color);
                border-radius: 8px;
                background-color: transparent;

                &__label {
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                    color: var(--read-button-label-color);
                    text-align: left;
                }

                &:hover {
                    border: 1px solid white;
                    background-color: var(--container-color);
                    cursor: pointer;
                }
            }
        }

        &__content-area {
            width: 100%;
            height: auto;
            max-height: 65vh;
            background-color: var(--app-background-color);
            border-radius: 12px;
            margin-top: 20px;
            overflow-y: auto;
        }

        &__empty-state {
            height: 62vh;
            width: 100%;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;

            &__image {
                width: 366px;
                height: 417px;
            }

            &__label {
                margin-top: 50px;
                font-family: 'font_medium', sans-serif;
                font-size: 24px;
                color: var(--regular-text-color);
            }
        }
    }

    .notification {
        margin-bottom: 1px;
    }

    .disabled {
        border: 1px solid transparent;
        background-color: var(--disabled-background-color);
        pointer-events: none;

        .notifications-container__header__button__svg path {
            fill: #979ba7 !important;
        }

        .notifications-container__header__button__label {
            color: #979ba7 !important;
        }
    }

    @media screen and (max-width: 1000px) {

        .notifications-container {
            padding: 0 37px;
            width: calc(100% - 74px);
        }
    }

    @media screen and (max-width: 450px) {

        .notifications-container {

            &__header {
                flex-direction: column;
                align-items: flex-start;
                margin: 0;
            }

            &__empty-state {

                &__image {
                    margin-top: 30px;
                    width: 275px;
                    height: 312px;
                }
            }
        }
    }

    @media screen and (max-height: 650px), (max-width: 300px) {

        .notifications-container {

            &__empty-state {

                &__image {
                    display: none;
                }
            }
        }
    }
</style>

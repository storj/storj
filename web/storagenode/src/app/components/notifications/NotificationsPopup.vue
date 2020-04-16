// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notification-popup-container">
        <div class="notification-popup-container__header">
            <p class="notification-popup-container__header__title">Notifications</p>
            <router-link :to="notificationsPath">
                <p class="notification-popup-container__header__link">See All</p>
            </router-link>
        </div>
        <div
            class="notification-popup-container__content"
            :class="{'collapsed': isCollapsed}"
            v-if="latestNotifications.length"
        >
            <SNONotification
                v-for="notification in latestNotifications"
                :key="notification.id"
                is-small="true"
                :notification="notification"
            />
        </div>
        <div class="notification-popup-container__empty-state" v-else>
            <img src="@/../static/images/notifications/EmptyState.png" alt="Empty state image">
            <p class="notification-popup-container__empty-state__label">No notifications yet</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import SNONotification from '@/app/components/notifications/SNONotification.vue';

import { RouteConfig } from '@/app/router';

@Component({
    components: {
        SNONotification,
    },
})
export default class NotificationsPopup extends Vue {
    /**
     * Path to notifications route.
     */
    public readonly notificationsPath: string = RouteConfig.Notifications.path;

    /**
     * Represents first page of notifications.
     */
    public get latestNotifications(): Notification[] {
        return this.$store.state.notificationsModule.latestNotifications;
    }

    /**
     * Indicates if popup is smaller than with scroll.
     */
    public get isCollapsed(): boolean {
        return this.latestNotifications.length < 4;
    }
}
</script>

<style scoped lang="scss">
    .notification-popup-container {
        position: relative;
        width: 400px;
        height: auto;
        max-height: 376px;
        background-color: var(--block-background-color);
        border-radius: 12px;
        padding: 27px 0 10px 0;
        box-shadow: 0 7px 17px var(--block-background-color);
        z-index: 104;

        &__header {
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 36px;
                color: var(--title-text-color);
                margin-left: 32px;
            }

            &__link {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: var(--navigation-link-color);
                margin-right: 20px;
            }
        }

        &__content {
            height: 300px;
            overflow-y: scroll;
        }

        &__empty-state {
            width: 100%;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 46px 0;

            &__label {
                margin-top: 35px;
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                color: var(--regular-text-color);
            }
        }
    }

    .collapsed {
        height: auto !important;
    }

    @media screen and (max-width: 460px) {

        .notification-popup-container {
            width: 100%;
            max-height: 350px;
        }
    }
</style>

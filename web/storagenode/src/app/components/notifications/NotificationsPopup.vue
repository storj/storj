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
        <div class="notification-popup-container__content">
            <SNONotification />
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
    }
})
export default class NotificationsPopup extends Vue {
    public readonly notificationsPath: string = RouteConfig.Notifications.path;

    public get latestNotifications() {
        return [];
    }
}
</script>

<style scoped lang="scss">
    .notification-popup-container {
        position: relative;
        width: 400px;
        max-height: 376px;
        background-color: #fff;
        border-radius: 12px;
        padding: 27px 0;
        box-shadow: 0 7px 17px #e7ebee;

        &__header {
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 36px;
                color: #384b65;
                margin-left: 32px;
            }

            &__link {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: #224ca5;
                margin-right: 20px;
            }
        }
    }
</style>

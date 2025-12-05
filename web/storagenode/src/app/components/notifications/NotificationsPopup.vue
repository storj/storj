// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notification-popup-container">
        <div class="notification-popup-container__header">
            <p class="notification-popup-container__header__title">Notifications</p>
            <a :href="notificationsPath" class="notification-popup-container__header__link">
                <p>See All</p>
            </a>
        </div>
        <div
            v-if="latest.length"
            class="notification-popup-container__content"
            :class="{'collapsed': isCollapsed}"
        >
            <SNONotification
                v-for="notification in latest"
                :key="notification.id"
                :notification="notification"
            />
        </div>
        <div v-else class="notification-popup-container__empty-state">
            <img src="@/../static/images/notifications/EmptyState.png" alt="Empty state image">
            <p class="notification-popup-container__empty-state__label">No notifications yet</p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { RouteConfig } from '@/app/router';
import { UINotification } from '@/app/types/notifications';
import { useNotificationsStore } from '@/app/store/modules/notificationsStore';

import SNONotification from '@/app/components/notifications/SNONotification.vue';

const notificationsStore = useNotificationsStore();

const notificationsPath = ref<string>(RouteConfig.Notifications.path);

const latest = computed<UINotification[]>(() => notificationsStore.state.latestNotifications);

const isCollapsed = computed<boolean>(() => latest.value.length < 4);
</script>

<style scoped lang="scss">
    .notification-popup-container {
        position: relative;
        width: 400px;
        height: auto;
        max-height: 376px;
        background-color: var(--block-background-color);
        border-radius: 12px;
        padding: 27px 0 10px;
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
                text-decoration: none;
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

    @media screen and (width <= 460px) {

        .notification-popup-container {
            width: 100%;
            max-height: 350px;
        }
    }
</style>

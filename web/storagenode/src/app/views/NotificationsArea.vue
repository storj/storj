// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notifications-container">
        <div class="notifications-container__header">
            <div class="notifications-container__header__left-area">
                <router-link :to="'/'" class="notifications-container__header__back-link">
                    <BackArrowIcon />
                </router-link>
                <p class="notifications-container__header__text">Notifications</p>
            </div>
            <div
                class="notifications-container__header__button"
                :class="{ 'disabled': isMarkAsReadPressed }"
                @click="markAsRead"
            >
                <p class="notifications-container__header__button__label">Mark all as read</p>
            </div>
        </div>
        <div class="notifications-container__content-area" v-if="false">
<!--            <SNONotification class="notification"/>-->
        </div>
        <div class="notifications-container__empty-state" v-else>
            <img src="@/../static/images/notifications/EmptyState.png" alt="Empty state image">
            <p class="notifications-container__empty-state__label">No notifications yet</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import SNONotification from '@/app/components/notifications/SNONotification.vue';

import BackArrowIcon from '@/../static/images/notifications/backArrow.svg';

@Component ({
    components: {
        SNONotification,
        BackArrowIcon,
    },
})
export default class NotificationsArea extends Vue {
    public isMarkAsReadPressed: boolean = false;

    public markAsRead(): void {
        this.isMarkAsReadPressed = true;
    }
}
</script>

<style scoped lang="scss">
    .notifications-container {
        width: 822px;
        height: calc(100vh - 89px - 89px);

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
                color: #535f77;
                margin-left: 29px;
                text-align: center;
            }

            &__button {
                width: 140px;
                height: 35px;
                display: flex;
                align-items: center;
                justify-content: center;
                border: 1px solid #c3cad8;
                border-radius: 8px;
                background-color: transparent;

                &__label {
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                    color: #535f77;
                    text-align: left;
                }

                &:hover {
                    border: 1px solid white;
                    background-color: #fff;
                    cursor: pointer;
                }
            }
        }

        &__content-area {
            width: 100%;
            height: auto;
            max-height: 62vh;
            background-color: #f3f4f9;
            border-radius: 12px;
            overflow-y: auto;
        }

        &__empty-state {
            height: 62vh;
            width: 100%;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;

            &__label {
                margin-top: 35px;
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                color: #1c2a3e;
            }
        }
    }

    .notification {
        margin-bottom: 1px;
    }

    .disabled {
        border: 1px solid transparent;
        background-color: #e8eaf2;
        pointer-events: none;

        .notifications-container__header__button__svg path {
            fill: #979ba7 !important;
        }

        .notifications-container__header__button__label {
            color: #979ba7 !important;
        }
    }
</style>

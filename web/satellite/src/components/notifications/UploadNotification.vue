// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notification-wrap__content" :class="{ 'notification-wrap__warning-colors': warningNotification}">
        <component :is="notificationIcon" class="notification-wrap__content__icon" />
        <div class="notification-wrap__content__middle">
            <p class="text"><b>{{ wordingBold }}</b> {{ wording }}</p>
            <a target="_blank" href="https://docs.storj.io/dcs/getting-started/quickstart-objectbrowser#WeS6v">See documentation</a>
        </div>
        <CloseIcon class="notification-wrap__content__close" @click="onCloseClick" />
    </div>
</template>

<script setup lang="ts">
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

const props = defineProps<{
  wordingBold: string;
  wording: string;
  notificationIcon: string;
  warningNotification: boolean;
  onCloseClick: () => void;
}>();

</script>

<style scoped lang="scss">
    .notification-wrap {

        &__content {
            display: flex;
            justify-content: space-between;
            align-items: center;
            background-color: #fff;
            border: 1px solid rgb(56 75 101 / 40%);
            padding: 16px;
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            border-radius: 16px;
            box-shadow: 0 7px 20px rgba(0 0 0 / 15%);

            &__icon {
                flex-shrink: 0;
                margin-right: 16px;
            }

            &__middle {
                display: flex;
                flex-flow: row wrap;
                justify-content: space-between;
                width: 100%;
                gap: 16px;

                & b {
                    font-family: 'font_medium', sans-serif;
                }

                & a {
                    color: black;
                    text-decoration: underline;
                }
            }

            &__close {
                flex-shrink: 0;
                margin-left: 16px;
                cursor: pointer;
            }
        }

        &__warning-colors {
            background-color: #fec;
            border: 1px solid #ffd78a;
        }
    }
</style>

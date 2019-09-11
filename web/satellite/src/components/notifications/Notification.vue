// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div :style="notification.style" class="notification-wrap" :class="{ active: isClassActive }" @mouseover="onMouseOver" @mouseleave="onMouseLeave" >
        <div class="notification-wrap__text">
            <div v-html="notification.imgSource"></div>
            <p>{{notification.message}}</p>
        </div>
        <div class="notification-wrap__buttons-group" @click="onCloseClick">
            <span class="notification-wrap__buttons-group__close">
                <svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M23.7071 9.70711C24.0976 9.31658 24.0976 8.68342 23.7071 8.29289C23.3166 7.90237 22.6834 7.90237 22.2929 8.29289L23.7071 9.70711ZM8.29289 22.2929C7.90237 22.6834 7.90237 23.3166 8.29289 23.7071C8.68342 24.0976 9.31658 24.0976 9.70711 23.7071L8.29289 22.2929ZM9.70711 8.29289C9.31658 7.90237 8.68342 7.90237 8.29289 8.29289C7.90237 8.68342 7.90237 9.31658 8.29289 9.70711L9.70711 8.29289ZM22.2929 23.7071C22.6834 24.0976 23.3166 24.0976 23.7071 23.7071C24.0976 23.3166 24.0976 22.6834 23.7071 22.2929L22.2929 23.7071ZM22.2929 8.29289L8.29289 22.2929L9.70711 23.7071L23.7071 9.70711L22.2929 8.29289ZM8.29289 9.70711L22.2929 23.7071L23.7071 22.2929L9.70711 8.29289L8.29289 9.70711Z" fill="#354049"/>
                </svg>
            </span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { DelayedNotification } from '@/types/DelayedNotification';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

@Component
export default class Notification extends Vue {
    @Prop({default: () => new DelayedNotification(() => { return; }, '', '')})
    private notification: DelayedNotification;

    public isClassActive = false;

    // Force delete notification
    public onCloseClick(): void {
        this.$store.dispatch(NOTIFICATION_ACTIONS.DELETE, this.notification.id);
    }

    // Force notification to stay on page on mouse over it
    public onMouseOver(): void {
        this.$store.dispatch(NOTIFICATION_ACTIONS.PAUSE, this.notification.id);
    }

    // Resume notification flow when mouse leaves notification
    public onMouseLeave(): void {
        this.$store.dispatch(NOTIFICATION_ACTIONS.RESUME, this.notification.id);
    }

    public mounted() {
        setTimeout(() => {
            this.isClassActive = true;
        }, 100);
    }
}
</script>

<style scoped lang="scss">
    .notification-wrap {
        position: relative;
        right: -100%;
        width: calc(100% - 40px);
        height: 78px;
        display: flex;
        justify-content: space-between;
        padding: 0 20px;
        align-items: center;
        border-radius: 12px;
        margin-bottom: 7px;
        transition: all 0.3s;

        &__text {
            display: flex;
            align-items: center;

            p {
                font-family: 'font_medium';
                font-size: 14px;
                margin-left: 17px;

                span {
                    margin-right: 10px;
                }
            }
        }

        &__buttons-group {
            display: flex;
            
            &__close {
                width: 32px;
                height: 32px;
                cursor: pointer;
            }
        }
    }

    .active {
        right: 0;
    }
</style>

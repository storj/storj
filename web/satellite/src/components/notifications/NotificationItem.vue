// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div :style="notification.style" class="notification-wrap" :class="{ active: isClassActive }" @mouseover="onMouseOver" @mouseleave="onMouseLeave" >
        <div class="notification-wrap__text-area">
            <div class="notification-wrap__text-area__image" v-html="notification.imgSource"></div>
            <p class="notification-wrap__text-area__message">{{notification.message}}</p>
        </div>
        <div class="notification-wrap__buttons-group" @click="onCloseClick">
            <span class="notification-wrap__buttons-group__close">
                <CloseIcon/>
            </span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import CloseIcon from '@/../static/images/notifications/close.svg';

import { DelayedNotification } from '@/types/DelayedNotification';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        CloseIcon,
    },
})
export default class NotificationItem extends Vue {
    @Prop({default: () => new DelayedNotification(() => { return; }, '', '')})
    private notification: DelayedNotification;

    public isClassActive = false;

    /**
     * Forces notification deletion.
     */
    public onCloseClick(): void {
        this.$store.dispatch(NOTIFICATION_ACTIONS.DELETE, this.notification.id);
    }

    /**
     * Forces notification to stay on page on mouse over it.
     */
    public onMouseOver(): void {
        this.$store.dispatch(NOTIFICATION_ACTIONS.PAUSE, this.notification.id);
    }

    /**
     * Resume notification flow when mouse leaves notification.
     */
    public onMouseLeave(): void {
        this.$store.dispatch(NOTIFICATION_ACTIONS.RESUME, this.notification.id);
    }

    /**
     * Uses for class change for animation.
     */
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
        height: auto;
        display: flex;
        justify-content: space-between;
        padding: 20px;
        align-items: center;
        border-radius: 12px;
        margin-bottom: 7px;
        transition: all 0.3s;

        &__text-area {
            display: flex;
            align-items: center;

            &__image {
                max-height: 40px;
            }

            &__message {
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                height: auto;
                width: 270px;
                margin: 0 0 0 17px;
                word-break: break-word;
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

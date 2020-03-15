// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notification-item" :class="{ 'unread': !notification.isRead }" @mouseenter="read">
        <div class="row">
            <div class="notification-item__new-indicator-container">
                <span v-if="!notification.isRead" class="notification-item__new-indicator-container__circle" />
            </div>
            <div class="notification-item__icon-container">
                <div class="icon" v-html="notification.icon"></div>
            </div>
            <div class="notification-item__text-container">
                <p
                    class="notification-item__text-container__message"
                    :class="{'small-font-size': isSmall}"
                >
                    <b class="notification-item__text-container__message__bold">{{ notification.title }}:</b> {{ notification.message }}
                </p>
                <p class="notification-item__text-container__date" v-if="isSmall">{{ notification.dateLabel }}</p>
            </div>
        </div>
        <div class="notification-item__date-container" v-if="!isSmall">
            <p class="notification-item__date-container__date">{{ notification.dateLabel }}</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { NOTIFICATIONS_ACTIONS } from '@/app/store/modules/notifications';
import { Notification } from '@/app/types/notifications';

@Component
export default class SNONotification extends Vue {
    @Prop({default: () => new Notification()})
    public readonly notification: Notification;

    /**
     * isSmall props indicates if component used in popup.
     */
    @Prop({default: false})
    public isSmall: boolean;

    /**
     * Minimal window width in pixels for normal notification.
     */
    private readonly MIN_WINDOW_WIDTH = 640;

    /**
     * Tracks window width for changing notification isSmall type.
     */
    public changeNotificationSize(): void {
        this.isSmall = window.innerWidth < this.MIN_WINDOW_WIDTH;
    }

    /**
     * Lifecycle hook after initial render.
     * Adds event on window resizing to change notification isSmall prop.
     */
    public mounted(): void {
        window.addEventListener('resize', this.changeNotificationSize);
        this.changeNotificationSize();
    }

    /**
     * Lifecycle hook before component destruction.
     * Removes event on window resizing.
     */
    public beforeDestroy(): void {
        window.removeEventListener('resize', this.changeNotificationSize);
    }

    /**
     * Fires on hover on notification. If notification is new, marks it as read.
     */
    public read(): void {
        if (this.notification.isRead) {
            return;
        }

        try {
            this.$store.dispatch(NOTIFICATIONS_ACTIONS.MARK_AS_READ, this.notification.id);
        } catch (error) {
            // TODO: implement UI notification system.
            console.error(error.message);
        }
    }
}
</script>

<style scoped lang="scss">
    .notification-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        background-color: var(--block-background-color);
        padding: 17px 24px 18px 15px;
        width: calc(100% - 24px - 15px);
        font-family: 'font_regular', sans-serif;

        &__new-indicator-container {
            display: flex;
            align-items: center;
            justify-items: center;
            width: 6px;
            height: 6px;
            min-width: 6px;
            min-height: 6px;
            margin-right: 11px;

            &__circle {
                display: inline-block;
                border-radius: 50%;
                background-color: var(--navigation-link-color);
                width: 100%;
                height: 100%;
            }
        }

        &__icon-container {
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 50%;
            background-color: var(--notifications-icon-background);
            width: 40px;
            height: 40px;
            min-width: 40px;
            min-height: 40px;
            margin-right: 17px;
        }

        &__text-container {

            &__message {
                font-size: 15px;
                color: var(--regular-text-color);
                text-align: left;
                word-break: break-word;

                &__bold {
                    font-family: 'font_bold', sans-serif;
                }
            }

            &__date {
                margin-top: 6px;
                font-size: 9px;
                color: var(--label-text-color);
                text-align: left;
            }
        }

        &__date-container {
            margin-left: 20px;
            min-width: 100px;

            &__date {
                font-size: 12px;
                color: var(--label-text-color);
                text-align: right;
            }
        }
    }

    .icon {
        width: 30px;
        height: 30px;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .unread {
        background-color: var(--unread-notification-background-color);
    }

    .small-font-size {
        font-size: 12px;
    }

    .row {
        display: flex;
        align-items: center;
    }
</style>

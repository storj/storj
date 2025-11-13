// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notification-item" :class="{ 'unread': !notification.isRead }" @mouseenter="read">
        <div class="row">
            <div class="notification-item__new-indicator-container">
                <span v-if="!notification.isRead" class="notification-item__new-indicator-container__circle" />
            </div>
            <div class="notification-item__icon-container">
                <div class="icon">
                    <component :is="notification.icon" />
                </div>
            </div>
            <div class="notification-item__text-container">
                <p
                    class="notification-item__text-container__message"
                    :class="{'small-font-size': isSmall}"
                >
                    <b class="notification-item__text-container__message__bold">{{ notification.title }}:</b> {{ notification.message }}
                </p>
                <p v-if="isSmall" class="notification-item__text-container__date">{{ notification.dateLabel }}</p>
            </div>
        </div>
        <div v-if="!isSmall" class="notification-item__date-container">
            <p class="notification-item__date-container__date">{{ notification.dateLabel }}</p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';

import { UINotification } from '@/app/types/notifications';
import { useNotificationsStore } from '@/app/store/modules/notificationsStore';

const props = withDefaults(defineProps<{
    notification?: UINotification;
}>(), {
    notification: () => new UINotification(),
});

const notificationsStore = useNotificationsStore();

const MIN_WINDOW_WIDTH = 640;

const isSmall = ref<boolean>(false);

function changeNotificationSize(): void {
    isSmall.value = window.innerWidth < MIN_WINDOW_WIDTH;
}

async function read(): Promise<void> {
    if (props.notification.isRead) {
        return;
    }

    try {
        await notificationsStore.markAsRead(props.notification.id);
    } catch (error) {
        // TODO: implement UI notification system.
        console.error(error);
    }
}

onMounted(() => {
    window.addEventListener('resize', changeNotificationSize);
    changeNotificationSize();
});

onBeforeUnmount(() => {
    window.removeEventListener('resize', changeNotificationSize);
});
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
            place-items: center center;
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
                overflow-wrap: anywhere;

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

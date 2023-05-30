// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div :style="notification.style" class="notification-wrap" :class="{ active: isClassActive }" @mouseover="onMouseOver" @mouseleave="onMouseLeave">
        <div class="notification-wrap__content-area">
            <div class="notification-wrap__content-area__image">
                <component :is="notification.icon" />
            </div>
            <div class="notification-wrap__content-area__message-area">
                <p class="notification-wrap__content-area__message">{{ notification.message }}</p>

                <p v-if="isTimeoutMentioned && notOnSettingsPage" class="notification-wrap__content-area__account-msg">
                    To change this go to your
                    <router-link :to="settingsRoute" class="notification-wrap__content-area__account-msg__link">
                        account settings
                    </router-link>
                </p>

                <a
                    v-if="isSupportLinkMentioned"
                    :href="requestURL"
                    class="notification-wrap__content-area__link"
                    target="_blank"
                >
                    {{ requestURL }}
                </a>
            </div>
        </div>
        <div class="notification-wrap__buttons-group" @click="onCloseClick">
            <span class="notification-wrap__buttons-group__close">
                <CloseIcon />
            </span>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import { DelayedNotification } from '@/types/DelayedNotification';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { RouteConfig } from '@/router';

import CloseIcon from '@/../static/images/notifications/close.svg';

const configStore = useConfigStore();
const notificationsStore = useNotificationsStore();
const route = useRoute();

const props = withDefaults(defineProps<{
    notification: DelayedNotification;
}>(), {
    notification: () => new DelayedNotification(() => { return; }, '', ''),
});

const isClassActive = ref<boolean>(false);

/**
 * Returns the correct settings route based on if we're on all projects dashboard.
 */
const settingsRoute = computed((): string => {
    if (
        route.path.includes(RouteConfig.AllProjectsDashboard.path)
        || route.path.includes(RouteConfig.AccountSettings.path)
    ) {
        return RouteConfig.AccountSettings.with(RouteConfig.Settings2).path;
    }

    return RouteConfig.Account.with(RouteConfig.Settings).path;
});

/**
 * Returns the URL for the general request page from the store.
 */
const requestURL = computed((): string => {
    return configStore.state.config.generalRequestURL;
});

/**
 * Returns whether we are not on a settings page.
 */
const notOnSettingsPage = computed((): boolean => {
    return route.name !== RouteConfig.Settings.name
        && route.name !== RouteConfig.Settings2.name;
});

/**
 * Indicates if session timeout is mentioned in message.
 * Temporal solution, can be changed later.
 */
const isTimeoutMentioned = computed((): boolean => {
    return props.notification.message.toLowerCase().includes('session timeout');
});

/**
 * Indicates if support word is mentioned in message.
 * Temporal solution, can be changed later.
 */
const isSupportLinkMentioned = computed((): boolean => {
    return props.notification.message.toLowerCase().includes('support');
});

/**
 * Forces notification deletion.
 */
function onCloseClick(): void {
    notificationsStore.deleteNotification(props.notification.id);
}

/**
 * Forces notification to stay on page on mouse over it.
 */
function onMouseOver(): void {
    notificationsStore.pauseNotification(props.notification.id);
}

/**
 * Resume notification flow when mouse leaves notification.
 */
function onMouseLeave(): void {
    notificationsStore.resumeNotification(props.notification.id);
}

/**
 * Uses for class change for animation.
 */
onMounted((): void => {
    setTimeout(() => {
        isClassActive.value = true;
    }, 100);
});
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

        &__content-area {
            display: flex;
            align-items: center;
            font-family: 'font_medium', sans-serif;
            font-size: 14px;

            &__image {
                max-height: 40px;
            }

            &__message-area {
                display: flex;
                flex-direction: column;
                align-items: flex-start;
                justify-content: space-between;
                margin: 0 0 0 17px;
            }

            &__message,
            &__link {
                height: auto;
                width: 270px;
                word-break: break-word;

                @media screen and (width <= 450px) {
                    width: unset;
                }
            }

            &__link {
                margin-top: 5px;
                color: #224ca5;
                text-decoration: underline;
                cursor: pointer;
                word-break: normal;
            }

            &__account-msg {
                margin-top: 20px;

                &__link {
                    display: block;
                    color: var(--c-black);
                    text-decoration: underline;
                    cursor: pointer;
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

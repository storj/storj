// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notification-wrap">
        <div class="notification-wrap__content">
            <div class="notification-wrap__content__left">
                <InfoIcon class="notification-wrap__content__left__icon" />
                <p><b>Billing details are now located under the account menu.</b> Check it out by clicking on My Account.</p>
            </div>
            <div class="notification-wrap__content__right">
                <router-link :to="billingPath">See Billing</router-link>
                <CloseIcon class="notification-wrap__content__right__close" @click="onCloseClick" />
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, reactive } from 'vue';

import { RouteConfig } from '@/router';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { NavigationLink } from '@/types/navigation';
import { useRouter, useStore } from '@/utils/hooks';

import InfoIcon from '@/../static/images/notifications/info.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

const store = useStore();
const nativeRouter = useRouter();
const router = reactive(nativeRouter);

/**
 * Returns the base account route based on if we're on all projects dashboard.
 */
const baseAccountRoute = computed((): NavigationLink => {
    if (router.currentRoute.path.includes(RouteConfig.AccountSettings.path)) {
        return RouteConfig.AccountSettings;
    }
    return RouteConfig.Account;
});

const billingPath = computed((): string => {
    const billing = baseAccountRoute.value.with(RouteConfig.Billing);

    return billing.with(RouteConfig.BillingOverview).path;
});

/**
 * Closes notification.
 */
function onCloseClick(): void {
    store.commit(APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION);
}
</script>

<style scoped lang="scss">
    .notification-wrap {
        position: relative;

        &__content {
            position: absolute;
            left: 40px;
            right: 44px;
            bottom: 32px;
            z-index: 9999;
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 20px;
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            background-color: var(--c-blue-2);
            border: 1px solid #a5beff;
            border-radius: 16px;
            box-shadow: 0 7px 20px rgba(0 0 0 / 15%);

            &__left {
                display: flex;
                align-items: center;

                & b {
                    font-family: 'font_medium', sans-serif;
                }

                &__icon {
                    flex-shrink: 0;
                    margin-right: 16px;
                }
            }

            &__right {
                display: flex;
                align-items: center;
                flex-shrink: 0;
                margin-left: 16px;

                & a {
                    color: black;
                    text-decoration: underline !important;
                }

                &__close {
                    margin-left: 16px;
                    cursor: pointer;
                }
            }
        }
    }
</style>

// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="onCloseClick">
        <template #content>
            <div class="modal">
                <p class="modal__title">Add Coupon Code</p>
                <AddCouponCodeInput />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { RouteConfig } from '@/types/router';
import { useAppStore } from '@/store/modules/appStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import AddCouponCodeInput from '@/components/common/AddCouponCodeInput.vue';
import VModal from '@/components/common/VModal.vue';

const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();

/**
 * Closes add coupon modal.
 */
function onCloseClick(): void {
    analyticsStore.pageVisit(RouteConfig.Account.with(RouteConfig.Billing).path);
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
    .modal {
        width: 500px;
        padding: 32px;

        @media screen and (width <= 650px) {
            width: unset;
            padding: 24px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 148.31%;
            margin: 15px 0;
        }
    }
</style>

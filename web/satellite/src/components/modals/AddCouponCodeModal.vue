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
import { MODALS } from '@/utils/constants/appStatePopUps';
import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useStore } from '@/utils/hooks';

import AddCouponCodeInput from '@/components/common/AddCouponCodeInput.vue';
import VModal from '@/components/common/VModal.vue';

const store = useStore();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Closes add coupon modal.
 */
function onCloseClick(): void {
    analytics.pageVisit(RouteConfig.Account.with(RouteConfig.Billing).path);
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.addCoupon);
}
</script>

<style scoped lang="scss">
    .modal {
        width: 500px;
        padding: 32px;

        @media screen and (max-width: 650px) {
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

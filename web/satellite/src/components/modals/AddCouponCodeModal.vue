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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import AddCouponCodeInput from '@/components/common/AddCouponCodeInput.vue';
import VModal from '@/components/common/VModal.vue';

// @vue/component
@Component({
    components: {
        AddCouponCodeInput,
        VModal,
    },
})
export default class AddCouponCodeModal extends Vue {
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
    * Closes add coupon modal.
    */
    public onCloseClick(): void {
        this.analytics.pageVisit(RouteConfig.Account.with(RouteConfig.Billing).path);
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_ADD_COUPON_MODAL_SHOWN);
    }
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

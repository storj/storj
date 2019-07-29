// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dialog-container" id="makeDefaultPaymentDialog">
        <div class="delete-container">
            <h1>Update Default Card</h1>
            <h2>We will automatically charge your default card at the close of the current billing period</h2>
            <div class="button-container">
                <Button height="48px" width="128px" label="Cancel" isWhite="true" :onPress="onCancelClick"/>
                <Button class="delete-button" height="48px" width="128px" label="Update" :onPress="onUpdateClick"/>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Prop, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import {
        APP_STATE_ACTIONS,
        NOTIFICATION_ACTIONS,
        PROJECT_PAYMENT_METHODS_ACTIONS
    } from '@/utils/constants/actionNames';

    @Component({
        components: {
            Button,
        }
    })
    export default class MakeDefaultPaymentMethodDialog extends Vue {
        @Prop({default: ''})
        private readonly paymentMethodID: string;

        public onCancelClick(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
        }

        public async onUpdateClick(): Promise<void> {
            const result = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.SET_DEFAULT, this.paymentMethodID);
            if (!result.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, result.errorMessage);

                return;
            }

            const paymentMethodsResponse = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
            if (!paymentMethodsResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch payment methods: ${paymentMethodsResponse.errorMessage}`);
            }

            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Default payment method set successfully');
        }
    }
</script>

<style scoped lang="scss">
    .dialog-container {
        background-image: url('../../../../static/images/ContainerCentered.png');
        background-size: 340px 240px;
        z-index: 1;
        position: absolute;
        left: 50%;
        bottom: 40px;
        transform: translate(-50%);
        height: 240px;
        width: 340px;
    }

    h1 {
        font-family: 'font_bold';
        font-size: 16px;
        line-height: 21px;
        color: #384B65;
    }

    h2 {
        font-family: 'font_regular';
        font-size: 12px;
        color: #384B65;
    }

    .delete-container {
        display: flex;
        flex-direction: column;
        padding: 25px 32px 33px 32px;
        box-shadow: 0px 4px 20px rgba(204, 208, 214, 0.25);
        margin-top: 4px;
    }

    .button-container {
        display: flex;
        flex-direction: row;
        margin-top: 25px;
        justify-content: space-between;
    }
</style>

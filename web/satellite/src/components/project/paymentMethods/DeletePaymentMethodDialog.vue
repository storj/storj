// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dialog-container" id="deletePaymentMethodDialog">
        <div class="delete-container">
            <h1>Confirm Delete Card</h1>
            <h2>Are you sure you want to remove your card?</h2>
            <div class="button-container">
                <Button height="48px" width="128px" label="Cancel" isWhite="true" :onPress="onCancelClick"/>
                <Button class="delete-button" height="48px" width="128px" label="Delete" :onPress="onDeleteClick"/>
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
    export default class DeletePaymentMethodDialog extends Vue {
        @Prop({default: ''})
        private readonly paymentMethodID: string;

        public onCancelClick(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
        }

        public async onDeleteClick(): Promise<void> {
            const response = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.DELETE, this.paymentMethodID);
            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
            }

            const paymentMethodsResponse = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
            if (!paymentMethodsResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch payment methods: ${paymentMethodsResponse.errorMessage}`);
            }

            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Payment method deleted successfully');
        }
    }
</script>

<style scoped lang="scss">
    .dialog-container {
        background-image: url('../../../../static/images/container.png');
        background-size: 340px 240px;
        z-index: 1;
        position: absolute;
        bottom: 40px;
        right: -17px;
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
        margin-top: 20px;
    }

    .delete-container {
        display: flex;
        flex-direction: column;
        justify-content: space-between;
        padding: 25px 32px 33px 32px;
        box-shadow: 0px 4px 20px rgba(204, 208, 214, 0.25);
    }

    .button-container {
        display: flex;
        flex-direction: row;
        justify-content: space-between;
        margin-top: 35px;
    }

    .delete-button {
        background-color: #EB5757;
        color: white;

        &:hover {
            box-shadow: none;
            background-color: #d24949;
        }
    }
</style>

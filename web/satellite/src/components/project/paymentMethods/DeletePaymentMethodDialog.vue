// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dialog-container" id="deletePaymentMethodDialog">
        <div class="delete-container">
            <h1>Confirm Delete Card</h1>
            <h2>Are you sure you want to remove your card?</h2>
            <div class="button-container">
                <Button height="48px" width="128px" label="Cancel" isWhite="true" :on-press="onCancelClick"/>
                <Button class="delete-button" height="48px" width="128px" label="Delete" :on-press="onDeleteClick"/>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';
    import {
        APP_STATE_ACTIONS,
        NOTIFICATION_ACTIONS,
        PROJECT_PAYMENT_METHODS_ACTIONS
    } from '@/utils/constants/actionNames';

    @Component({
        props: {
            paymentMethodID: {
                type: String,
                default: ''
            },
        },
        methods: {
            onCancelClick: function () {
                this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
            },
            onDeleteClick: async function () {
                const response = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.DELETE, this.$props.paymentMethodID);
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
                }

                const paymentMethodsResponse = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
                if (!paymentMethodsResponse.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch payment methods: ' + paymentMethodsResponse.errorMessage);
                }

                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Successfully delete payment method');
            }
        },
        components: {
            Button,
        }
    })
    export default class DeletePaymentMethodDialog extends Vue {
    }
</script>

<style scoped lang="scss">
    .dialog-container{
        background-image: url('../../../../static/images/container.svg');
        background-size: cover;
        background-repeat: no-repeat;

        z-index: 1;

        position: absolute;
        top: 40px;
        right: -38px;

        height: 223px;
        width: 351px;

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
        margin-top: 12px;
    }

    .button-container {
        display: flex;
        flex-direction: row;
        margin-top: 25px;
    }

    .delete-button {
        margin-left: 11px;

        /*&:hover {*/
        /*&.container {*/
        /*box-shadow: none;*/
        /*background-color: #d24949;*/
        /*}*/
        /*}*/

    }
    .delete-button.container {
        background-color: #EB5757;
    &:hover {

         box-shadow: none;
         background-color: #d24949;

     }
    }

    .delete-button.label {
        color: white;
    }

</style>

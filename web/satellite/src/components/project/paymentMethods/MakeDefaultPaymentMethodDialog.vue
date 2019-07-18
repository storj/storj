// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dialog-container" id="makeDefaultPaymentDialog">
        <div class="delete-container">
            <h1>Update Default Card</h1>
            <h2>We will automatically charge your default card at the close of the current billing period</h2>
            <div class="button-container">
                <Button height="48px" width="128px" label="Cancel" isWhite="true" :on-press="onCancelClick"/>
                <Button class="delete-button" height="48px" width="128px" label="Update" :on-press="onUpdateClick"/>
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
            onUpdateClick: async function () {
                const result = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.SET_DEFAULT, this.$props.paymentMethodID);
                if (!result.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, result.errorMessage);

                    return;
                }

                const paymentMethodsResponse = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
                if (!paymentMethodsResponse.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch payment methods: ' + paymentMethodsResponse.errorMessage);
                }

                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Successfully set default payment method');
            }
        },
        components: {
            Button,
        }
    })
    export default class MakeDefaultPaymentMethodDialog extends Vue {
    }
</script>

<style scoped lang="scss">
    .dialog-container {
        background-image: url('../../../../static/images/ContainerCentered.svg');
        background-size: cover;
        background-repeat: no-repeat;

        z-index: 1;

        position: absolute;
        left: 50%;
        transform: translate(-50%);
        top: 40px;

        height: 240px;
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

    }
</style>

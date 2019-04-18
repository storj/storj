// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./registrationSuccessPopup.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import ROUTES from '@/utils/constants/routerConstants';

@Component(
        {
            computed:{
                isPopupShown: function () {
                    return this.$store.state.appStateModule.appState.isSuccessfulRegistrationPopupShown;
                }
            },
            methods: {
                onCloseClick: function () {
                    this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
                    this.$router.push(ROUTES.LOGIN.path);
                }
            },
            components: {
                Button,
            },
        }
    )

    export default class RegistrationSuccessPopup extends Vue {
    }
</script>

<style scoped lang="scss">
    p {
        font-family: 'font_medium';
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        padding: 27px 0 0 0;
        margin: 0;
    }

    a {
        font-family: 'font_bold';
        color: #2683ff;
    }

    .register-success-popup-container {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1000;
        display: flex;
        justify-content: center;
        align-items: center;
    }
    .register-success-popup {
        width: 100%;
        max-width: 845px;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: center;
        padding: 80px 100px 80px 50px;

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 100px;
            margin-top: 20px;
        }

        &__form-container {
            width: 100%;
            max-width: 440px;
            margin-top: 10px;

            &__main-label-text {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
                margin: 0;
            }

            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 15px;
            }
        }

        &__close-cross-container {
            display: flex;
            justify-content: center;
            align-items: center;
            position: absolute;
            right: 30px;
            top: 40px;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover svg path {
                fill: #2683FF;
            }
        }
    }

    @media screen and (max-width: 720px) {
        .register-success-popup {

            &__info-panel-container {
                display: none;

            }

            &__form-container {

                &__button-container {
                    width: 100%;
                }
            }
        }
    }
</style>

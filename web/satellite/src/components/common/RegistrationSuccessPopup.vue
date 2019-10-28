// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./registrationSuccessPopup.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import RegistrationSuccessIcon from '@/../static/images/register/registerSuccess.svg';

import { AuthApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { getUserId } from '@/utils/consoleLocalStorage';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        VButton,
        RegistrationSuccessIcon,
        CloseCrossIcon,
    },
})
export default class RegistrationSuccessPopup extends Vue {
    private isResendEmailButtonDisabled: boolean = true;
    private timeToEnableResendEmailButton: string = '00:30';
    private intervalID: any = null;

    private readonly auth: AuthApi = new AuthApi();

    public beforeDestroy(): void {
        if (this.intervalID) {
            clearInterval(this.intervalID);
        }
    }

    public async onResendEmailButtonClick(): Promise<void> {
        this.isResendEmailButtonDisabled = true;

        const userId = getUserId();
        if (!userId) {
            return;
        }

        try {
            await this.auth.resendEmail(userId);
        } catch (error) {
            await this.$notify.error('Could not send email ');
        }

        this.startResendEmailCountdown();
    }

    public onCloseClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
        this.$router.push(RouteConfig.Login.path);
    }

    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isSuccessfulRegistrationPopupShown;
    }

    private startResendEmailCountdown(): void {
        let countdown = 30;

        this.intervalID = setInterval(() => {
            countdown--;

            const secondsLeft = countdown > 9 ? countdown : `0${countdown}`;
            this.timeToEnableResendEmailButton = `00:${secondsLeft}`;

            if (countdown <= 0) {
                clearInterval(this.intervalID);
                this.isResendEmailButtonDisabled = false;
            }
        }, 1000);
    }
}
</script>

<style scoped lang="scss">
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
        background-color: #fff;
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

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                color: #384b65;
                margin: 0;
            }

            &__text {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                padding: 27px 0 0 0;
                margin: 0;
            }

            &__verification-cooldown {
                font-family: 'font_medium', sans-serif;
                font-size: 12px;
                line-height: 16px;
                color: #354049;
                padding: 27px 0 0 0;
                margin: 0;

                &__bold-text {
                    color: #2683ff;
                }
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

            &:hover .close-cross-svg-path {
                fill: #2683ff;
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

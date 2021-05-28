// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="register-success-area">
        <div class="register-success-area__form-container">
            <MailIcon/>
            <h2 class="register-success-area__form-container__title">You're almost there!</h2>
            <p class="register-success-area__form-container__sub-title">
                Check your email to confirm your account and get started.
            </p>
            <p class="register-success-area__form-container__text">
                Didn't receive a verification email?
                <b class="register-success-area__form-container__verification-cooldown__bold-text">
                    {{timeToEnableResendEmailButton}}
                </b>
            </p>
            <div class="register-success-area__form-container__button-container">
                <VButton
                    label="Resend Email"
                    width="450px"
                    height="50px"
                    :on-press="onResendEmailButtonClick"
                    :is-disabled="isResendEmailButtonDisabled"
                />
            </div>
            <p class="register-success-area__form-container__contact">
                or
                <a
                    class="register-success-area__form-container__contact__link"
                    href="https://support.storj.io/hc/en-us"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Contact our support team
                </a>
            </p>

        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import MailIcon from '@/../static/images/register/mail.svg';

import { AuthHttpApi } from '@/api/auth';
import { LocalData } from '@/utils/localData';

@Component({
    components: {
        VButton,
        MailIcon,
    },
})
export default class RegistrationSuccess extends Vue {
    private isResendEmailButtonDisabled: boolean = true;
    private timeToEnableResendEmailButton: string = '00:30';
    private intervalID: any = null;

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Starts resend email button availability countdown.
     */
    public mounted(): void {
        this.startResendEmailCountdown();
    }

    /**
     * Lifecycle hook before component destroying.
     * Resets interval.
     */
    public beforeDestroy(): void {
        if (this.intervalID) {
            clearInterval(this.intervalID);
        }
    }

    /**
     * Checks if page is inside iframe.
     */
    public get isInsideIframe(): boolean {
        return window.self !== window.top;
    }

    /**
     * Resend email if interval timer is expired.
     */
    public async onResendEmailButtonClick(): Promise<void> {
        if (this.isResendEmailButtonDisabled) {
            return;
        }

        this.isResendEmailButtonDisabled = true;

        const userId = LocalData.getUserId();
        if (!userId) {
            return;
        }

        try {
            await this.auth.resendEmail(userId);
        } catch (error) {
            await this.$notify.error('Could not send email.');
        }

        this.startResendEmailCountdown();
    }

    /**
     * Resets timer blocking email resend button spamming.
     */
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
    .register-success-area {
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: 'font_regular', sans-serif;

        &__form-container {
            padding: 100px;
            max-width: 395px;
            text-align: center;
            border-radius: 20px;
            background-color: #fff;
            box-shadow: 0 0 19px 9px #ddd;
            display: flex;
            flex-direction: column;
            align-items: center;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 40px;
                line-height: 1.2;
                color: #252525;
                margin: 25px 0;
            }

            &__sub-title {
                font-size: 16px;
                line-height: 21px;
                color: #252525;
                margin: 0;
                max-width: 350px;
            }

            &__text {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #252525;
                margin: 27px 0 0 0;
            }

            &__verification-cooldown {
                font-family: 'font_medium', sans-serif;
                font-size: 12px;
                line-height: 16px;
                padding: 27px 0 0 0;
                margin: 0;

                &__bold-text {
                    color: #252525;
                }
            }

            &__button-container {
                width: 100%;
                display: flex;
                justify-content: center;
                align-items: center;
                margin-top: 15px;
            }

            &__contact {
                margin-top: 20px;

                &__link {
                    color: #376fff;

                    &:visited {
                        color: #376fff;
                    }
                }
            }
        }
    }

    @media screen and (max-width: 650px) {

        .register-success-area {

            &__form-container {
                padding: 50px;
            }
        }

        /deep/ .container {
            width: 100% !important;
        }
    }

    @media screen and (max-width: 500px) {

        .register-success-area {

            &__form-container {
                padding: 50px 20px;
            }
        }
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./registrationSuccess.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import { AuthHttpApi } from '@/api/auth';
import { LocalData } from '@/utils/localData';

@Component({
    components: {
        VButton,
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

<style scoped lang="scss" src="./registrationSuccess.scss"></style>

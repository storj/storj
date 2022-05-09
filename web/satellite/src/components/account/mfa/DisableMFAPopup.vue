// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="disable-mfa">
        <div class="disable-mfa__container">
            <h1 class="disable-mfa__container__title">Two-Factor Authentication</h1>
            <p class="disable-mfa__container__subtitle">
                Enter code from your favorite TOTP app to disable 2FA.
            </p>
            <div class="disable-mfa__container__confirm">
                <h2 class="disable-mfa__container__confirm__title">Confirm Authentication Code</h2>
                <ConfirmMFAInput ref="mfaInput" :on-input="onConfirmInput" :is-error="isError" :is-recovery="isRecoveryCodeState" />
                <span class="disable-mfa__container__confirm__toggle" @click="toggleRecoveryCodeState">
                    Or use {{ isRecoveryCodeState ? '2FA code' : 'recovery code' }}
                </span>
            </div>
            <p class="disable-mfa__container__info">
                After disabling 2FA, remove the authentication code from your TOTP app.
            </p>
            <div class="disable-mfa__container__buttons">
                <VButton
                    class="cancel-button"
                    label="Cancel"
                    width="50%"
                    height="44px"
                    is-white="true"
                    :on-press="toggleModal"
                />
                <VButton
                    label="Disable 2FA"
                    width="50%"
                    height="44px"
                    :on-press="disable"
                    :is-disabled="!(request.recoveryCode || request.passcode) || isLoading"
                />
            </div>
            <div class="disable-mfa__container__close-container" @click="toggleModal">
                <CloseCrossIcon />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import ConfirmMFAInput from '@/components/account/mfa/ConfirmMFAInput.vue';
import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

import { USER_ACTIONS } from '@/store/modules/users';
import { DisableMFARequest } from '@/types/users';

interface ClearInput {
    clearInput(): void;
}

// @vue/component
@Component({
    components: {
        ConfirmMFAInput,
        CloseCrossIcon,
        VButton,
    },
})
export default class DisableMFAPopup extends Vue {
    @Prop({default: () => () => {}})
    public readonly toggleModal: () => void;

    public isError = false;
    public isLoading = false;
    public request = new DisableMFARequest();
    public isRecoveryCodeState = false;

    public $refs!: {
        mfaInput: ConfirmMFAInput & ClearInput;
    }

    /**
     * Sets confirmation passcode value from input.
     */
    public onConfirmInput(value: string): void {
        this.isError = false;
        this.isRecoveryCodeState ? this.request.recoveryCode = value : this.request.passcode = value;
    }

    /**
     * Toggles whether the MFA recovery code page is shown.
     */
    public toggleRecoveryCodeState(): void {
        this.isError = false;
        this.request.recoveryCode = this.request.passcode = '';
        this.$refs.mfaInput.clearInput();
        this.isRecoveryCodeState = !this.isRecoveryCodeState;
    }

    /**
     * Disables user MFA.
     */
    public async disable(): Promise<void> {
        if (!(this.request.recoveryCode || this.request.passcode) || this.isLoading || this.isError) return;

        this.isLoading = true;

        try {
            await this.$store.dispatch(USER_ACTIONS.DISABLE_USER_MFA, this.request);
            await this.$store.dispatch(USER_ACTIONS.GET);

            await this.$notify.success('MFA was disabled successfully');

            this.toggleModal();
        } catch (error) {
            await this.$notify.error(error.message);
            this.isError = true;
        }

        this.isLoading = false;
    }
}
</script>

<style scoped lang="scss">
.disable-mfa {
    position: fixed;
    top: 0;
    bottom: 0;
    right: 0;
    left: 0;
    display: flex;
    justify-content: center;
    z-index: 1000;
    background: rgb(27 37 51 / 75%);

    &__container {
        padding: 60px;
        height: fit-content;
        margin-top: 100px;
        position: relative;
        background: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            text-align: center;
            color: #000;
            margin: 0 0 30px;
        }

        &__subtitle {
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #000;
            margin: 0 0 45px;
        }

        &__confirm {
            padding: 25px;
            background: #f5f6fa;
            border-radius: 6px;
            width: calc(100% - 50px);
            display: flex;
            flex-direction: column;
            align-items: center;

            &__title {
                font-size: 16px;
                line-height: 19px;
                text-align: center;
                color: #000;
                margin-bottom: 20px;
            }

            &__toggle {
                font-size: 16px;
                color: #0068dc;
                cursor: pointer;
                margin-top: 20px;
                text-align: center;
            }
        }

        &__info {
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #768394;
            max-width: 485px;
            margin-top: 30px;
        }

        &__buttons {
            display: flex;
            align-items: center;
            width: 100%;
            margin-top: 30px;
        }

        &__close-container {
            display: flex;
            justify-content: center;
            align-items: center;
            position: absolute;
            right: 30px;
            top: 30px;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover .close-cross-svg-path {
                fill: #2683ff;
            }
        }
    }
}

.cancel-button {
    margin-right: 15px;
}

@media screen and (max-height: 750px) {

    .disable-mfa {
        padding-bottom: 20px;
        overflow-y: scroll;
    }
}
</style>

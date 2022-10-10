// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="disable-mfa">
                <h1 class="disable-mfa__title">Two-Factor Authentication</h1>
                <p class="disable-mfa__subtitle">
                    Enter code from your favorite TOTP app to disable 2FA.
                </p>
                <div class="disable-mfa__confirm">
                    <h2 class="disable-mfa__confirm__title">Confirm Authentication Code</h2>
                    <ConfirmMFAInput ref="mfaInput" :on-input="onConfirmInput" :is-error="isError" :is-recovery="isRecoveryCodeState" />
                    <span class="disable-mfa__confirm__toggle" @click="toggleRecoveryCodeState">
                        Or use {{ isRecoveryCodeState ? '2FA code' : 'recovery code' }}
                    </span>
                </div>
                <p class="disable-mfa__info">
                    After disabling 2FA, remove the authentication code from your TOTP app.
                </p>
                <div class="disable-mfa__buttons">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="44px"
                        is-white="true"
                        :on-press="closeModal"
                    />
                    <VButton
                        label="Disable 2FA"
                        width="100%"
                        height="44px"
                        :on-press="disable"
                        :is-disabled="!(request.recoveryCode || request.passcode) || isLoading"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { USER_ACTIONS } from '@/store/modules/users';
import { DisableMFARequest } from '@/types/users';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import ConfirmMFAInput from '@/components/account/mfa/ConfirmMFAInput.vue';
import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

interface ClearInput {
    clearInput(): void;
}

// @vue/component
@Component({
    components: {
        ConfirmMFAInput,
        VButton,
        VModal,
    },
})
export default class DisableMFAModal extends Vue {
    public isError = false;
    public isLoading = false;
    public request = new DisableMFARequest();
    public isRecoveryCodeState = false;

    public $refs!: {
        mfaInput: ConfirmMFAInput & ClearInput;
    };

    /**
     * Closes disable MFA modal.
     */
    public closeModal(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_DISABLE_MFA_MODAL_SHOWN);
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

            this.closeModal();
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
        padding: 60px;
        background: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;

        @media screen and (max-width: 550px) {
            padding: 48px 24px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            text-align: center;
            color: #000;
            margin: 0 0 30px;

            @media screen and (max-width: 550px) {
                font-size: 24px;
                line-height: 28px;
                margin-bottom: 15px;
            }
        }

        &__subtitle {
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #000;
            margin: 0 0 45px;

            @media screen and (max-width: 550px) {
                font-size: 14px;
                line-height: 18px;
                margin-bottom: 20px;
            }
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
            column-gap: 15px;

            @media screen and (max-width: 550px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 10px;
                margin-top: 15px;
            }
        }
    }
</style>

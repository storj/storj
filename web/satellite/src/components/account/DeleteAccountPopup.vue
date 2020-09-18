// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class='delete-account-container'>
        <div class='delete-account' id="deleteAccountPopup">
            <div class='delete-account__info-panel-container'>
                <h2 class='delete-account__info-panel-container__main-label-text'>Delete account</h2>
                <DeleteAccountIcon/>
            </div>
            <div class='delete-account__form-container'>
                <p class='delete-account__form-container__confirmation-text'>Are you sure you want to delete your account? If you do so, all your information, projects and API Keys will be deleted forever (drop from the satellite).</p>
                <HeaderedInput
                    label='Enter your password'
                    placeholder='Your Password'
                    class='full-input'
                    width='100%'
                    is-password="true"
                    :error='passwordError'
                    @setData='setPassword'
                />
                <div class='delete-account__form-container__button-container'>
                    <VButton
                        label='Cancel'
                        width='205px' height='48px'
                        :on-press='onCloseClick'
                        is-transparent="true"
                    />
                    <VButton
                        label='Delete'
                        width='205px'
                        height='48px'
                        class='red'
                        :on-press='onDeleteAccountClick'
                    />
                </div>
            </div>
            <div class='delete-account__close-cross-container' @click='onCloseClick'>
                <CloseCrossIcon/>
            </div>
        </div>
    </div>
</template>

<script lang='ts'>
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import DeleteAccountIcon from '@/../static/images/account/deleteAccountPopup/deleteAccount.svg';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { Validator } from '@/utils/validation';

@Component({
    components: {
        DeleteAccountIcon,
        CloseCrossIcon,
        HeaderedInput,
        VButton,
    },
})
export default class DeleteAccountPopup extends Vue {
    public passwordError: string = '';
    private password: string = '';
    private isLoading: boolean = false;

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    public setPassword(value: string): void {
        this.password = value;
        this.passwordError = '';
    }

    /**
     * Validates password and if it is correct tries to delete account, close popup and redirect to login page.
     */
    public async onDeleteAccountClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        if (!Validator.password(this.password)) {
            this.passwordError = 'Invalid password. Must be 6 or more characters';
            this.isLoading = false;

            return;
        }

        try {
            await this.auth.delete(this.password);
            await this.$notify.success('Account was successfully deleted');

            this.$segment.track(SegmentEvent.USER_DELETED, {
                email: this.$store.getters.user.email,
            });

            this.isLoading = false;
            await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_ACCOUNT);
            await this.$router.push(RouteConfig.Login.path);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }

    /**
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_ACCOUNT);
    }
}
</script>

<style scoped lang='scss'>
    .delete-account-container {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1121;
        display: flex;
        justify-content: center;
        align-items: center;
        font-family: 'font_regular', sans-serif;
    }

    .input-container.full-input {
        width: 100%;
    }

    .red {
        background-color: #eb5757;
    }

    .text {
        margin: 0 !important;
        font-size: 16px;
        line-height: 25px;
    }

    .delete-account {
        width: 100%;
        max-width: 845px;
        background-color: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: center;
        padding: 100px 100px 100px 80px;

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 100px;

            &__main-label-text {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                color: #384b65;
                margin: 0 0 60px 0;
            }
        }

        &__form-container {
            width: 100%;
            max-width: 450px;

            &__confirmation-text {
                margin: 0 0 25px 0;
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 25px;

                &:nth-child(2) {
                    margin-top: 20px;
                }
            }

            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 30px;
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

        .delete-account {
            padding: 10px;

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

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="change-password-popup-container">
        <div class="change-password-popup">
            <div class="change-password-popup__form-container">
                <div class="change-password-row-container">
                    <svg class="change-password-popup__form-container__svg" width="60" height="60" viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M30 60C46.5685 60 60 46.5685 60 30C60 13.4315 46.5685 0 30 0C13.4315 0 0 13.4315 0 30C0 46.5685 13.4315 60 30 60Z" fill="#2683FF"/>
                        <path d="M29.5001 34.5196C30.1001 34.5196 30.5865 34.0452 30.5865 33.46C30.5865 32.8748 30.1001 32.4004 29.5001 32.4004C28.9 32.4004 28.4136 32.8748 28.4136 33.46C28.4136 34.0452 28.9 34.5196 29.5001 34.5196Z" fill="#FEFEFF"/>
                        <path d="M39.9405 40.2152C40.1781 40 40.3139 39.6854 40.3139 39.3709V25.5464C40.3139 24.9007 39.7707 24.3709 39.1086 24.3709H35.7473V21.0927C35.7473 17.7318 32.9462 15 29.5 15C26.0538 15 23.2527 17.7318 23.2527 21.0927V24.3709H19.8914C19.2293 24.3709 18.686 24.9007 18.686 25.5464V39.3709C18.686 39.6854 18.8218 40 19.0595 40.2152L23.7959 44.6689C24.0166 44.8841 24.3222 45 24.6278 45H34.3552C34.6608 45 34.9664 44.8841 35.1871 44.6689L39.9405 40.2152ZM30.7053 36.6391V38.1291C30.7053 38.7748 30.1621 39.3046 29.5 39.3046C28.8379 39.3046 28.2947 38.7748 28.2947 38.1291V36.6391C26.9705 36.1589 26.0198 34.9172 26.0198 33.4437C26.0198 31.5728 27.5817 30.0497 29.5 30.0497C31.4183 30.0497 32.9801 31.5728 32.9801 33.4437C32.9801 34.9172 32.0295 36.1589 30.7053 36.6391ZM33.3367 24.3709H25.6464V21.0927C25.6464 19.0232 27.3779 17.351 29.483 17.351C31.5881 17.351 33.3197 19.0397 33.3197 21.0927V24.3709H33.3367Z" fill="#FEFEFF"/>
                        <defs>
                            <clipPath id="clip0">
                                <rect width="21.6279" height="30" fill="#FFFFFF" transform="translate(18.686 15)"/>
                            </clipPath>
                        </defs>
                    </svg>
                    <h2 class="change-password-popup__form-container__main-label-text">Change Password</h2>
                </div>
                <HeaderlessInput
                    class="full-input"
                    label="Old Password"
                    placeholder ="Enter Old Password"
                    width="100%"
                    isPassword="true"
                    ref="oldPasswordInput"
                    :error="oldPasswordError"
                    @setData="setOldPassword" />
                <HeaderlessInput
                    class="full-input mt"
                    label="New Password"
                    placeholder ="Enter New Password"
                    width="100%"
                    ref="newPasswordInput"
                    isPassword="true"
                    :error="newPasswordError"
                    @setData="setNewPassword" />
                <HeaderlessInput
                    class="full-input mt"
                    label="Confirm password"
                    placeholder="Confirm password"
                    width="100%"
                    ref="confirmPasswordInput"
                    isPassword="true"
                    :error="confirmationPasswordError"
                    @setData="setPasswordConfirmation" />
                <div class="change-password-popup__form-container__button-container">
                    <Button label="Cancel" width="205px" height="48px" :onPress="onCloseClick" isWhite="true" />
                    <Button label="Update" width="205px" height="48px" :onPress="onUpdateClick" />
                </div>
            </div>
            <div class="change-password-popup__close-cross-container" @click="onCloseClick">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
    import Button from '@/components/common/Button.vue';
    import { USER_ACTIONS, NOTIFICATION_ACTIONS, APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
    import { validatePassword } from '@/utils/validation';
    import { RequestResponse } from '@/types/response';

    @Component({
        components: {
            HeaderlessInput,
            Button,
        }
    })
    export default class ChangePasswordPopup extends Vue {
        private oldPassword: string = '';
        private newPassword: string = '';
        private confirmationPassword: string = '';
        private oldPasswordError: string = '';
        private newPasswordError: string = '';
        private confirmationPasswordError: string = '';

        public setOldPassword(value: string): void {
            this.oldPassword = value;
            this.oldPasswordError = '';
        }

        public setNewPassword(value: string): void {
            this.newPassword = value;
            this.newPasswordError = '';
        }

        public setPasswordConfirmation(value: string): void {
            this.confirmationPassword = value;
            this.confirmationPasswordError = '';
        }

        public async onUpdateClick(): Promise<void> {
            let hasError = false;
            if (!this.oldPassword) {
                this.oldPasswordError = 'Password required';
                hasError = true;
            }

            if (!validatePassword(this.newPassword)) {
                this.newPasswordError = 'Invalid password. Use 6 or more characters';
                hasError = true;
            }

            if (!this.confirmationPassword) {
                this.confirmationPasswordError = 'Password required';
                hasError = true;
            }

            if (this.newPassword !== this.confirmationPassword) {
                this.confirmationPasswordError = 'Password not match to new one';
                hasError = true;
            }

            if (hasError) {
                return;
            }

            let response: RequestResponse<object> = await this.$store.dispatch(USER_ACTIONS.CHANGE_PASSWORD,
                {
                    oldPassword: this.oldPassword,
                    newPassword: this.newPassword
                }
            );

            if (!response.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                return;
            }

            this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Password successfully changed!');
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_CHANGE_PASSWORD_POPUP);

            this.oldPassword = '';
            this.newPassword = '';
            this.confirmationPassword = '';

            this.oldPasswordError = '';
            this.newPasswordError = '';
            this.confirmationPasswordError = '';

            let oldPasswordInput: any = this.$refs['oldPasswordInput'];
            oldPasswordInput.setValue('');

            let newPasswordInput: any = this.$refs['newPasswordInput'];
            newPasswordInput.setValue('');

            let confirmPasswordInput: any = this.$refs['confirmPasswordInput'];
            confirmPasswordInput.setValue('');
        }

        public onCloseClick(): void {
            this.cancel();
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_CHANGE_PASSWORD_POPUP);
        }

        private cancel(): void {
            this.oldPassword = '';
            this.newPassword = '';
            this.confirmationPassword = '';

            this.oldPasswordError = '';
            this.newPasswordError = '';
            this.confirmationPasswordError = '';

            let oldPasswordInput: any = this.$refs['oldPasswordInput'];
            oldPasswordInput.setValue('');

            let newPasswordInput: any = this.$refs['newPasswordInput'];
            newPasswordInput.setValue('');

            let confirmPasswordInput: any = this.$refs['confirmPasswordInput'];
            confirmPasswordInput.setValue('');
        }
    }
</script>

<style scoped lang="scss">
    p {
        font-family: 'font_medium';
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        display: flex;
    }
    
    .change-password-popup-container {
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
    
    .input-container.full-input {
        width: 100%;
    }
    
    .change-password-row-container {
        width: 100%;
        display: flex;
        flex-direction: row;
        align-content: center;
        justify-content: flex-start;
        margin-bottom: 20px;
    }
    
    .change-password-popup {
        width: 100%;
        max-width: 440px;
        max-height: 470px;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: center;
        padding: 80px;
        
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
            
            p {
                font-family: 'font_regular';
                font-size: 16px;
                margin-top: 20px;
            
                &:first-child {
                    margin-top: 0;
                }
            }
            
            &__main-label-text {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 60px;
                color: #384B65;
                margin-bottom: 0;
                margin-top: 0;
                margin-left: 32px;
            }
            
            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 32px;
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
        .change-password-popup {
            
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

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class='delete-account-container'>
        <div class='delete-account' id="deleteAccountPopup">
            <div class='delete-account__info-panel-container'>
                <h2 class='delete-account__info-panel-container__main-label-text'>Delete account</h2>
                <div v-html='imageSource'></div>
            </div>
            <div class='delete-account__form-container'>
                <p>Are you sure you want to delete your account? If you do so, all your account information will be deleted from the Satellite forever.</p>
                <HeaderedInput 
                    label='Enter your password' 
                    placeholder='Your Password'
                    class='full-input'
                    width='100%'
                    isPassword
                    :error='passwordError'
                    @setData='setPassword'>
                </HeaderedInput>
                <div class='delete-account__form-container__button-container'>
                    <Button label='Cancel' width='205px' height='48px' :onPress='onCloseClick' isWhite/>
                    <Button label='Delete' width='205px' height='48px' class='red' :onPress='onDeleteAccountClick'/>
                </div>
            </div>
            <div class='delete-account__close-cross-container'>
                <svg width='16' height='16' viewBox='0 0 16 16' fill='none' xmlns='http://www.w3.org/2000/svg' v-on:click='onCloseClick'>
                    <path d='M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z' fill='#384B65'/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang='ts'>
import { Component, Vue } from 'vue-property-decorator';
import HeaderedInput from '@/components/common/HeaderedInput.vue';
import Button from '@/components/common/Button.vue';
import { removeToken } from '@/utils/tokenManager';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import { APP_STATE_ACTIONS, USER_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

@Component(
    {
        data: function() {
            return {
                password: '',
                passwordError: '',
                imageSource: EMPTY_STATE_IMAGES.DELETE_ACCOUNT,
            };
        },
        methods: {
            setPassword: function(value: string): void {
                this.$data.password = value;
            },
            onDeleteAccountClick: async function() {
                let response = await this.$store.dispatch(USER_ACTIONS.DELETE, this.$data.password);

                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                    return;
                }

                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Account was successfully deleted');
                removeToken();
                this.$router.push('/login');
            },
            onCloseClick: function (): void {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_ACCOUNT);
            }
        },
        components: {
            HeaderedInput,
            Button
        }
    }
)

export default class DeleteAccountPopup extends Vue {}
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
    }
    .input-container.full-input {
        width: 100%;
    }
    .red {
        background-color: #EB5757;
    }
    .text {
        margin: 0;
        margin-bottom: 0 !important;
        font-family: 'font_regular' !important;
        font-size: 16px;
        line-height: 25px;
    }
    .delete-account {
        width: 100%;
        max-width: 845px;
        background-color: #FFFFFF;
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
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
                margin-bottom: 60px;
                margin-top: 0;
            }
        }

        &__form-container {
            width: 100%;
            max-width: 450px;

            p {
                margin: 0;
                margin-bottom: 25px;
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 25px;

                &:nth-child(2) {
                    margin-top: 20px;
                }
            }

            a {
                font-family: 'font_medium';
                font-size: 16px;
                color: #2683FF;
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

            &:hover svg path {
                fill: #2683FF;
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

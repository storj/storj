// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>   
    <div id="addApiKeyPopup">
        <div class="save-api-popup">
           <div class="save-api-popup__main">
               <div class="save-api-popup__content">
                   <h1 class="save-api-popup__content__title">Save your API Key</h1>
                   <p class="save-api-popup__content__name">You will need this to share access to the project with your team members.</p>
                   <div class="save-api-popup__content__copy-area">
                       <p class="save-api-popup__content__copy-area__save-api">{{apiKey}}</p>
                       <Button class="save-api-popup__content__copy-area__save-btn" v-clipboard="apiKey" label="Copy" width="140px" height="48px" :onPress="onCopyClick" />
                   </div>
               </div>
               <div class="save-api-popup__close-cross-container">
                   <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="onCloseClick">
                       <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                   </svg>
               </div>
           </div>
            <div class="notification-wrap">
                <svg width="40" height="40" viewBox="0 0 40 40" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <rect width="40" height="40" rx="10" fill="#2683FF"/>
                    <path d="M18.1489 17.043H21.9149V28H18.1489V17.043ZM20 12C20.5816 12 21.0567 12.1823 21.4255 12.5468C21.8085 12.8979 22 13.357 22 13.9241C22 14.4776 21.8085 14.9367 21.4255 15.3013C21.0567 15.6658 20.5816 15.8481 20 15.8481C19.4184 15.8481 18.9362 15.6658 18.5532 15.3013C18.1844 14.9367 18 14.4776 18 13.9241C18 13.357 18.1844 12.8979 18.5532 12.5468C18.9362 12.1823 19.4184 12 20 12Z" fill="#F5F6FA"/>
                </svg>
                <div class="notification-wrap__text">
                    <p>Warning! You must save it anyway, otherwise you will lose a further access to this API Key for ever.</a></p>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';
import HeaderedInput from '@/components/common/HeaderedInput.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from "@/utils/constants/actionNames";

@Component(
    {
        props: {
            onClose: {
                type: Function
            },
            apiKey: String
        },
        data: function () {
            return {
                imageSource: EMPTY_STATE_IMAGES.ADD_API_KEY,
            };
        },
        methods: {
            onCloseClick: function (): void {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_API_KEY);
            },
            onCopyClick: function (): void {
                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, "Key saved to clipboard");
            }
        },
        components: {
            Button,
            HeaderedInput
        }
    }
)

export default class AddApiKeyPopup extends Vue {
}
</script>

<style scoped lang="scss">
    p {
        font-family: 'montserrat_medium';
        font-size: 16px;
        line-height: 21px;
        color: #354049;
    }

    .save-api-popup {
        width: 100%;
        max-width: 1041px;
        max-height: 500px;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        position: relative;
        justify-content: space-between;

        &__main {
            width: 100%;
            border-top-left-radius: 6px;
            border-top-right-radius: 6px;
            display: flex;
            flex-direction: row;
            align-items: flex-start;
            position: relative;
            justify-content: center;
            background-color: #FFFFFF;
            padding: 30px 0 45px 0;
        }

        &__content {
            max-width: 1041px;
            width: 90%;

            &__name {
                font-family: 'montserrat_medium';
                font-size: 16px;
                color: #AFB7C1;
                display: flex;

                span {
                    color: #354049;
                    margin-bottom: 20px;
                    margin-left: 10px;;
                    display: block;
                }
            }

            &__title {
                font-family: 'montserrat_bold';
                font-size: 32px;
                margin-top: 50px;
            }

            &__copy-area {
                background: #F5F6FA;
                display: flex;
                align-items: center;
                justify-content: space-between;
                padding: 10px 20px;
                width: 100%;
                margin-top: 50px;

                &__save-btn {
                    margin-right: 10px;
                }
            }
        }
    }
    .input-container.full-input {
        width: 100%;
    }
    .notification-wrap {
        background-color: rgba(194, 214, 241, 1);
        height: 98px;
        display: flex;
        justify-content: space-between;
        padding: 0 50px;
        align-items: center;
        border-bottom-left-radius: 6px;
        border-bottom-right-radius: 6px;

        &__text {
            display: flex;
            align-items: center;

            p {
                font-family: 'montserrat_medium';
                font-size: 16px;
                margin-left: 40px;

                span {
                    margin-right: 10px;
                }
            }
        }
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>   
    <div>
        <div class="save-api-popup" v-if="true">
            <div class="save-api-popup__content">
                <h1 class="save-api-popup__content__title">Save you API Key or download it in .txt file.</h1>
                <p class="save-api-popup__content__name">You will need this to share access to the project with your team members.</p>
                <div class="save-api-popup__content__copy-area">
                    <p class="save-api-popup__content__copy-area__save-api">ab4923re124NSVDLkvdmsfv mwm45678gnhab4923rewm45678gn</p>
                    <Button class="save-api-popup__content__copy-area__save-btn" label="Copy" width="140px" height="48px" />
                    <Button label="Download" width="140px" height="48px" isWhite/>
                </div>
                <div class="save-api-popup__content__info-area">
                    <img src="../../../static/images/register/ErrorInfo.svg"/>
                    <p class="save-api-popup__content__info-area__text" >Warning! You must save it anyway, otherwise you will lose a further access to this API Key for ever.</p>
                </div>
            </div>
            <div class="add-api-key-popup__close-cross-container">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="onCloseClick">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
        <div class="confirm-api-key-container" v-if="false">
            <div class="confirm-api-key-container__content">
                <div class="confirm-api-key-container__content__back-area">
                    <svg width="19" height="19" viewBox="0 0 19 19" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path fill-rule="evenodd" clip-rule="evenodd" d="M10.5607 0.43934C11.1464 1.02513 11.1464 1.97487 10.5607 2.56066L5.12132 8H17.5C18.3284 8 19 8.67157 19 9.5C19 10.3284 18.3284 11 17.5 11H5.12132L10.5607 16.4393C11.1464 17.0251 11.1464 17.9749 10.5607 18.5607C9.97487 19.1464 9.02513 19.1464 8.43934 18.5607L0.43934 10.5607C-0.146447 9.97487 -0.146447 9.02513 0.43934 8.43934L8.43934 0.43934C9.02513 -0.146447 9.97487 -0.146447 10.5607 0.43934Z" fill="#384B65"/>
                    </svg>
                    <p>Back</p>
                </div>
                <h1 class="confirm-api-key-container__content__title">To ensure that you saved API Key please right down it below</h1>
                <p class="confirm-api-key-container__content__label" >API Key</p>
                <div class="confirm-api-key-container__content__input-area">
                    <HeaderedInput
                        placeholder="Enter API Key"
                        class="full-input"
                        width="98%"/>
                    <Button label="Ok" width="120px" height="52px" />
                </div>
                <div class="confirm-api-key-container__content__info-area">
                    <img src="../../../static/images/register/ErrorInfo.svg"/>
                    <p class="confirm-api-key-container__content__info-area__text" >Warning! You must save it anyway, otherwise you will lose a further access to this API Key for ever.</p>
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

@Component(
    {
        props: {
            onClose: {
                type: Function
            },
        },
        data: function () {
            return {
                imageSource: EMPTY_STATE_IMAGES.ADD_API_KEY,
            };
        },
        methods: {
            onCloseClick: function (): void {
                // TODO: save popup states in store
                this.$emit('onClose');
            },
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
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: space-between;
        padding: 30px 30px 45px 50px;

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

            &__info-area {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: flex-start;
                margin-top: 70px;

                &__text {
                    font-size: 12px;
                    line-height: 18px;
                    color: #AFB7C1;
                    margin-left: 20px;
                }
            }
        }
    }
    .confirm-api-key-container {
        width: 100%;
        max-width: 1041px;
        max-height: 500px;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: space-between;
        padding: 60px 40px 60px 40px;

        &__content {
            max-width: 1041px;
            width: 100%;

            &__back-area {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: flex-start;
                width: 100%;

                p {
                    font-family: 'montserrat_medium';
                    margin-left: 27px;
                }
            }

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
                width: 80%;
            }

            &__label {
                margin-block-start: 0;
                margin-block-end: 0;
            }

            &__input-area {
                display: flex;
                flex-direction: row;
                justify-content: flex-end;
                align-items: flex-end;
                height: 60px;
            }

            &__copy-area {
                background: #F5F6FA;
                display: flex;
                align-items: center;
                justify-content: space-between;
                padding: 10px 20px;
                width: 100%;
                margin-top: 30px;

                &__save-btn {
                    margin-right: 20px;
                }
            }

            &__info-area {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: flex-start;
                margin-top: 70px;

                &__text {
                    font-size: 12px;
                    line-height: 18px;
                    color: #AFB7C1;
                    margin-left: 20px;
                }
            }
        }
    }
    .input-container.full-input {
        width: 100%;
    }
</style>

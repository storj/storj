// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-api-key-popup-container">
        <div v-if="isMacaroonPopupShown !== true" class="add-api-key-popup">
            <div class="add-api-key-popup__info-panel-container">
                <h2 class="add-api-key-popup__info-panel-container__main-label-text">New API Key</h2>
                <div v-html="imageSource"></div>
            </div>
            <div class="add-api-key-popup__form-container">
                <p>Name <span>Up To 20 Characters</span></p>
                <HeaderlessInput 
                    placeholder="Enter API Key Name"
                    class="full-input"
                    width="100%">
                </HeaderlessInput>
                <p>Bucket</p>
                <HeaderlessInput 
                    placeholder="Enter Bucket Name"
                    class="full-input"
                    width="100%">
                </HeaderlessInput>
                <p>Rights</p>
                <ApiKeysDropdown />
                <div class="add-api-key-popup__form-container__button-container">
                    <Button label="Cancel" width="205px" height="48px" :onPress="onCloseClick" isWhite />
                    <Button label="Create API Key" width="205px" height="48px" />
                </div>
            </div>
            <div class="add-api-key-popup__close-cross-container">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" v-on:click="onCloseClick">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
        <div v-if="isMacaroonPopupShown === true" class="macaroon-popup">
            <div v-html="imageSource"></div>
            <div class="macaroon-popup__content">
                <h1 class="macaroon-popup__content__title">Macaroon</h1>
                <p class="macaroon-popup__content__name">Name: <span>Kahmir</span></p>
                <div class="macaroon-popup__content__copy-area">
                    <p class="macaroon-popup__content__copy-area__macaroon">ab4923re124NSVDLkvdmsfv mwm45678gnhab4923rewm45678gn</p>
                    <Button label="Copy" width="140px" height="48px" />
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import Button from '@/components/common/Button.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import ApiKeysDropdown from '@/components/apiKeys/ApiKeysDropdown.vue';

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
                isMacaroonPopupShown: false,
            };
        },
        methods: {
            onCloseClick: function (): void {
                // TODO: save popup states in store
                this.$emit('onClose');
            },
        },
        components: {
            HeaderlessInput,
            Button,
            ApiKeysDropdown
        }
    }
)

export default class AddApiKeyPopup extends Vue {
}
</script>

<style scoped lang="scss">
    .macaroon-popup {
        width: 100%;
        max-width: 845px;
        background-color: #FFFFFF;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: space-between;
        padding: 80px 100px 80px 50px;

        &__content {
            max-width: 555px;
            margin-left: 70px;

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
                font-family: 'montserrat_medium';
                font-size: 32px;
            }

            &__copy-area {
                background: #F5F6FA;
                display: flex;
                align-items: center;
                justify-content: space-between;
                padding: 10px 20px;
            }
        }
    }

    .add-api-key-popup-container {
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
    .add-api-key-popup {
        width: 100%;
        max-width: 845px;
        background-color: #FFFFFF;
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
             margin-right: 55px;

            &__main-label-text {
                 font-family: 'montserrat_bold';
                 font-size: 32px;
                 line-height: 39px;
                 color: #384B65;
                 margin-bottom: 60px;
                 margin-top: 0;
            }
        }

        &__form-container {
             width: 100%;
             max-width: 440px;
             margin-top: 10px;

             p {
                 font-family: 'montserrat_regular';
                 font-size: 16px;
                 margin-top: 20px;

                 &:first-child {
                     margin-top: 0;
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
             align-items: flex-start;
             position: absolute;
             right: 30px;
            top: 40px;
            svg {
                cursor: pointer;
            }
        }
    }

    @media screen and (max-width: 720px) {
        .add-api-key-popup {

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

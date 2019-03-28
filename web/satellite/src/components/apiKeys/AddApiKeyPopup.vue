// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-api-key-popup-container">
        <div id="addApiKeyPopup" v-if="!key" class="add-api-key-popup">
            <div class="add-api-key-popup__info-panel-container">
                <div v-html="imageSource"></div>
            </div>
            <div class="add-api-key-popup__form-container">
                <h2 class="add-api-key-popup__form-container__main-label-text">New API Key</h2>
                <HeaderedInput
                    @setData="onChangeName"
                    label="Name"
                    additionalLabel="Up To 20 Characters"
                    placeholder="Enter API Key Name"
                    class="full-input"
                    width="100%" />
                <div class="add-api-key-popup__form-container__button-container">
                    <Button label="Cancel" width="205px" height="48px" :onPress="onCloseClick" isWhite />
                    <Button label="Create API Key" width="205px" height="48px" :onPress="onCreateClick" />
                </div>
            </div>
            <div class="add-api-key-popup__close-cross-container" v-on:click="onCloseClick">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
                </svg>
            </div>
        </div>
        <CopyApiKeyPopup :apiKey="key" v-if="key" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import VueClipboards from 'vue-clipboards';
import HeaderedInput from '@/components/common/HeaderedInput.vue';
import CopyApiKeyPopup from './CopyApiKeyPopup.vue';
import Button from '@/components/common/Button.vue';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS, API_KEYS_ACTIONS } from '@/utils/constants/actionNames';

Vue.use(VueClipboards);

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
                name: '',
                key: '',
            };
        },
        methods: {
            onCloseClick: function (): void {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_API_KEY);
            },
            onCreateClick: async function (): Promise<any> {
                let result: any = await this.$store.dispatch(API_KEYS_ACTIONS.CREATE, this.$data.name);

                if (!result.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, result.errorMessage);

                    return;
                }

                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Successfully created new api key');
                this.$data.key = result.data.key;
            },
            onChangeName: function (value: string): void {
                this.$data.name = value;
            },
        },
        components: {
            HeaderedInput,
            Button,
            CopyApiKeyPopup
        }
    }
)

export default class AddApiKeyPopup extends Vue {
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
             margin-right: 100px;
             margin-top: 20px;
        }

        &__form-container {
             width: 100%;
             max-width: 440px;
             margin-top: 10px;

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
                line-height: 39px;
                color: #384B65;
                margin-bottom: 35px;
                margin-top: 0;
            }

            &__button-container {
                 width: 100%;
                 display: flex;
                 flex-direction: row;
                 justify-content: space-between;
                 align-items: center;
                 margin-top: 40px;
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

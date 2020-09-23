// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="no-api-keys-area">
        <h1 class="no-api-keys-area__title">Create Your First API Key</h1>
        <p class="no-api-keys-area__sub-title">API keys give access to the project to create buckets, upload objects</p>
        <VButton
            class="no-api-keys-area__button"
            width="180px"
            height="48px"
            label="Create API Key"
            :on-press="onButtonClick"
        />
        <div class="no-api-keys-area__steps-area">
            <div class="no-api-keys-area__steps-area__numbers">
                <FirstStepIcon class="no-api-keys-area__steps-area__numbers__icon"/>
                <div class="no-api-keys-area-divider"></div>
                <SecondStepIcon class="no-api-keys-area__steps-area__numbers__icon"/>
                <div class="no-api-keys-area-divider"></div>
                <ThirdStepIcon class="no-api-keys-area__steps-area__numbers__icon"/>
            </div>
            <div class="no-api-keys-area__steps-area__items">
                <div class="no-api-keys-area__steps-area__items__create-api-key">
                    <h2 class="no-api-keys-area__steps-area__items__create-api-key__title">Create & Save API Key</h2>
                    <img class="no-api-keys-area-image" src="@/../static/images/apiKeys/noApiKeysArea/apiKey.jpg" alt="api key image">
                </div>
                <div class="no-api-keys-area__steps-area__items__setup-uplink">
                    <h2 class="no-api-keys-area__steps-area__items__setup-uplink__title">Setup Uplink CLI</h2>
                    <img class="no-api-keys-area-image" src="@/../static/images/apiKeys/noApiKeysArea/uplink.jpg" alt="setup uplink image">
                </div>
                <div class="no-api-keys-area__steps-area__items__store-data">
                    <h2 class="no-api-keys-area__steps-area__items__store-data__title">Store Data</h2>
                    <img class="no-api-keys-area-image" src="@/../static/images/apiKeys/noApiKeysArea/store.jpg" alt="store data image">
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import FirstStepIcon from '@/../static/images/common/one.svg';
import ThirdStepIcon from '@/../static/images/common/three.svg';
import SecondStepIcon from '@/../static/images/common/two.svg';

@Component({
    components: {
        VButton,
        FirstStepIcon,
        SecondStepIcon,
        ThirdStepIcon,
    },
})
export default class NoApiKeysArea extends Vue {
    @Prop({default: () => { return; }})
    public readonly onButtonClick: Function;
}
</script>

<style scoped lang="scss">
    h1,
    h2,
    p,
    a {
        margin: 0;
    }

    .no-api-keys-area {
        width: auto;
        height: calc(100% - 130px);
        padding: 65px 65px 0 65px;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-size: 32px;
            line-height: 39px;
            color: #1b2533;
            margin-bottom: 25px;
            font-family: 'font_bold', sans-serif;
        }

        &__sub-title {
            font-size: 16px;
            line-height: 21px;
            color: #354049;
            margin-bottom: 38px;
        }

        &__button {
            margin-bottom: 65px;
        }

        &__steps-area {
            padding: 55px;
            width: auto;
            background-color: #fff;
            border-radius: 12px;

            &__numbers {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: space-between;
                padding: 0 125px;
                margin-bottom: 15px;

                &__icon {
                    min-width: 30px;
                    min-height: 30px;
                }
            }

            &__items {
                display: flex;
                justify-content: space-between;
                align-items: center;

                &__create-api-key,
                &__setup-uplink,
                &__store-data {
                    display: flex;
                    flex-direction: column;
                    align-items: center;
                    justify-content: space-between;

                    &__title {
                        font-family: 'font_bold', sans-serif;
                        font-size: 24px;
                        line-height: 16px;
                        text-align: center;
                        color: #354049;
                        margin: 25px 0;
                    }
                }

                &__setup-uplink,
                &__create-api-key {
                    margin-right: 20px;
                }
            }
        }
    }

    .no-api-keys-area-divider {
        width: calc(46% + 2px);
        border: 1px solid #cfd5da;
    }

    @media screen and (max-width: 1590px) {

        .no-api-keys-area {
            padding: 65px 0;

            &__steps-area {

                &__numbers {
                    padding: 0 95px;
                }

                &__items {

                    &__create-api-key,
                    &__setup-uplink,
                    &__store-data {

                        &__title {
                            font-family: 'font_bold', sans-serif;
                            font-size: 19px;
                            line-height: 16px;
                            text-align: center;
                            color: #354049;
                        }
                    }
                }
            }
        }

        .no-api-keys-area-image {
            max-width: 230px;
            max-height: 170px;
        }
    }

    @media screen and (max-width: 900px) {

        .no-api-keys-area {

            &__steps-area {

                &__numbers {
                    padding: 0 75px;
                }

                &__items {

                    &__create-api-key,
                    &__setup-uplink,
                    &__store-data {

                        &__title {
                            font-family: 'font_bold', sans-serif;
                            font-size: 14px;
                            line-height: 16px;
                            text-align: center;
                            color: #354049;
                        }
                    }
                }
            }
        }

        .no-api-keys-area-image {
            max-width: 200px;
            max-height: 140px;
        }
    }
</style>

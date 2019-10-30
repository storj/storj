// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="no-buckets-area">
        <div class="no-buckets-area__header">
            <h1 class="no-buckets-area__header__title">Bucket Usage</h1>
            <p class="no-buckets-area__header__sub-title">Create Buckets & Upload Objects from the Command Line</p>
        </div>
        <div class="no-buckets-area__steps-numbers-area">
            <FirstStepIcon class="no-buckets-area__steps-numbers-area__icon"/>
            <div class="buckets-divider"></div>
            <SecondStepIcon class="no-buckets-area__steps-numbers-area__icon"/>
            <div class="buckets-divider"></div>
            <ThirdStepIcon class="no-buckets-area__steps-numbers-area__icon"/>
        </div>
        <div class="no-buckets-area__steps-area">
            <div class="no-buckets-area__steps-area__setup">
                <h2 class="no-buckets-area__steps-area__setup__title">Set Up The Uplink CLI</h2>
                <div class="no-buckets-area__steps-area__setup__container">
                    <DotsIcon class="dots-icon"/>
                    <p class="no-buckets-area__steps-area__setup__container__address-info">Please enter numeric choice or enter satellite address manually [1]: 1</p>
                    <p class="no-buckets-area__steps-area__setup__container__api-key-info">Enter your API key:</p>
                    <p class="no-buckets-area__steps-area__setup__container__api-key">13YqgbUXRiavXB1qHMtKkHGNHPydtTBgXoZeeofy6aY4n1ewBMDoBS7eV7726nsgVQDfqeL9gQRqjQTyMUGDCFC6yWGPUEHGxJsT</p>
                </div>
            </div>
            <div class="no-buckets-area__steps-area__upload">
                <h2 class="no-buckets-area__steps-area__upload__title">Upload Data Securely</h2>
                <div class="no-buckets-area__steps-area__upload__container">
                    <DotsIcon class="dots-icon"/>
                    <p class="no-buckets-area__steps-area__upload__container__command">./uplink mb sj://sample</p>
                    <p class="no-buckets-area__steps-area__upload__container__command">"Sample" Bucket Created</p>
                    <p class="no-buckets-area__steps-area__upload__container__command">./uplink cp ~/Desktop/your-large-file.mp4 sj://bucket-name</p>
                    <p class="no-buckets-area__steps-area__upload__container__command">"your-large-file.mp4" Successfully uploaded</p>
                </div>
            </div>
            <div class="no-buckets-area__steps-area__usage">
                <h2 class="no-buckets-area__steps-area__usage__title">View Usage Here</h2>
                <UsageIcon class="usage-icon"/>
            </div>
        </div>
        <div class="no-buckets-area__footer">
            <a class="no-buckets-area__footer__button" href="https://documentation.tardigrade.io/api-reference/uplink-cli" target="_blank">
                <ButtonArrowIcon/>
                <p class="no-buckets-area__footer__button__label">Create My First Bucket</p>
            </a>
            <a class="no-buckets-area__footer__help">Why can't I upload from the browser?</a>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import FirstStepIcon from '@/../static/images/buckets/noBucketState/one.svg';
import SecondStepIcon from '@/../static/images/buckets/noBucketState/two.svg';
import ThirdStepIcon from '@/../static/images/buckets/noBucketState/three.svg';
import UsageIcon from '@/../static/images/buckets/noBucketState/usage.svg';
import DotsIcon from '@/../static/images/buckets/noBucketState/dots.svg';
import ButtonArrowIcon from '@/../static/images/buckets/noBucketState/button.svg';

@Component({
    components: {
        VButton,
        FirstStepIcon,
        SecondStepIcon,
        ThirdStepIcon,
        UsageIcon,
        DotsIcon,
        ButtonArrowIcon,
    },
})
export default class NoBucketArea extends Vue {}
</script>


<style scoped lang="scss">
    h1,
    h2,
    p,
    a {
        margin: 0;
    }

    .no-buckets-area {
        padding: 40px 180px 120px 180px;
        font-family: 'font_regular', sans-serif;

        &__header {
            width: 100%;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            margin-bottom: 42px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
                margin-bottom: 24px;
            }

            &__sub-title {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
            }
        }

        &__steps-numbers-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            padding: 0 170px;
            margin-bottom: 14px;

            &__icon {
                min-width: 30px;
            }
        }

        &__steps-area {
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__setup,
            &__upload,
            &__usage {
                display: flex;
                flex-direction: column;
                justify-content: center;

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 24px;
                    line-height: 32px;
                    text-align: center;
                    margin-bottom: 20px;
                    color: #354049;
                }

                &__container {
                    width: 323px;
                    height: 236px;
                    padding: 25px;
                    background-color: #fff;
                    border-radius: 11px;
                    border: 1px solid #EDEDED;

                    &__address-info,
                    &__api-key-info,
                    &__api-key,
                    &__command {
                        font-size: 17px;
                        line-height: 18px;
                        color: #646464;
                    }

                    &__command {
                        margin-bottom: 15px;
                    }

                    &__api-key-info {
                        margin-top: 25px;
                    }

                    &__api-key {
                        margin-top: 10px;
                        word-break: break-all;
                    }
                }
            }
        }

        &__footer {
            display: flex;
            flex-direction: column;
            align-items: center;
            margin-top: 40px;

            &__button {
                display: flex;
                justify-content: center;
                align-items: center;
                width: 250px;
                height: 55px;
                background-color: #2683ff;
                border-radius: 6px;

                &__label {
                    color: #fff;
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    margin: 2px 0 0 10px;
                }

                &:hover {
                    background-color: #0059D0;
                }
            }

            &__help {
                margin-top: 20px;
                font-size: 15px;
                line-height: 18px;
                text-decoration: underline;
                color: #2683ff;
            }
        }
    }

    .buckets-divider {
        width: calc(46% + 2px);
        border: 1px solid #CFD5DA;
    }

    .dots-icon {
        margin-bottom: 36px;
    }

    @media screen and (max-width: 1800px) {
        .no-buckets-area {
            padding: 40px 140px 120px 140px;
        }
    }

    @media screen and (max-width: 1750px) {
        .no-buckets-area {
            padding: 40px 100px 120px 100px;
        }
    }

    @media screen and (max-width: 1680px) {
        .no-buckets-area {
            padding: 40px 150px 120px 150px;

            &__steps-numbers-area {
                padding: 0 145px;
            }

            &__steps-area__setup,
            &__steps-area__upload {

                &__container {
                    width: 277px;
                    height: 201px;

                    &__address-info,
                    &__api-key-info,
                    &__api-key,
                    &__command {
                        font-size: 15px;
                        line-height: 16px;
                    }
                }
            }
        }

        .usage-icon,
        .usage-rect {
            width: 329px;
            height: 253px;
        }

        .dots-icon {
            margin-bottom: 32px;
        }
    }

    @media screen and (max-width: 1630px) {
        .no-buckets-area {
            padding: 40px 120px 120px 120px;
        }
    }

    @media screen and (max-width: 1570px) {
        .no-buckets-area {
            padding: 40px 100px 120px 100px;
        }
    }

    @media screen and (max-width: 1520px) {
        .no-buckets-area {
            padding: 40px 80px 120px 80px;
        }
    }

    @media screen and (max-width: 1470px) {
        .no-buckets-area {
            padding: 40px 65px 120px 65px;
        }
    }

    @media screen and (max-width: 1440px) {
        .no-buckets-area {
            padding: 40px 60px 120px 60px;

            &__steps-numbers-area {
                padding: 0 133px;
            }

            &__steps-area__setup,
            &__steps-area__upload,
            &__steps-area__usage {

                &__title {
                    font-size: 20px;
                    line-height: 16px;
                }

                &__container {
                    width: 240px;
                    height: 173px;

                    &__address-info,
                    &__api-key-info,
                    &__api-key,
                    &__command {
                        font-size: 13px;
                        line-height: 14px;
                    }
                }
            }
        }

        .usage-icon,
        .usage-rect {
            width: 292px;
            height: 225px;
        }

        .dots-icon {
            margin-bottom: 28px;
        }
    }

    @media screen and (max-width: 1320px) {
        .no-buckets-area {
            padding: 40px 50px 120px 50px;
        }
    }

    @media screen and (max-width: 1280px) {
        .no-buckets-area {
            padding: 40px 40px 120px 40px;
        }
    }
</style>

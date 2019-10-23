// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="no-buckets-area">
        <div class="no-buckets-area__header">
            <FolderIcon class="no-buckets-area__header__image"/>
            <h1 class="no-buckets-area__header__title">Buckets</h1>
        </div>
        <p class="no-buckets-area__info">To create a bucket, please use the following CLI documentation. We will soon enable client-side encryption through the browser, at which time you will be able to create buckets and upload objects directly from this dashboard.</p>
        <div class="buckets-divider"></div>
        <p class="no-buckets-area__subtitle">Create Buckets and Upload Objects from the Command Line</p>
        <div class="no-buckets-area__example-container">
            <div class="no-buckets-area__example-container__info">
                <h2 class="no-buckets-area__example-container__info__title">Uplink CLI</h2>
                <p class="no-buckets-area__example-container__info__text">One of the ways you can interact with the network to upload and download files</p>
                <a class="documentation-link" href="https://github.com/storj/storj/wiki/Uplink-CLI" target="_blank">
                    <div class="documentation-link__button">
                        <p class="documentation-link__button__label">Documentation</p>
                    </div>
                </a>
            </div>
            <UplinkIcon/>
        </div>
        <p class="no-buckets-area__s3-link">Familiar with AWS S3? <a class="s3-gateway-link" href="https://github.com/storj/storj/wiki/S3-Gateway" target="_blank">Try our S3 Gateway</a></p>
        <div class="no-buckets-area__next-step-area">
            <div class="no-buckets-area__next-step-area__arrow-image">
                <ArrowIcon/>
            </div>
            <div class="no-buckets-area__next-step-area__text-area">
                <h2 class="no-buckets-area__next-step-area__text-area__title">Next Step</h2>
                <p class="no-buckets-area__next-step-area__text-area__text">After you create a bucket using the CLI, you will be able to see your buckets and usage here. This happens within 1 hour of creating your first bucket.</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import ArrowIcon from '@/../static/images/buckets/noBucketState/arrow.svg';
import FolderIcon from '@/../static/images/buckets/noBucketState/folder.svg';
import UplinkIcon from '@/../static/images/buckets/noBucketState/uplink.svg';

@Component({
    components: {
        VButton,
        FolderIcon,
        UplinkIcon,
        ArrowIcon,
    },
})
export default class NoBucketArea extends Vue {}
</script>


<style scoped lang="scss">
    h1,
    h2,
    p {
        margin: 0;
    }

    .no-buckets-area {
        padding: 50px 60px 50px 70px;
        font-family: 'font_regular';

        &__header {
            width: 100%;
            display: flex;
            justify-content: flex-start;
            align-items: center;

            &__title {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 39px;
                color: #384B65;
            }

            &__image {
                margin-right: 22px;
            }
        }

        &__info {
            font-family: 'font_medium';
            height: 81px;
            display: flex;
            align-items: center;
            justify-content: flex-start;
            font-size: 16px;
            line-height: 21px;
            color: rgba(56, 75, 101, 0.7);
        }

        &__subtitle {
            font-family: 'font_bold';
            font-size: 18px;
            line-height: 27px;
            color: #354049;
            margin-top: 24px;
        }

        &__example-container {
            margin-top: 18px;
            height: 158px;
            width: calc(100% - 86px);
            background-color: #fff;
            border-radius: 6px;
            padding: 43px 46px 43px 40px;
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__info {
                width: 50%;
                height: 100%;
                display: flex;
                flex-direction: column;
                justify-content: space-between;
                align-items: flex-start;

                &__title {
                    font-family: 'font_bold';
                    font-size: 18px;
                    line-height: 27px;
                    color: #354049;
                }

                &__text {
                    font-family: 'font_medium';
                    font-size: 16px;
                    line-height: 21px;
                    color: rgba(56, 75, 101, 0.7);
                }
            }
        }

        &__s3-link {
            font-family: 'font_medium';
            margin-top: 18px;
            font-size: 12px;
            line-height: 17px;
            color: #354049;
        }

        &__next-step-area {
            width: calc(100% - 97px);
            height: 81px;
            padding: 22px 57px 22px 40px;
            margin-top: 35px;
            display: flex;
            justify-content: flex-start;
            align-items: center;
            background-color: rgba(255, 255, 255, 0.5);
            border-radius: 6px;

            &__arrow-image {
                height: 40px;
                min-width: 40px;
                margin-right: 42px;
                background-color: #2683FF;
                border-radius: 10px;
                display: flex;
                justify-content: center;
                align-items: center;
            }

            &__text-area {

                &__title {
                    font-family: 'font_bold';
                    font-size: 18px;
                    line-height: 27px;
                    color: #354049;
                    margin-bottom: 10px;
                }

                &__text {
                    font-family: 'font_medium';
                    font-size: 16px;
                    line-height: 21px;
                    color: rgba(56, 75, 101, 0.7);
                }
            }
        }
    }

    .s3-gateway-link {
        font-family: 'font_medium';
        color: #2683FF;

        &:hover {
            text-decoration: underline;
        }
    }

    .buckets-divider {
        height: 1px;
        background-color: #AFB7C1;
        margin-top: 12px;
    }

    .documentation-link__button {
        width: 205px;
        height: 48px;
        background-color: #2683FF;
        display: flex;
        justify-content: center;
        align-items: center;
        border-radius: 6px;

        .documentation-link__button__label {
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 23px;
            color: #fff;

        }

        &:hover {
            box-shadow: 0 4px 20px rgba(35, 121, 236, 0.4);
        }
    }

    @media screen and (max-width: 1024px) {
        .no-buckets-area {
            padding: 52px 40px 52px 40px;
        }
    }

    @media screen and (min-height:790px) and (max-height: 818px) {
        .no-buckets-area {
            overflow-y: scroll;
            height: 628px;
        }
    }

    @media screen and (min-height:760px) and (max-height: 789px) {
        .no-buckets-area {
            overflow-y: scroll;
            height: 595px;
        }
    }

    @media screen and (min-height:730px) and (max-height: 759px) {
        .no-buckets-area {
            overflow-y: scroll;
            height: 550px;
        }
    }
</style>

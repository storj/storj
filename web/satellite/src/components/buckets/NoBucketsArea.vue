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
            </div>
            <div class="no-buckets-area__steps-area__upload">
                <h2 class="no-buckets-area__steps-area__upload__title">Upload Data Securely</h2>
            </div>
            <div class="no-buckets-area__steps-area__usage">
                <h2 class="no-buckets-area__steps-area__usage__title">View Usage Here</h2>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import FirstStepIcon from '@/../static/images/buckets/noBucketState/one.svg';
import SecondStepIcon from '@/../static/images/buckets/noBucketState/two.svg';
import ThirdStepIcon from '@/../static/images/buckets/noBucketState/three.svg';

@Component({
    components: {
        VButton,
        FirstStepIcon,
        SecondStepIcon,
        ThirdStepIcon,
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
        padding: 50px 60px;

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
            padding: 0 150px;
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
                }
            }
        }
    }

    .buckets-divider {
        width: calc(46% + 2px);
        border: 1px solid #CFD5DA;
    }
</style>

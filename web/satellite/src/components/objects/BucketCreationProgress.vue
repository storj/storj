// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bucket-creation-progress">
        <div class="bucket-creation-progress__item active">
            <div class="bucket-creation-progress__item__outer-circle">
                <div class="bucket-creation-progress__item__inner-circle">
                    <span v-if="creationStep === BucketCreationSteps.Name">1</span>
                    <check-icon v-else />
                </div>
            </div>
            <h4 class="bucket-creation-progress__item__label" :class="{ passed: creationStep >= BucketCreationSteps.Passphrase }">Bucket</h4>
            <div class="bucket-creation-progress__item__divider" />
        </div>
        <div class="bucket-creation-progress__item" :class="{ active: creationStep >= BucketCreationSteps.Passphrase }">
            <div class="bucket-creation-progress__item__outer-circle">
                <div class="bucket-creation-progress__item__inner-circle">
                    <check-icon v-if="creationStep === BucketCreationSteps.Upload" />
                    <span v-else>2</span>
                </div>
            </div>
            <h4 class="bucket-creation-progress__item__label" :class="{ passed: creationStep >= BucketCreationSteps.Passphrase }">Encryption</h4>
            <div class="bucket-creation-progress__item__divider" />
        </div>
        <div class="bucket-creation-progress__item" :class="{ active: creationStep > BucketCreationSteps.Passphrase }">
            <div class="bucket-creation-progress__item__outer-circle">
                <div class="bucket-creation-progress__item__inner-circle">
                    <span>3</span>
                </div>
            </div>
            <h4 class="bucket-creation-progress__item__label">Upload</h4>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { BucketCreationSteps } from '@/components/objects/BucketCreation.vue';

import CheckIcon from '@/../static/images/objects/check.svg';

// @vue/component
@Component({
    components: {
        CheckIcon,
    },
})
export default class BucketCreationProgress extends Vue {
    @Prop({ default: 0 })
    public readonly creationStep: BucketCreationSteps;

    public readonly BucketCreationSteps = BucketCreationSteps;
}
</script>

<style scoped lang="scss">
.bucket-creation-progress {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;

    &__item {
        position: relative;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: flex-start;

        &__outer-circle,
        &__inner-circle {
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 50%;
        }

        &__outer-circle {
            width: 35px;
            height: 35px;
            background: rgb(20 20 42 / 20%);
        }

        &__inner-circle {
            width: 23px;
            height: 23px;
            background: #b1b1b1;

            span {
                font-family: 'font_semiBold', sans-serif;
                font-size: 10px;
                color: white;
            }
        }

        &__label {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 19px;
            color: rgb(20 20 42 / 20%);
            margin-top: 13px;
        }

        &__divider {
            position: absolute;
            width: 130px;
            height: 1px;
            top: 16px;
            left: 100%;
            background: #ebeef1;

            @media screen and (max-width: 760px) {
                display: none;
            }
        }

        &.active {

            .bucket-creation-progress__item__outer-circle {
                background: #d7e8ff;
            }

            .bucket-creation-progress__item__inner-circle {
                background: #0149ff;
            }

            .bucket-creation-progress__item__label {
                color: #0149ff;

                &.passed {
                    color: #14142a !important;
                }
            }
        }
    }
}
</style>

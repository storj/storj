// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bucket-item__functional__message" @click.stop>
        <p class="bucket-item__functional__message__title">Upload</p>
        <p class="bucket-item__functional__message__content">To upload files, open an existing bucket or create a new one.</p>
        <div class="bucket-item__functional__message__actions">
            <div class="bucket-item__functional__message__actions__primary" @click.stop="hideGuide">
                <p class="bucket-item__functional__message__actions__primary__label">OK</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// @vue/component
@Component({})
export default class BucketGuide extends Vue {
    @Prop({ default: null })
    public readonly hideGuide: () => void;

}
</script>

<style scoped lang="scss">
    .bucket-item {

        &__functional {

            &__message {
                position: absolute;
                top: 80%;
                width: 25rem;
                display: flex;
                flex-direction: column;
                align-items: start;
                transform: translateX(31%);
                background-color: #0149ff;
                text-align: center;
                border-radius: 8px;
                box-sizing: border-box;
                padding: 20px;
                z-index: 1001;
                cursor: default;

                @media screen and (max-width: 430px) {
                    width: 18rem;
                    transform: translateX(27%);
                }

                &:after {
                    content: '';
                    position: absolute;
                    bottom: 100%;
                    left: 10%;
                    border-width: 5px;
                    border-style: solid;
                    border-color: #0149ff transparent transparent;
                    transform: rotate(180deg);

                    @media screen and (max-width: 550px) {
                        left: 45%;
                    }
                }

                &__title {
                    font-weight: 400;
                    font-size: 12px;
                    line-height: 18px;
                    color: white;
                    opacity: 0.5;
                    margin-bottom: 4px;
                }

                &__content {
                    color: white;
                    font-weight: 500;
                    font-size: 15px;
                    line-height: 24px;
                    margin-bottom: 8px;
                    white-space: initial;
                    text-align: start;
                }

                &__actions {
                    display: flex;
                    justify-content: end;
                    align-items: center;
                    width: 100%;

                    &__primary {
                        padding: 6px 12px;
                        border-radius: 8px;
                        background-color: white;
                        margin-left: 10px;
                        cursor: pointer;

                        &__label {
                            font-weight: 700;
                            font-size: 13px;
                            line-height: 20px;
                            color: #0149ff;
                        }
                    }
                }
            }
        }
    }
</style>

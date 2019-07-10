// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="empty-state">
        <div class="empty-state__wrap">
            <h1>{{mainTitle}}</h1>
            <div class="empty-state__wrap__additional-text" v-html="additionalText"/>
            <div id="addApiKeysPopupEmptyButton" v-if="isButtonShown">
                <Button
                    :onPress="onButtonClick"
                    :label="buttonLabel"
                    width="190px"
                    height="50px"/>
            </div>
            <div class="empty-state__wrap__img" v-html="imageSource"></div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';

    @Component({
        props: {
            mainTitle: String,
            additionalText: String,
            imageSource: String,
            isButtonShown: {
                type: Boolean,
                default: false
            },
            onButtonClick: Function,
            buttonLabel: String
        },
        components: {
            Button
        }
    })

    export default class EmptyStateProjectArea extends Vue {}
</script>

<style scoped lang="scss">
    .empty-state {
        width: 100%;
        display: flex;
        justify-content: center;
        margin-top: 120px;
        color: rgba(56, 75, 101, 0.7);

        &__wrap {
            text-align: center;
            margin-top: 30px;
            display: flex;
            flex-direction: column;
            align-items: center;

            &__additional-text {
                font-family: 'font_regular';
                font-size: 16px;
                width: 60%;
                margin-bottom: 10px;

                b {
                    font-family: 'font_bold' !important;
                    font-size: 16px;
                }
            }

            &__img {
                margin-top: 50px;
            }

            h1 {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 35px;
                margin-bottom: 15px;
                min-width: 900px;
                color: #354049;
            }

            p {
                font-family: 'font_regular';
                font-size: 16px;
                line-height: 21px;
            }

            img {
                margin-top: 42px;
            }
        }
    }

    @media screen and (max-width: 1440px) {
        .empty-state {
            margin-top: 60px;
        }
    }

    @media screen and (max-width: 1280px) {
        .empty-state {
            margin-top: 20px;
        }
    }
</style>

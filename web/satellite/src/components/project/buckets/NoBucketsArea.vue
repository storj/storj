// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="no-buckets-area">
        <h1 class="no-buckets-area__title">Buckets Usage</h1>
        <div class="no-buckets-area__content-area">
            <img class="no-buckets-area__content-area__image" src="../../../../static/images/buckets/bucket.png" alt="bucket image">
            <h2 class="no-buckets-area__content-area__message">Create your first bucket to get started.</h2>
            <a class="no-buckets-area__content-area__first-button" href="https://documentation.tardigrade.io/api-reference/uplink-cli" target="_blank">
                Get Started
            </a>
            <a class="no-buckets-area__content-area__second-button" href="https://documentation.tardigrade.io/" target="_blank">
                Visit the Docs
            </a>
            <a class="no-buckets-area__content-area__help" href="https://support.tardigrade.io/hc/en-us/articles/360035332472-Why-can-t-I-upload-from-the-browser-" target="_blank">Why can't I upload from the browser?</a>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component
export default class NoBucketArea extends Vue {}
</script>

<style scoped lang="scss">
    h1,
    h2 {
        margin: 0;
    }

    .no-buckets-area {
        padding: 25px 30px;
        font-family: 'font_regular', sans-serif;
        background-color: #fff;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 18px;
            line-height: 18px;
            color: #acacac;
        }

        &__content-area {
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 30px 0;

            &__image {
                margin-right: 10px;
            }

            &__message {
                font-family: 'font_bold', sans-serif;
                font-size: 18px;
                line-height: 26px;
                color: #354049;
                margin: 15px 0 30px 0;
            }

            &__first-button {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 158px;
                height: 49px;
                font-size: 15px;
                line-height: 22px;
                border-radius: 6px;
                background-color: #2683ff;
                color: #fff;
                margin-bottom: 7px;

                &:hover {
                    background-color: #0059d0;
                }
            }

            &__second-button {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 156px;
                height: 47px;
                font-size: 15px;
                line-height: 22px;
                border-radius: 6px;
                background-color: #fff;
                color: #2683ff;
                border: 1px solid #2683ff;

                &:hover {
                    background-color: #2683ff;
                    color: #fff;
                }
            }

            &__help {
                margin-top: 20px;
                font-size: 13px;
                line-height: 18px;
                text-decoration: underline;
                color: #2683ff;
            }
        }
    }
</style>

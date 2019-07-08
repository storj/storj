// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="no-buckets-area">
        <div class="no-buckets-header">
            <img src="../../../static/images/buckets/foldericon.svg">
            <h1 class="no-buckets-header__title">Buckets</h1>
        </div>
        <h3 class="no-buckets-header-info">To create a bucket, please use the following CLI documentation. We will soon enable client-side encryption through the browser, at which time you will be able to create buckets and upload objects directly from this dashboard.</h3>
        <div class="divider"></div>
        <h2 class="no-buckets-subtitle">Create Buckets and Upload Objects from the Command Line</h2>
        <div class="no-buckets-example-container">
            <div class="no-buckets-example-container__info">
                <h2>Uplink CLI</h2>
                <h3>One of the ways you can interact with the network to upload and download files</h3>
                <a class="documentation-link" href="https://github.com/storj/storj/wiki/Uplink-CLI" target="_blank">
                    <div class="documentation-link__button">
                        <p class="documentation-link__button__label">Documentation</p>
                    </div>
                </a>
            </div>
            <img src="../../../static/images/buckets/uplink.svg">
        </div>
        <h4 class="no-buckets-s3-link">Familiar with AWS S3? <a href="https://github.com/storj/storj/wiki/S3-Gateway" target="_blank">Try our S3 Gateway</a></h4>
        <div class="next-step">
            <div class="arrow">
                <svg width="20" height="15" viewBox="0 0 20 15" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M0.916504 6.51758H17.4412V8.64949H0.916504V6.51758Z" fill="#FAFAFD"/>
                    <path d="M12.8575 14.7272L11.3643 13.234L17.0154 7.58383L11.3643 1.93367L12.8575 0.44043L19.9999 7.58385L12.8575 14.7272Z" fill="#FAFAFD"/>
                </svg>
            </div>
            <div class="next-step__text">
                <h2>Next step</h2>
                <h3>After you create a bucket using the CLI, you will be able to see your buckets and usage here. This happens within 10 minutes of creating your first bucket.</h3>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';

    @Component({
        components: {
            Button,
        },
    })

    export default class NoBucketArea extends Vue {}
</script>


<style scoped lang="scss">
    .no-buckets-area {
        padding: 50px 60px 50px 70px;
    }

    h1 {
        font-family: 'font_bold';
        font-size: 32px;
        line-height: 39px;
        color: #384B65;
        margin: 0;
    }

    h2 {
        font-family: 'font_bold';
        font-size: 18px;
        line-height: 27px;
        color: #354049;
        margin: 0;
    }

    h3 {
        font-family: 'font_regular';
        font-size: 16px;
        line-height: 21px;
        color: rgba(56, 75, 101, 0.7);
        margin: 0;
    }

    h4 {
        font-family: 'font_regular';
        font-size: 12px;
        line-height: 17px;
        color: #354049;
        margin: 0;
    }

    a {
        font-family: 'font_medium';
        color: #2683FF;

        &:hover {
            text-decoration: underline;
        }
    }

    .no-buckets-header {
        width: 100%;
        display: flex;
        justify-content: flex-start;
        align-items: center;

        img {
            margin-right: 22px;
        }
    }

    .no-buckets-header-info {
        height: 81px;
        display: flex;
        align-items: center;
        justify-content: flex-start;
    }

    .divider {
        width: 100%;
        height: 1px;
        background-color: #AFB7C1;
        margin-top: 12px;
    }

    .no-buckets-subtitle {
        margin-top: 24px;
    }

    .no-buckets-example-container {
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
        }
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
            box-shadow: 0px 4px 20px rgba(35, 121, 236, 0.4);
        }
    }

    .no-buckets-s3-link {
        margin-top: 18px;
    }

    .next-step {
        width: calc(100% - 97px);
        height: 81px;
        padding: 22px 57px 22px 40px;
        margin-top: 35px;
        display: flex;
        justify-content: flex-start;
        align-items: center;
        background-color: #fff;
        border-radius: 6px;

        .arrow {
            height: 40px;
            min-width: 40px;
            width: 40px;
            margin-right: 42px;
            background-color: #2683FF;
            border-radius: 10px;
            display: flex;
            justify-content: center;
            align-items: center;
        }

        h2 {
            line-height: 35px;
        }
    }

    .next-step__text {
        max-width: 91%;
    }

    @media screen and (max-width: 1024px) {
        .no-buckets-area {
            padding: 52px 40px 52px 34px;
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

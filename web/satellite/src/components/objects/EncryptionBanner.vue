// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="banner">
        <div class="banner__left">
            <SunIcon />
            <p class="banner__left__text">
                The web browser uses
                <a class="banner__left__text__link" href="https://docs.storj.io/concepts/encryption-key/design-decision-server-side-encryption" target="_blank" rel="noopener noreferrer">server-side encryption</a>.<br>
                If you want to use Storj with end-to-end encryption, try the <a class="banner__left__text__link" href="https://docs.storj.io/dcs/getting-started/quickstart-uplink-cli/" target="_blank" rel="noopener noreferrer">Uplink CLI</a>.
            </p>
        </div>
        <div class="banner__right" @click="hide">
            <p class="banner__right__label">Dismiss</p>
            <CloseIcon />
        </div>
    </div>
</template>

<script setup lang="ts">
import SunIcon from '@/../static/images/objects/sun.svg';
import CloseIcon from '@/../static/images/common/closeCross.svg';

const props = withDefaults(defineProps<{
    hide: () => void;
}>(), {
    hide: () => {},
});
</script>

<style scoped lang="scss">
    .banner {
        background: #fff;
        border: 1px solid var(--c-grey-3);
        box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
        border-radius: 10px;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 20px;
        width: calc(100% - 40px);
        font-family: 'font_regular', sans-serif;
        margin-top: 16px;

        &__left {
            display: flex;
            align-items: center;

            @media screen and (width <= 500px) {
                flex-direction: column;
                align-items: flex-start;
            }

            svg {
                min-width: 40px;
            }

            &__text {
                font-size: 14px;
                line-height: 20px;
                color: #000;
                margin-left: 16px;

                @media screen and (width <= 500px) {
                    margin: 10px 0 0;
                }

                &__link {
                    text-decoration: underline !important;
                    text-underline-position: below;
                    color: #000;

                    &:visited {
                        color: #000;
                    }
                }
            }
        }

        &__right {
            display: flex;
            align-items: center;
            cursor: pointer;
            margin-left: 10px;

            &__label {
                text-decoration: underline;
                font-size: 14px;
                line-height: 22px;
                color: #000;
                margin-right: 16px;

                @media screen and (width <= 860px) {
                    display: none;
                }
            }
        }
    }
</style>

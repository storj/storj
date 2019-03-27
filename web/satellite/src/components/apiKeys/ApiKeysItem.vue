// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="apikey-item-container">
        <div class="apikey-item-container__avatar">
            <svg width="30" height="30" viewBox="0 0 30 30" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M0 6C0 2.68629 2.68629 0 6 0H24C27.3137 0 30 2.68629 30 6V24C30 27.3137 27.3137 30 24 30H6C2.68629 30 0 27.3137 0 24V6Z" fill="#2683FF"/>
                <path class="white" d="M11.9184 11C13.4286 11 14.7239 11.8848 15.3777 13.1667H22.0408C22.1155 13.1667 22.1875 13.1921 22.2449 13.2396L23.8776 14.5886C23.9573 14.6537 24.0026 14.7533 23.9994 14.8581C23.9968 14.9623 23.9458 15.0593 23.8623 15.1198L21.5766 16.7708C21.456 16.8581 21.2928 16.8516 21.1786 16.7552L20.4082 16.099L19.6378 16.7552C19.5172 16.8581 19.3406 16.8581 19.2194 16.7552L18.449 16.099L17.6786 16.7552C17.6205 16.8053 17.5465 16.8327 17.4694 16.8333H15.3724C14.7174 18.112 13.426 19 11.9184 19C9.75837 19 8 17.205 8 15C8 12.795 9.75837 11 11.9184 11H11.9184Z" fill="white"/>
                <path d="M10.7754 13.666C11.4928 13.666 12.0815 14.2669 12.0815 14.9993C12.0815 15.7318 11.4928 16.3327 10.7754 16.3327C10.0579 16.3327 9.46924 15.7318 9.46924 14.9993C9.46924 14.2669 10.0579 13.666 10.7754 13.666Z" fill="#2683FF"/>
                <path d="M10.7756 14.334C10.4108 14.334 10.1226 14.6283 10.1226 15.0007C10.1226 15.3731 10.4108 15.6673 10.7756 15.6673C11.1404 15.6673 11.4287 15.3731 11.4287 15.0007C11.4287 14.6283 11.1404 14.334 10.7756 14.334Z" fill="#2683FF"/>
            </svg>
        </div>
        <p class="apikey-item-container__name">{{ apiKeyName }}</p>
        <p class="apikey-item-container__date">{{ new Date(apiKey.createdAt).toLocaleDateString() }}</p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component(
    {
        props: {
            apiKey: Object,
        },
        computed: {
            apiKeyName: function (): string {
                let name = this.$props.apiKey.name;

                if (name.length < 12) {
                    return name;
                }

                return name.slice(0, 12) + '...';
            }
        }
    }
)
export default class ApiKeysItem extends Vue {}
</script>

<style scoped lang="scss">
   .apikey-item-container {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        border-radius: 6px;
        height: 180px;
        background-color: #fff;
        cursor: pointer;
        transition: box-shadow .2s ease-out;

        &:hover {
            box-shadow: 0px 12px 24px rgba(175, 183, 193, 0.4);
        }

        &:last-child {
            margin-left: 0;
        }

        &__date {
            font-family: 'font_regular';
            font-size: 12px;
            line-height: 16px;
            color: #AFB7C1;
            margin: 0;
        }

        &__name {
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 21px;
            color: #354049;
            margin-top: 20px;
        }

        &__avatar {
            min-width: 40px;
            max-width: 40px;
            min-height: 40px;
            max-height: 40px;
            display: flex;
            align-items: center;
            justify-content: center;
        }
    }
    .apikey-item-container.selected {
        box-shadow: 0px 12px 24px rgba(38, 131, 255, 0.4);
        background-color: #2683FF;

        p {

            &:nth-child(2) {
                color: #fff;
            }

            &:nth-child(3) {
                color: #fff;
            }

            &:nth-child(4) {
                color: #fff;
            }

            &:nth-child(5) {
                color: #fff;
            }
        }

        svg path {
            fill: white;
        }

        .white {
            fill: #2683FF;
        }
    }
</style>

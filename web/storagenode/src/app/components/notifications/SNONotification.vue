// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notification-item">
        <div class="row">
            <div class="notification-item__new-indicator-container">
                <span class="notification-item__new-indicator-container__circle"></span>
            </div>
            <div class="notification-item__icon-container">

            </div>
            <div class="notification-item__text-container">
                <p>Software Update Required: Your node software version 0.12.1 is out of date.</p>
            </div>
        </div>
        <div class="notification-item__date-container">
            <p class="notification-item__date-container__date">1 hour ago</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

@Component
export default class SNONotification extends Vue {
    @Prop({default: () => ({})})
    public notification: any;
}
</script>

<style scoped lang="scss">
    .notification-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        background-color: transparent;
        padding: 17px 24px 18px 15px;
        width: calc(100% - 24px - 15px);

        &__new-indicator-container {
            display: flex;
            align-items: center;
            justify-items: center;
            width: 6px;
            height: 6px;
            min-width: 6px;
            min-height: 6px;
            margin-right: 11px;

            &__circle {
                display: inline-block;
                border-radius: 50%;
                background-color: #2683ff;
                width: 100%;
                height: 100%;
            }
        }

        &__icon-container {
            display: inline-block;
            border-radius: 50%;
            background-color: #f3f4f9;
            width: 40px;
            height: 40px;
            min-width: 40px;
            min-height: 40px;
            margin-right: 17px;
        }

        &__text-container {

            &__message {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: #535f77;
                text-align: left;
            }
        }

        &__date-container {
            margin-left: 20px;
            min-width: 60px;

            &__date {
                font-family: 'font_regular', sans-serif;
                font-size: 12px;
                color: #586c86;
                text-align: right;
            }
        }
    }

    .row {
        display: flex;
        align-items: center;
    }
</style>

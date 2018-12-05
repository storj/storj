// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="notification-container" >
            <div class="notification-container__text">
                <img src="../../../../static/images/team/info.svg" alt="">
                <p>All team members succesfully deleted</p>
            </div>
            <div class="notification-container__buttons-group">
                <span class="notification-container__buttons-group__close"></span>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component({})

export default class Notification extends Vue {}
</script>

<style scoped lang="scss">
    .notification-container {
        height: 98px;
        max-width: 74.2%;
        width: 100%;
        background-color: #fff;
        display: flex;
        position: fixed;
        bottom: 100px;
        align-items: center;
        justify-content: space-between;
        padding: 0 50px;
        background: rgba(194, 214, 241, 1);
        box-shadow: 0px 12px 24px rgba(175, 183, 193, 0.4);
        border-radius: 6px;
        z-index: 999;

        &__text {
            display: flex;
            align-items: center;

            p {
                font-family: 'montserrat_medium';
                font-size: 16px;
                margin-left: 40px;
            }
        }

        &__buttons-group {
            display: flex;
            
            &__close {
                width: 32px;
                height: 32px;
                background-image: url('../../../../static/images/team/close.svg');
            }
        }
    }
</style>
// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="blur-wrap"></div>
        <div class="delete-user-container" >
            <div class="delete-user-container__text">
                <img src="../../../../static/images/team/atten.svg" alt="">
                <p>Are you sure you want tot delete <span>1</span> user?</p>
            </div>
            <div class="delete-user-container__buttons-group">
                <span class="delete-user-container__buttons-group__cancel">Cancel</span>
                <span class="delete-user-container__buttons-group__delete">Delete</span>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component({})

export default class ApproveDeleteUserArea extends Vue {}
</script>

<style scoped lang="scss">
    .delete-user-container {
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
            
            span {
                width: 142px;
                padding: 14px 0;
                display: flex;
                align-items: center;
                justify-content: center;
                border-radius: 6px;
                font-family: 'montserrat_medium';
                font-size: 16px;
                cursor: pointer;
            }

            &__cancel {
                color: #354049;
                border: 1px solid #354049;
                margin-right: 24px;
            }

            &__delete {
                color: #fff;
                border: 1px solid #2683FF;
                background: #2683FF;
            }
        }
    }
    .blur-wrap {
        position: fixed;
        height: 100vh;
        top: 0;
        left: 0;
        z-index: 99;
        width: 100%;
        filter: blur(4px);
        background-color: rgba(0,0,0,0.5);
    }
</style>
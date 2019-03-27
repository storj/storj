// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="blur-wrap"></div>
        <div class="delete-api-key-container" >
            <div class="delete-api-key-container__text">
                <img src="../../../../static/images/team/atten.svg" alt="">
                <p>Are you sure you want to delete <span>1</span> API key?</p>
            </div>
            <div class="delete-api-key-container__buttons-group">
                <Button 
                    class="delete-api-key-container__buttons-group__cancel" 
                    label="Cancel" 
                    width="140px" 
                    height="48px"
                    :onPress="onCancelButtonClick" 
                    isWhite />
                <Button 
                    label="Delete" 
                    width="140px" 
                    height="48px"
                    :onPress="onDeleteButtonClick" />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';

@Component({
    components: {
        Button
    }
})

export default class ApproveDeleteApiKeysArea extends Vue {
}
</script>

<style scoped lang="scss">
    .delete-api-key-container {
        height: 98px;
        max-width: 74.2%;
        width: 100%;
        background-color: #fff;
        display: flex;
        position: fixed;
        bottom: 100px;
        align-items: center;
        justify-content: space-between;
        padding: 0 32px;
        box-shadow: 0px 12px 24px rgba(175, 183, 193, 0.4);
        border-radius: 6px;
        z-index: 1000;

        &__text {
            display: flex;
            align-items: center;

            p {
                font-family: 'font_medium';
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
                font-family: 'font_medium';
                font-size: 16px;
                cursor: pointer;
            }

            &__cancel {
                margin-right: 24px;
            }
        }
    }
    .blur-wrap {
        position: fixed;
        height: 100vh;
        top: 0;
        left: 0;
        z-index: 1000;
        width: 100%;
        filter: blur(4px);
        background-color: rgba(0,0,0,0.5);
    }

    @media screen and (max-width: 1600px) {
        .delete-api-key-container {
            max-width: 72%;
        }
    }

    @media screen and (max-width: 1366px) {
        .delete-api-key-container {
            max-width: 65%;
        }
    }

    @media screen and (max-width: 1120px) {
        .delete-api-key-container {
            max-width: 60%;
        }
    }

    @media screen and (max-width: 1025px) {
        .delete-api-key-container {
            max-width: 73%;
        }
    }
</style>
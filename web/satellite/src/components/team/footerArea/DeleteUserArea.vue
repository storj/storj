// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="delete-user-container" >
        <div class="delete-user-container__wrap">
            <div class="delete-user-container__selected-users-count">
                <span class="delete-user-container__selected-users-count__button"></span>
                <p class="delete-user-container__selected-users-count__count">1 </p>
                <p class="delete-user-container__selected-users-count__total-count"> of <span>8</span> Users Selected</p>
            </div>
            <div class="delete-user-container__buttons-group">
                <Button 
                    class="delete-user-container__buttons-group__cancel" 
                    label="Cancel" 
                    width="140px" 
                    height="58px" 
                    :onPress="onCancelButtonClick" 
                    isWhite />
                <Button 
                    label="Delete" 
                    width="140px" 
                    height="58px" 
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

export default class DeleteUserArea extends Vue {}
</script>

<style scoped lang="scss">
    .delete-user-container {
        padding-bottom: 50px;
        position: fixed;
        bottom: 0px;
        max-width: 79.7%;
        width: 100%;

        &__wrap {
            padding: 0 50px;
            height: 98px;
            background-color: #fff;
            display: flex;
            align-items: center;
            justify-content: space-between;
            box-shadow: 0px 12px 24px rgba(175, 183, 193, 0.4);
            border-radius: 6px;
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
                margin-right: 24px;
            }
        }

        &__selected-users-count {
            display: flex;
            align-items: center;
            font-family: 'montserrat_regular';
            font-size: 18px;
            color: #AFB7C1;

            &__count {
                margin: 0 7px;
            }

            &__button {
                height: 16px;
                display: block;
                cursor: pointer;
                width: 16px;
                background-image: url('../../../../static/images/team/delete.svg');
            }
        }
    }
    @media screen and (max-width: 1600px) {
        .delete-user-container {
            max-width: 74%;
        }
    }

    @media screen and (max-width: 1366px) {
        .delete-user-container {
            max-width: 72%;
        }
    }

    @media screen and (max-width: 1120px) {
        .delete-user-container {
            max-width: 82.7%;
        }
    }
</style>
// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="delete-user-container" >
        <div class="delete-user-container__selected-users-count">
            <span class="delete-user-container__selected-users-count__button"></span>
            <p class="delete-user-container__selected-users-count__count">1 </p>
            <p class="delete-user-container__selected-users-count__total-count"> of <span>8</span> Users Selected</p>
        </div>
        <div class="delete-user-container__buttons-group">
            <span class="delete-user-container__buttons-group__cancel">Cancel</span>
            <span class="delete-user-container__buttons-group__delete">Delete</span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component({})

export default class DeleteUserArea extends Vue {}
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
</style>
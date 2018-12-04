// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="user-container">
        <div class="user-container__avatar">
            <h1>m</h1>
        </div>
        <p class="user-container__company-name"> Test</p>
        <p class="user-container__user-name">Larry Test</p>
        <p class="user-container__user-email">larry@mail.com</p>
        <p class="user-container__date">23 june 2018</p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component({
    props: {
        userInfo: Object
    }
})

export default class TeamMemberItem extends Vue {}
</script>

<style scoped lang="scss">
    .user-container {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        border-radius: 6px;
        height: 180px;
        background-color: #fff;
        margin-bottom: 24px;
        padding: 30px 0;
        cursor: pointer;
        transition: box-shadow .2s ease-out;

        &:hover {
            box-shadow: 0px 12px 24px rgba(175, 183, 193, 0.4);
        }

        &:last-child {
            margin-left: 0;
        }

        &__date {
            font-family: 'montserrat_regular';
            font-size: 12px;
            line-height: 16px;
            color: #AFB7C1;
            margin-top: 10px;
            margin-bottom: 0;
        }

        &__user-email {
            font-family: 'montserrat_regular';
            font-size: 14px;
            line-height: 19px;
            color: #AFB7C1;
            margin-top: 0;
            margin-bottom: 15px;
        }

        &__company-name {
            font-family: 'montserrat_bold';
            font-size: 16px;
            color: #354049;
            margin-top: 20px;
            margin-bottom: 10px;
        }

        &__user-name {
            font-family: 'montserrat_regular';
            font-size: 14px;
            line-height: 19px;
            color: #AFB7C1;
            margin: 0;
        }

        &__avatar {
            min-width: 40px;
            max-width: 40px;
            min-height: 40px;
            max-height: 40px;
            border-radius: 6px;
            display: flex;
            align-items: center;
            justify-content: center;
            background-color: #FF8658;
            h1 {
                font-family: 'montserrat_medium';
                font-size: 16px;
                line-height: 23px;
                color: #fff;
            }
        }
    }
    .user-container.selected {
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
    }
</style>
// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="empty-state">
        <div class="empty-state__wrap">
            <h1>{{mainTitle}}</h1>
            <p>{{additionalText}}</p>
            <Button 
                v-if="isButtonShown" 
                @onPress="onButtonClick"
                :label="buttonLabel"
                width="170px" />
            <div class="empty-state__wrap__img" v-html="imageSource"></div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Button from '@/components/common/Button.vue';

    @Component({
        props: {
            mainTitle: String,
            additionalText: String,
            imageSource: String,
            isButtonShown: {
                type: Boolean,
                default: false
            },
            onButtonClick: Function,
            buttonLabel: String
        },
        components: {
            Button
        }
    })

    export default class EmptyStateProjectArea extends Vue {}
</script>

<style scoped lang="scss">
    .empty-state {
        width: 100%;
        display: flex;
        justify-content: center;
        margin-top: 120px;

        &__wrap {
             text-align: center;
             margin-top: 30px;

             &__img {
                 margin-top: 90px;
             }

            h1 {
                font-family: 'montserrat_bold';
                font-size: 32px;
                line-height: 35px;
                margin-bottom: 30px;
            }

            p {
                font-family: 'montserrat_regular';
                font-size: 16px;
                line-height: 21px;
            }

            img {
                margin-top: 42px;
            }
        }
    }

    @media screen and (max-width: 14400px) {
        .empty-state {
            margin-top: 60px;
        }
    }

    @media screen and (max-width: 1280px) {
        .empty-state {
            margin-top: 20px;
        }
    }
</style>

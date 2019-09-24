// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="empty-state">
        <div class="empty-state__wrap">
            <h1 class="empty-state__wrap__title">{{mainTitle}}</h1>
            <div class="empty-state__wrap__additional-text" v-html="additionalText"></div>
            <div v-if="isButtonShown">
                <Button
                    :on-press="onButtonClick"
                    :label="buttonLabel"
                    width="190px"
                    height="50px"/>
            </div>
            <div class="empty-state__wrap__img" v-html="imageSource"></div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import Button from '@/components/common/VButton.vue';

@Component({
    components: {
        Button,
    },
})
export default class EmptyStateProjectArea extends Vue {
    @Prop({default: ''})
    private readonly mainTitle: string;

    @Prop({default: ''})
    private readonly additionalText: string;

    @Prop({default: ''})
    private readonly imageSource: string;

    @Prop({default: false})
    private readonly isButtonShown: boolean;

    @Prop()
    private readonly onButtonClick: Function;

    @Prop({default: 'Default'})
    private readonly buttonLabel: string;
}
</script>

<style scoped lang="scss">
    .empty-state {
        width: 100%;
        display: flex;
        justify-content: center;
        margin-top: 100px;
        color: rgba(56, 75, 101, 0.7);
        font-family: 'font_regular';

        &__wrap {
            text-align: center;
            margin-top: 30px;
            display: flex;
            flex-direction: column;
            align-items: center;

            &__additional-text {
                font-family: 'font_medium';
                font-size: 16px;
                width: 60%;
                margin-bottom: 10px;
            }

            &__img {
                margin-top: 50px;
            }

            &__title {
                font-family: 'font_bold';
                font-size: 32px;
                line-height: 35px;
                margin-bottom: 15px;
                min-width: 900px;
                color: #354049;
            }
        }
    }

    @media screen and (max-width: 1440px) {
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

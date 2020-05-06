// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="checks-area-container">
        <div class="checks-area-container__header">
            <p class="checks-area-container__header__title">{{label}}</p>
            <VInfo
                :text="infoText"
                is-extra-padding="true"
                is-custom-position="true"
            >
                <div>
                    <ChecksInfoIcon
                        class="checks-area-image"
                        alt="Blue info icon with question mark"
                    />
                </div>
            </VInfo>
        </div>
        <p class="checks-area-container__amount"><b>{{value}}%</b></p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VInfo from '@/app/components/VInfo.vue';

import ChecksInfoIcon from '@/../static/images/checksInfo.svg';

@Component ({
    components: {
        VInfo,
        ChecksInfoIcon,
    },
})
export default class ChecksArea extends Vue {
    @Prop({default: ''})
    private readonly label: string;
    @Prop({default: ''})
    private readonly amount: number;
    @Prop({default: ''})
    private readonly infoText: string;

    public get value(): string {
        return this.amount.toFixed(1);
    }
}
</script>

<style scoped lang="scss">
    .checks-area-container {
        width: calc(48% - 60px);
        height: 79px;
        background-color: var(--block-background-color);
        border: 1px solid #e9eff4;
        border-radius: 11px;
        padding: 32px 30px;
        margin-bottom: 13px;
        position: relative;

        &__header {
            display: flex;
            align-items: center;

            &__title {
                font-size: 14px;
                line-height: 21px;
                color: #586c86;
                margin: 0 5px 0 0;
            }

            .checks-area-image {
                margin-top: 3px;
                cursor: pointer;

                &:hover {

                    .checks-area-svg-rect {
                        fill: #a5c7ef;
                    }
                }
            }
        }

        &__amount {
            font-size: 32px;
            line-height: 57px;
            color: #535f77;
            margin: 0;
        }
    }

    /deep/ .info__message-box {
        min-width: 190px;
        white-space: normal;
    }

    @media screen and (max-width: 460px) {

        .checks-area-image {
            display: none;
        }
    }
</style>

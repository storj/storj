// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="apikey-item-container">
        <div class="apikey-item-container__common-info">
            <div class="checkbox-container">
                <CheckboxIcon class="checkbox-container__image"/>
            </div>
            <div class="avatar">
                <AvatarIcon class="avatar__image"/>
            </div>
            <div :title="itemData.name">
                <p class="name">{{ itemData.formattedName() }}</p>
            </div>
        </div>
        <div class="apikey-item-container__common-info">
            <p class="date">{{ itemData.getDate() }}</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import AvatarIcon from '@/../static/images/apiKeys/avatar.svg';
import CheckboxIcon from '@/../static/images/common/checkbox.svg';

import { ApiKey } from '@/types/apiKeys';

@Component({
    components: {
        CheckboxIcon,
        AvatarIcon,
    },
})
export default class ApiKeysItem extends Vue {
    @Prop({default: () => new ApiKey('', '', '', '')})
    private readonly itemData: ApiKey;
}
</script>

<style scoped lang="scss">
    .apikey-item-container {
        display: flex;
        align-items: center;
        justify-content: flex-start;
        height: 83px;
        background-color: #fff;
        cursor: pointer;
        width: 100%;

        &:hover {
            background-color: rgba(255, 255, 255, 0.5);
        }

        &__common-info {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            width: 40%;

            .checkbox-container {
                margin-left: 28px;
                max-height: 23px;
                border-radius: 4px;
            }

            .avatar {
                min-width: 40px;
                max-width: 40px;
                min-height: 40px;
                max-height: 40px;
                display: flex;
                align-items: center;
                justify-content: center;
                margin-left: 25px;
            }

            .name {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                margin-left: 17px;
            }
        }

        .date {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            line-height: 21px;
            color: #354049;
            margin: 0;
        }
    }

    .apikey-item-container.selected {
        background-color: #2683ff;

        .apikey-item-container__common-info {

            .checkbox-container {
                background-image: url('../../../static/images/apiKeys/Vector.png');
                background-repeat: no-repeat;
                background-size: 18px 12px;
                background-position: center;
                background-color: #fff;

                &__image {

                    &__rect {
                        stroke: #fff;
                    }
                }
            }

            .avatar {

                &__image {
                    border: 1px solid #fff;
                    box-sizing: border-box;
                    border-radius: 6px;
                }
            }
        }

        .name,
        .date {
            color: #fff;
        }
    }
</style>

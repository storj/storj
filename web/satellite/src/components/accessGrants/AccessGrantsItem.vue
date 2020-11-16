// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="grants-item-container">
        <div class="grants-item-container__common-info">
            <div class="checkbox-container">
                <CheckboxIcon class="checkbox-container__image"/>
            </div>
            <div class="name-container" :title="itemData.name">
                <p class="name">{{ itemData.name }}</p>
            </div>
        </div>
        <div class="grants-item-container__common-info date-item-container">
            <p class="date">{{ itemData.localDate() }}</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import CheckboxIcon from '@/../static/images/common/checkbox.svg';

import { AccessGrant } from '@/types/accessGrants';

@Component({
    components: {
        CheckboxIcon,
    },
})
export default class AccessGrantsItem extends Vue {
    @Prop({ default: new AccessGrant('', '', new Date(), '') })
    private readonly itemData: AccessGrant;
}
</script>

<style scoped lang="scss">
    .grants-item-container {
        display: flex;
        align-items: center;
        justify-content: flex-start;
        height: 83px;
        background-color: #fff;
        cursor: pointer;
        width: 100%;

        &:hover {
            background-color: rgba(242, 244, 247, 0.6);
        }

        &__common-info {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            width: 60%;
        }
    }

    .checkbox-container {
        margin-left: 28px;
        max-height: 23px;
        border-radius: 4px;
    }

    .name-container {
        max-width: calc(100% - 131px);
        margin-right: 15px;
    }

    .name {
        font-family: 'font_bold', sans-serif;
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        margin-left: 17px;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .date {
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        line-height: 21px;
        color: #354049;
        margin: 0;
    }

    .grants-item-container.selected {
        background-color: rgba(242, 244, 247, 0.6);

        .grants-item-container__common-info {

            .checkbox-container {
                background-image: url('../../../static/images/accessGrants/vector.png');
                background-repeat: no-repeat;
                background-size: 18px 12px;
                background-position: center;
                background-color: #0068dc;

                &__image {

                    &__rect {
                        stroke: #fff;
                    }
                }
            }
        }
    }

    .date-item-container {
        width: 40%;
    }
</style>

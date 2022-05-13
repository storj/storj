// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="grants-item-container">
        <div class="grants-item-container__common-info">
            <div class="checkbox-container">
                <CheckboxIcon class="checkbox-container__image" />
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

import CheckboxIcon from '@/../static/images/accessGrants/vector.svg';

import { AccessGrant } from '@/types/accessGrants';

// @vue/component
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
            background-color: rgb(242 244 247 / 60%);
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
        min-width: 21px;
        min-height: 21px;
        border-radius: 4px;
        border: 1px solid #1b2533;

        &__image {
            display: none;
        }
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
        background-color: rgb(242 244 247 / 60%);

        .grants-item-container__common-info {

            .checkbox-container {
                border: none;

                &__image {
                    display: block;
                }
            }
        }
    }

    .date-item-container {
        width: 40%;
    }

</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="user-container">
        <div class="user-container__base-info">
            <div class="checkbox" >
            </div>
            <div class="user-container__base-info__avatar" :style="avatarData.style">
                <h1>{{avatarData.letter}}</h1>
            </div>
            <p class="user-container__base-info__user-name">{{this.itemData.formattedFullName()}}</p>
        </div>
        <p class="user-container__date">{{this.itemData.joinedAtLocal()}}</p>
        <p class="user-container__user-email">{{this.itemData.formattedEmail()}}</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { ProjectMember } from '@/types/projectMembers';
import { getColor } from '@/utils/avatarColorManager';

@Component
export default class ProjectMemberListItem extends Vue {
    @Prop({default: new ProjectMember('', '', '', '', '')})
    public itemData: ProjectMember;

    public get avatarData(): object {
        const fullName: string = this.itemData.user.getFullName();

        const letter = fullName.slice(0, 1).toLocaleUpperCase();

        const style = {
            background: getColor(letter),
        };

        return {
            letter,
            style,
        };
    }
}
</script>

<style scoped lang="scss">
    .user-container {
        margin-top: 2px;
        display: flex;
        flex-direction: row;
        align-items: center;
        padding-left: 28px;
        height: 83px;
        background-color: #fff;
        cursor: pointer;
        width: calc(100% - 28px);

        &__base-info {
            width: 50%;
            display: flex;
            align-items: center;
            justify-content: flex-start;

            &__avatar {
                min-width: 40px;
                max-width: 40px;
                min-height: 40px;
                max-height: 40px;
                margin-left: 20px;
                border-radius: 6px;
                display: flex;
                align-items: center;
                justify-content: center;
                background-color: #FF8658;

                h1 {
                    font-size: 16px;
                    font-family: 'font_regular';
                    color: #F5F6FA;
                }
            }

            &__user-name {
                width: 100%;
                margin-left: 20px;
                font-size: 16px;
                font-family: 'font_bold';
                color: #354049;
            }
        }

        &__date {
            width: 25%;
            font-family: 'font_regular';
            font-size: 16px;
            color: #354049;
        }

        &__user-email {
            width: 25%;
            font-family: 'font_regular';
            font-size: 16px;
            color: #354049;
        }
    }


    .checkbox {
        background-image: url("../../../static/images/team/checkboxEmpty.svg");
        min-width: 23px;
        height: 23px;
    }


    .user-container.selected {
        background-color: #2683FF;

        .checkbox {
            min-width: 23px;
            height: 23px;
            background-image: url("../../../static/images/team/checkboxChecked.svg");
        }

        h1 {
            font-size: 16px;
            font-family: 'font_regular';
            color: #FFFFFF;
        }

        p {
            color: #FFFFFF;
        }
    }
</style>

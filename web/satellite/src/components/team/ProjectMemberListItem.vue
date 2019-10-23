// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="user-container">
        <div class="user-container__base-info">
            <div class="checkbox"></div>
            <div class="user-container__base-info__avatar" :style="avatarData.style">
                <h1 class="user-container__base-info__avatar__letter">{{avatarData.letter}}</h1>
            </div>
            <div class="user-container__base-info__name-area">
                <p class="user-container__base-info__name-area__user-name">{{itemName}}</p>
                <p v-if="isProjectOwner" class="user-container__base-info__name-area__owner-status">Project Owner</p>
            </div>
        </div>
        <p class="user-container__date">{{itemDate}}</p>
        <p class="user-container__user-email">{{itemEmail}}</p>
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

    public get itemName(): string {
        return this.itemData.formattedFullName();
    }

    public get itemDate(): string {
        return this.itemData.joinedAtLocal();
    }

    public get itemEmail(): string {
        return this.itemData.formattedEmail();
    }

    public get isProjectOwner(): boolean {
        return this.itemData.user.id === this.$store.getters.selectedProject.ownerId;
    }
}
</script>

<style scoped lang="scss">
    .user-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        padding-left: 28px;
        height: 83px;
        background-color: #fff;
        cursor: pointer;
        width: calc(100% - 28px);
        font-family: 'font_regular';

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

                &__letter {
                    margin: 0;
                    font-size: 16px;
                    color: #F5F6FA;
                }
            }

            &__name-area {

                &__user-name {
                    margin: 0 0 0 20px;
                    font-size: 16px;
                    font-family: 'font_bold';
                    color: #354049;
                }

                &__owner-status {
                    margin: 0 0 0 20px;
                    font-size: 13px;
                    color: #AFB7C1;
                    font-family: 'font_medium';
                }
            }
        }

        &__date {
            width: 25%;
            font-size: 16px;
            color: #354049;
        }

        &__user-email {
            width: 25%;
            font-size: 16px;
            color: #354049;
        }
    }

    .checkbox {
        background-image: url("../../../static/images/team/checkboxEmpty.png");
        min-width: 23px;
        height: 23px;
    }

    .user-container.selected {
        background-color: #2683FF;

        .checkbox {
            min-width: 23px;
            height: 23px;
            background-image: url("../../../static/images/team/checkboxChecked.png");
        }

        .user-container__base-info__name-area__user-name,
        .user-container__base-info__name-area__owner-status,
        .user-container__date,
        .user-container__user-email {
            color: #FFFFFF;
        }
    }
</style>

// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="user-container" :class="{ 'owner': isProjectOwner }">
        <div class="user-container__base-info">
            <div v-if="!isProjectOwner" class="checkbox"></div>
            <div class="user-container__base-info__avatar" :class="{ 'extra-margin': isProjectOwner }" :style="avatarData.style">
                <h1 class="user-container__base-info__avatar__letter">{{avatarData.letter}}</h1>
            </div>
            <div class="user-container__base-info__name-area" :title="itemData.name">
                <p class="user-container__base-info__name-area__user-name">{{ itemData.name }}</p>
                <p v-if="isProjectOwner" class="user-container__base-info__name-area__owner-status">Project Owner</p>
            </div>
        </div>
        <p class="user-container__date">{{ itemData.localDate() }}</p>
        <p class="user-container__user-email" :title="itemData.email">{{ itemData.email }}</p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { ProjectMember } from '@/types/projectMembers';
import { getColor } from '@/utils/avatarColorManager';

@Component
export default class ProjectMemberListItem extends Vue {
    @Prop({default: new ProjectMember('', '', '', new Date(), '')})
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
        font-family: 'font_regular', sans-serif;

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
                border-radius: 6px;
                display: flex;
                align-items: center;
                justify-content: center;
                background-color: #ff8658;
                margin-left: 20px;

                &__letter {
                    margin: 0;
                    font-size: 16px;
                    color: #f5f6fa;
                }
            }

            &__name-area {
                max-width: calc(100% - 100px);
                margin-right: 15px;

                &__user-name {
                    margin: 0 0 0 20px;
                    font-size: 16px;
                    font-family: 'font_bold', sans-serif;
                    color: #354049;
                    white-space: nowrap;
                    overflow: hidden;
                    text-overflow: ellipsis;
                }

                &__owner-status {
                    margin: 0 0 0 20px;
                    font-size: 13px;
                    color: #afb7c1;
                    font-family: 'font_medium', sans-serif;
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
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }
    }

    .checkbox {
        background-image: url('../../../static/images/team/checkboxEmpty.png');
        min-width: 23px;
        height: 23px;
    }

    .user-container.selected {
        background-color: #2683ff;

        .checkbox {
            min-width: 23px;
            height: 23px;
            background-image: url('../../../static/images/team/checkboxChecked.png');
        }

        .user-container__base-info__name-area__user-name,
        .user-container__base-info__name-area__owner-status,
        .user-container__date,
        .user-container__user-email {
            color: #fff;
        }
    }

    .owner {
        cursor: default;
    }

    .extra-margin {
        margin-left: 43px;
    }
</style>

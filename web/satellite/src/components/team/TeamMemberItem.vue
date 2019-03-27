// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="user-container">
        <div class="user-container__avatar" :style="avatarData.style">
            <h1>{{avatarData.letter}}</h1>
        </div>
        <p class="user-container__user-name">{{userInfo.fullName}}</p>
        <p class="user-container__user-email">{{userInfo.email}}</p>
        <p class="user-container__date">{{new Date(projectMember.joinedAt).toLocaleDateString()}}</p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { getColor } from '@/utils/avatarColorManager';

@Component({
    props: {
        projectMember: Object,
    },
    computed: {
        userInfo: function (): object { 
            let fullName = getFullName(this.$props.projectMember.user);

            let email: string = this.$props.projectMember.user.email;

            if (fullName.length > 16) {
                fullName = fullName.slice(0, 13) + '...';
            }

            if (email.length > 16) {
                email = this.$props.projectMember.user.email.slice(0, 13) + '...';
            }

            return { fullName, email };
        },
        avatarData: function (): object {
            let fullName = getFullName(this.$props.projectMember.user);

            const letter = fullName.slice(0, 1).toLocaleUpperCase();

            const style = {
                background: getColor(letter)
            };

            return {
                letter,
                style
            };
        }
    }
})

export default class TeamMemberItem extends Vue {
}

function getFullName(user: any): string {
    return user.shortName === '' ? user.fullName : user.shortName;
}
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
            font-family: 'font_regular';
            font-size: 12px;
            line-height: 16px;
            color: #AFB7C1;
            margin-top: 10px;
            margin-bottom: 0;
        }

        &__user-email {
            font-family: 'font_regular';
            font-size: 14px;
            line-height: 19px;
            color: #AFB7C1;
            margin-top: 0;
            margin-bottom: 15px;
        }

        &__user-name {
            font-family: 'font_bold';
            font-size: 14px;
            line-height: 19px;
            color: #354049;
            margin-top: 20px;
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
                font-family: 'font_medium';
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
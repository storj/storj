// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-container" id="sortTeamMemberByDropdownButton">
        <!-- TODO: fix dd styles on hover -->
        <div class="sort-toggle-container" v-on:click="toggleSelection" >
            <h1 class="sort-toggle-container__sort-name">Sort by {{sortOption}}</h1>
            <div class="sort-toggle-container__expander-area">
                <img v-if="!isChoiceShown" src="../../../../static/images/register/BlueExpand.svg" />
                <img v-if="isChoiceShown" src="../../../../static/images/register/BlueHide.svg" />
            </div>
        </div>
        <SortDropdown v-if="isChoiceShown" @onClose="toggleSelection" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import SortDropdown from './SortDropdown.vue';
import { mapState } from 'vuex';
import { ProjectMemberSortByEnum } from '@/utils/constants/ProjectMemberSortEnum';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component(
    {
        data: function () {
            return {
                userName: this.$store.getters.userName,
            };
        },
        methods: {
            toggleSelection: function (): void {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SORT_PM_BY_DROPDOWN);
            }
        },
        computed: mapState({
            sortOption: (state: any) => {
                switch (state.projectMembersModule.searchParameters.sortBy) {
                    case ProjectMemberSortByEnum.EMAIL:
                        return 'email';

                    case ProjectMemberSortByEnum.CREATED_AT:
                        return 'date';
                    default: // ProjectMemberSortByEnum.NAME
                        return 'name';
                }
            },
            isChoiceShown: (state: any) => state.appStateModule.appState.isSortProjectMembersByPopupShown
        }),
        components: {
            SortDropdown
        }
    }
)

export default class SortUsersDropdown extends Vue {
}
</script>

<style scoped lang="scss">
    a {
        text-decoration: none;
        outline: none;
    }
    .sort-container {
        position: relative;
        padding-right: 10px;
        background-color: #FFFFFF;
        cursor: pointer;
        width: 100%;
        max-width: 260px;
        height: 56px;
        box-sizing: border-box;
        border-radius: 6px;
        transition: all .2s ease-in-out;

        &:hover {
            box-shadow: 0px 4px rgba(231, 232, 238, 0.6);
        }

        &:focus {
            box-shadow: 0px 4px rgba(231, 232, 238, 0.6);
        }
    }

    .sort-toggle-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
        width: 100%;
        height: 56px;

        &__sort-name {
            margin-left: 20px;
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 23px;
            color: #AFB7C1;
        }

        &__expander-area {
            margin-left: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            width: 28px;
            height: 28px;
        }
    }

    @media screen and (max-width: 720px) {

        .sort-toggle-container {
            &__sort-name {
                display: none;
            }
        }
    }
</style>
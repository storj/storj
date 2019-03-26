// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- To close popup we need to use method onCloseClick -->
    <div class="sort-dropdown-choice-container" id="sortTeamMemberByDropdown">
        <div class="sort-dropdown-overflow-container">
            <!-- TODO: add selection logic onclick -->
            <div class="sort-dropdown-item-container" v-on:click="onSortUsersClick(sortByEnum.EMAIL)">
                <h2>Sort by email</h2>
            </div>
            <div class="sort-dropdown-item-container" v-on:click="onSortUsersClick(sortByEnum.CREATED_AT)">
                <h2>Sort by date</h2>
            </div>
            <div class="sort-dropdown-item-container" v-on:click="onSortUsersClick(sortByEnum.NAME)">
                <h2>Sort by name</h2>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { ProjectMemberSortByEnum } from '@/utils/constants/ProjectMemberSortEnum';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';

@Component(
    {
        data: function () {
            return {
                sortByEnum: ProjectMemberSortByEnum,
            };
        },
        props: {
            onClose: {
                type: Function
            }
        },
        methods: {
            onCloseClick: function (): void {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SORT_PM_BY_DROPDOWN);
            },
            onSortUsersClick: async function (sortBy: ProjectMemberSortByEnum) {
                this.$store.dispatch(PM_ACTIONS.SET_SORT_BY, sortBy);
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SORT_PM_BY_DROPDOWN);

                const response = await this.$store.dispatch(PM_ACTIONS.FETCH);
                if (response.isSuccess) return;

                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');

            }
        },
    }
)

export default class SortDropdown extends Vue {
}
</script>

<style scoped lang="scss">
    .sort-dropdown-choice-container {
        position: absolute;
        top: 70px;
        left: 0px;
        border-radius: 4px;
        padding: 10px 0px 10px 0px;
        box-shadow: 0px 4px rgba(231, 232, 238, 0.6);
        background-color: #FFFFFF;
        z-index: 800;
    }

    .sort-dropdown-overflow-container {
        position: relative;
        width: 260px;
        height: auto;
        background-color: #FFFFFF;
    }

    .sort-dropdown-item-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        padding-left: 20px;
        padding-right: 20px;

        h2 {
            font-family: 'font_regular';
            margin-left: 20px;
            font-size: 14px;
            line-height: 20px;
            color: #354049;
        }

        &:hover {
            background-color: #F2F2F6;

            path {
                fill: #2683FF !important;
            }
        }

    }

    a {
        text-decoration: none;
        outline: none;
    }
</style>
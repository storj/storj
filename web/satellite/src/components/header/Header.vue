// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header-container">
        <div class="header-container__left-area">
            <ProjectSelectionArea class="header-container__left-area__project-selection"/>
        </div>
        <div class="header-container__right-area">
            <NewProjectArea class="header-container__right-area__new-project" />
            <AccountButton class="header-container__right-area__account-button" />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import ProjectSelectionArea from '@/components/header/projectSelection/ProjectSelectionArea.vue';
import NewProjectArea from '@/components/header/NewProjectArea.vue';
import AccountButton from './AccountButton.vue';

@Component(
    {
        components: {
            ProjectSelectionArea,
            NewProjectArea,
            AccountButton,
        },
    }
)

export default class DashboardHeader extends Vue {
}
</script>

<style scoped lang="scss">
    .header-container {
        width: 100%;
        height: 100px;
        background-color: white;
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
        padding-left: 60px;
        padding-right: 60px;

        &__left-area {
            width: 100%;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: flex-start;
        }

        &__right-area {
            @extend .header-container__left-area;
            justify-content: flex-end;

            &__new-project {
                margin-right: 24px;
            }

            &__account-button {
                margin-right: 100px;
            }
        }
    }
</style>

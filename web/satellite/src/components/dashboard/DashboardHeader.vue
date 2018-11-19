<template>
    <div class="header-container">
        <img class="header-container__logo" src="../../../static/images/dashboard/Logo.png" alt="">
        <ProjectSelectionArea class="header-container__project-selection"/>
        <AccountButton class="header-container__account-button" />
        <NewProjectArea class="header-container__new-project" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import ProjectSelectionArea from "./projectSelection/ProjectSelectionArea.vue";
import NewProjectArea from "./newProject/NewProjectArea.vue";
import AccountButton from "./account/AccountButton.vue"

@Component(
    { 
		components: {
            ProjectSelectionArea,
            NewProjectArea,
            AccountButton
        }
    }
)

export default class DashboardHeader extends Vue {}
</script>

<style scoped lang="scss">
    .header-container {
        width: 100%;
        height: 10vh;
        background-color: white;
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        padding-left: 4vw;
        padding-right: 4vw;

        img {
            cursor: pointer;
        }

        &__logo {
            width: 10vw;
            height: 7.5vh;
        }

        &__project-selection {
            margin-left: 10vw;
        }
        
        &__account-button {
            margin-left: 32vw;
        }
        
        &__new-project {
            margin-left: 20px;
        }
    }
</style>

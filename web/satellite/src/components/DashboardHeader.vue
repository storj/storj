<template>
    <div class="headerContainer">
        <img class="hLogo" src="../../static/images/dashboard/Logo.png" alt="">
        <ProjectSelectionDropdown class="hPSD"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import ProjectSelectionDropdown from "./ProjectSelectionDropdown.vue";

@Component(
    { 
		components: {
            ProjectSelectionDropdown
        }
    }
)

export default class DashboardHeader extends Vue {}
</script>

<style scoped lang="scss">
    .headerContainer {
        width: 100%;
        height: 10vh;
        background-color: white;
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        padding-left: 4vw;
        padding-right: 4vw;
    }
    .hLogo {
        width: 10vw;
        height: 7.5vh;
    }
    .hPSD {
        margin-left: 10vw;
    }
</style>

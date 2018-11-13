<template>
    <div class="dashboardContainer">
        <div class="dNavigationContainer">

        </div>
        <DashboardHeader />
        <div class="dMainAreaContainer">
            <router-view />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import DashboardHeader from "@/components/dashboard/DashboardHeader.vue";

@Component({
	components: {
		DashboardHeader
	}
})
export default class Dashboard extends Vue {}
</script>

<style scoped lang="scss">
	.dashboardContainer {
        position: fixed;
		width: 100%;
		height: 100%;
		left: 0;
		top: 0;
        background-color: #F5F6FA;
        z-index: 10;
    }
    .dNavigationContainer {
        position: fixed;
        width: 20vw;
		height: 100%;
		left: 0;
		top: 10vh;
    }
    .dMainAreaContainer {
        position: fixed;
        width: 80vw;
		height: 100%;
		left: 20vw;
		top: 10vh;
    }
</style>
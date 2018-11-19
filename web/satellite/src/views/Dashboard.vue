<template>
    <div class="dashboard-container">
        <NavigationArea />
        <DashboardHeader />
        <div class="dashboard-container__main-area">
            <router-view />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import DashboardHeader from "@/components/dashboard/DashboardHeader.vue";
import NavigationArea from "@/components/navigation/NavigationArea.vue";
import NAVIGATION_ITEMS from "@/utils/constants/navigationLinks";

@Component({
	components: {
        NavigationArea,
		DashboardHeader
	}
})
export default class Dashboard extends Vue {}
</script>

<style scoped lang="scss">
	.dashboard-container {
        position: fixed;
		width: 100%;
		height: 100%;
		left: 0;
		top: 0;
        background-color: #F5F6FA;
        z-index: 10;

        &__main-area {
            position: fixed;
            width: 80vw;
            height: 100%;
            left: 20vw;
            top: 10vh;
        }
    }
</style>
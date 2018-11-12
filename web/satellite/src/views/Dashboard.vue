<template>
    <div class="dashboardContainer">
        <!-- Navigator here-->
        <DashboardHeader />
        <!-- MainArea here-->
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import DashboardHeader from "@/components/DashboardHeader.vue";

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
</style>
// TODO: refactor component and placement
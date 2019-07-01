<template>
	<div class="page">
		<SNOHeader />
		<div class="content">
			<SNOContentTitle />
			<SNOContentFilling />
		</div>
		<SNOFooter />
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import SNOHeader from '@/components/SNOHeader.vue';
    import SNOContentTitle from '@/components/SNOContentTitle.vue';
    import SNOContentFilling from '@/components/SNOContentFilling.vue';
    import SNOFooter from '@/components/SNOFooter.vue';
	import { NODE_ACTIONS } from '@/utils/constants'

    @Component ({
        components: {
            SNOHeader,
	        SNOContentTitle,
            SNOContentFilling,
			SNOFooter,
        },
		mounted: async function () {
			let info = await this.$store.dispatch(NODE_ACTIONS.GET_NODE_INFO, '/api/dashboard/');
		}
    })

    export default class DashboardTest extends Vue {
    }
</script>

<style lang="scss">
	.page {
		background-color: #F4F6F9;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
	}

	.content {
		width: 822px;
		padding-top: 31px;
	}
</style>

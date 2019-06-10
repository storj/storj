<template>
	<div class="page">
		<SNOHeader />
		<div class="content">
			<SNOContentTitle />
			<SNOContentFilling />
		</div>
	</div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import SNOHeader from '@/components/sno/SNOHeader.vue';
    import SNOContentTitle from '@/components/sno/SNOContentTitle.vue';
    import SNOContentFilling from '@/components/sno/SNOContentFilling.vue';

    @Component ({
        components: {
            SNOHeader,
	        SNOContentTitle,
            SNOContentFilling,
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

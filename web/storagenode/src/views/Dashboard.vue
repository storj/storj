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

    @Component ({
        components: {
            SNOHeader,
	        SNOContentTitle,
            SNOContentFilling,
			SNOFooter,
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

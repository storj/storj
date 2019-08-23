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
    import SNOHeader from '@/app/components/SNOHeader.vue';
    import SNOContentTitle from '@/app/components/SNOContentTitle.vue';
    import SNOContentFilling from '@/app/components/SNOContentFilling.vue';
    import SNOFooter from '@/app/components/SNOFooter.vue';
    import { NODE_ACTIONS } from '@/app/utils/constants';

    @Component ({
        components: {
            SNOHeader,
            SNOContentTitle,
            SNOContentFilling,
            SNOFooter,
        },
    })
    export default class Dashboard extends Vue {
        public async mounted() {
            await this.$store.dispatch(NODE_ACTIONS.GET_NODE_INFO);
            await this.$store.dispatch(NODE_ACTIONS.SELECT_SATELLITE, null);
        }
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

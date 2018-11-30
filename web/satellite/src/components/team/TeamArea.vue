// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-container">
        <div class="team-container__header">
            <HeaderArea/>
        </div>
        <div class="team-container__content">
            
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import TeamMemberItem from '@/components/team/TeamMemberItem.vue';
import HeaderArea from '@/components/team/headerArea/HeaderArea.vue';

@Component({
    components: {
        TeamMemberItem,
        HeaderArea
    }
})

export default class TeamArea extends Vue {}
</script>

<style scoped lang="scss">
   .team-container {
       padding: 55px 30px 55px 64px;

       &__content {
           display: flex;
           flex-direction: row;
           align-items: center;
           justify-content: flex-start;
           flex-wrap: wrap;
           width: 100%;
       }
   }
</style>
// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="team-header-container">
        <SortUsersDropdown />
        <SearchArea />
        <Button label="Add User" width="240px" height="58px" :onPress="onAddUsersClick" id="addTeamMemberPopupButton" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import SortUsersDropdown from '@/components/team/headerArea/SortUsersDropdown.vue';
import SearchArea from './SearchArea.vue';
import Button from '@/components/common/Button.vue';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    methods: {
        onAddUsersClick: function(): void {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);
        }
    },
    components: {
        SortUsersDropdown,
        SearchArea,
        Button
    }
})

export default class HeaderArea extends Vue {
}
</script>

<style scoped lang="scss">
    .team-header-container {
        margin-bottom: 4px;
        display: flex;
        align-items: center;
        justify-content: flex-start;
    }
</style>
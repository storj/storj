// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="footer">
        <!-- if selection mode -->
        <DeleteUserArea />
        <!-- approval after delete click -->
         <!--<ApproveDeleteUserArea />-->
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import ApproveDeleteUserArea from './ApproveDeleteUserArea.vue';
import DeleteUserArea from './DeleteUserArea.vue';

@Component({
    components: {
        DeleteUserArea,
        ApproveDeleteUserArea,
    }
})

export default class TeamFooter extends Vue {
}
</script>

<style scoped lang="scss">

    .footer {
        z-index: 99;
    }

</style>
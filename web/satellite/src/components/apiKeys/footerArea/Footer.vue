// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <!-- if selection mode -->
        <DeleteApiKeysArea />
        <!-- approval after delete click -->
         <!--<ApproveDeleteApiKeysArea />-->
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import ApproveDeleteApiKeysArea from './ApproveDeleteApiKeysArea.vue';
import DeleteApiKeysArea from './DeleteApiKeysArea.vue';

@Component({
    components: {
        DeleteApiKeysArea,
        ApproveDeleteApiKeysArea,
    }
})

export default class APIFooter extends Vue {
}
</script>

<style scoped lang="scss">

</style>
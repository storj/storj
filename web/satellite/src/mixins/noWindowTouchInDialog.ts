// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineComponent } from 'vue';

export const noWindowTouchInDialog = defineComponent({
    beforeMount() {
        // We return early if current component is not VWindow.
        if (this.$options.name !== 'VWindow') return;

        // We search through components hierarchy to find VDialog as a parent.
        let parent = this.$parent;
        while (parent && parent.$options.name !== 'VDialog') parent = parent.$parent;

        // If at this point our parent is defined and is VDialog component then we set value of VWindow 'touch' prop to false.
        if (parent && 'touch' in this.$props) this.$props.touch = false;
    },
});

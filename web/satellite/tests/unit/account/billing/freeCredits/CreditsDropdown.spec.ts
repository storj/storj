// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';

import CreditsDropdown from '@/components/account/billing/freeCredits/CreditsDropdown.vue';

import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();

let clickOutsideEvent: EventListener;

localVue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
        clickOutsideEvent = function(event: Event): void {
            if (el === event.target) {
                return;
            }

            if (vnode.context) {
                vnode.context[binding.expression](event);
            }
        };

        document.body.addEventListener('click', clickOutsideEvent);
    },
    unbind: function(): void {
        document.body.removeEventListener('click', clickOutsideEvent);
    },
});

describe('CreditsDropdown', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(CreditsDropdown, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('clicks work correctly', async (): Promise<void> => {
        const clickSpy = sinon.spy();
        const wrapper = mount(CreditsDropdown, {
            localVue,
            methods: {
                redirect: clickSpy,
            },
        });

        await wrapper.find('.credits-dropdown__link-container').trigger('click');

        expect(clickSpy.callCount).toBe(1);
    });
});

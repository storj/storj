import { createLocalVue, mount } from '@vue/test-utils';
import Vuex from 'vuex';
import ApiKeysCopyPopup from '@/components/apiKeys/ApiKeysCopyPopup.vue';
import { apiKeysModule } from '@/store/modules/apiKeys';

const localVue = createLocalVue();

localVue.use(Vuex);

let state = apiKeysModule.state;
let mutations = apiKeysModule.mutations;
let actions = apiKeysModule.actions;
let getters = apiKeysModule.getters;

const store = new Vuex.Store({
    modules: {
        apiKeysModule: {
            state,
            mutations,
            actions,
            getters
        }
    }
});

describe('ApiKeysCopyPopup', () => {
    it('renders correctly', () => {
        const wrapper = mount(ApiKeysCopyPopup, {
            store,
            localVue
        });

        expect(wrapper).toMatchSnapshot();
    });


    it('function onCloseClick works correctly', () => {
        const wrapper = mount(ApiKeysCopyPopup, {
            store,
            localVue,
        });

        wrapper.vm.onCloseClick();

        expect(wrapper.vm.$data.isCopiedButtonShown).toBe(false);
        expect(wrapper.emitted()).toEqual({'closePopup': [[]]});
    });

    it('function onCopyClick works correctly', () => {
        const wrapper = mount(ApiKeysCopyPopup, {
            store,
            localVue,
        });

        wrapper.vm.onCopyClick();

        expect(wrapper.vm.$data.isCopiedButtonShown).toBe(true);
    });
});

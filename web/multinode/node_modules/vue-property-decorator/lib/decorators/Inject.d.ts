import { InjectKey } from 'vue/types/options';
export declare type InjectOptions = {
    from?: InjectKey;
    default?: any;
};
/**
 * decorator of an inject
 * @param from key
 * @return PropertyDecorator
 */
export declare function Inject(options?: InjectOptions | InjectKey): import("vue-class-component").VueDecorator;

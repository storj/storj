import { InjectKey } from 'vue/types/options';
import { InjectOptions } from './Inject';
/**
 * decorator of a reactive inject
 * @param from key
 * @return PropertyDecorator
 */
export declare function InjectReactive(options?: InjectOptions | InjectKey): import("vue-class-component").VueDecorator;

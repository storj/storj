/// <reference types="node" />
import webpack from 'webpack';
import { EventEmitter } from 'events';
interface Watcher extends EventEmitter {
    mtimes: Record<string, number>;
}
declare function getWatcher(compiler: webpack.Compiler): Watcher | undefined;
export { getWatcher, Watcher };

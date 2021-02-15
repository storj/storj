import * as parse5 from 'parse5';
/**
 * Type alias to work with Parse5's funky types.
 */
export declare type Node = (parse5.AST.Default.Element & parse5.AST.Default.Node) | undefined;
/**
 * Options for you.
 */
export interface Options {
    lang?: string | [string];
    emptyExport?: boolean;
}
/**
 * Parse a vue file's contents.
 */
export declare function parse(input: string, tag: string, options?: Options): string;
/**
 * Get an array of all the nodes (tags).
 */
export declare function getNodes(input: string): parse5.AST.Default.Element[];
/**
 * Get the node.
 */
export declare function getNode(input: string, tag: string, options?: Options): Node;

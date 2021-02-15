/**
 * @license
 * Copyright 2018 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

function isAsync (chunk) {
  if ('canBeInitial' in chunk) {
    return !chunk.canBeInitial()
  } else {
    return !chunk.isInitial()
  }
}

function getChunkEntryNames (chunk) {
  if ('groupsIterable' in chunk) {
    return Array.from(new Set(getNames(chunk.groupsIterable)))
  } else {
    return chunk.entrypoints.map(e => e.options.name)
  }
}

function getNames (groups, processed = new Set()) {
  const Entrypoint = require('webpack/lib/Entrypoint')
  const names = []
  for (const group of groups) {
    if (group instanceof Entrypoint) {
      // entrypoint
      if (group.options.name) {
        names.push(group.options.name)
      }
    } else if (!processed.has(group)) {
      processed.add(group)
      names.push(...getNames(group.parentsIterable, processed))
    }
  }
  return names
}

function extractChunks ({ compilation, optionsInclude }) {
  let includeChunks
  let includeType
  let includeEntryPoints
  if (optionsInclude && typeof optionsInclude === 'object') {
    includeType = optionsInclude.type
    includeChunks = optionsInclude.chunks
    includeEntryPoints = optionsInclude.entries
  } else {
    if (Array.isArray(optionsInclude)) {
      includeChunks = optionsInclude
    } else {
      includeType = optionsInclude
    }
  }

  let chunks = compilation.chunks

  if (Array.isArray(includeChunks)) {
    chunks = chunks.filter((chunk) => {
      return chunk.name && includeChunks.includes(chunk.name)
    })
  }

  if (Array.isArray(includeEntryPoints)) {
    chunks = chunks.filter(chunk => {
      const names = getChunkEntryNames(chunk)
      return names.some(name => includeEntryPoints.includes(name))
    })
  }

  // 'asyncChunks' are chunks intended for lazy/async loading usually generated as
  // part of code-splitting with import() or require.ensure(). By default, asyncChunks
  // get wired up using link rel=preload when using this plugin. This behaviour can be
  // configured to preload all types of chunks or just prefetch chunks as needed.
  if (includeType === undefined || includeType === 'asyncChunks') {
    return chunks.filter(isAsync)
  }

  if (includeType === 'initial') {
    return chunks.filter(chunk => !isAsync(chunk))
  }

  if (includeType === 'allChunks') {
    // Async chunks, vendor chunks, normal chunks.
    return chunks
  }

  if (includeType === 'allAssets') {
    // Every asset, regardless of which chunk it's in.
    // Wrap it in a single, "psuedo-chunk" return value.
    return [{ files: Object.keys(compilation.assets) }]
  }

  return chunks
}

module.exports = extractChunks

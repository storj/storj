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

module.exports = function doesChunkBelongToHtml ({ chunk, htmlAssetsChunks, compilation }) {
  // Get all the hashes of the HTML assets.
  const rootHashes = Object.values(htmlAssetsChunks).map(({ hash }) => hash)
  // Get a list of chunk groups that contain one of those hashes.
  const rootChunkGroups = compilation.chunkGroups.filter((chunkGroup) => {
    return chunkGroup.chunks.filter((chunk) => rootHashes.includes(chunk.renderedHash))
  })
  // Get an id for each of those chunk groups.
  const rootChunkGroupsIds = new Set(rootChunkGroups.map(({ id }) => id))
  // Return true iff the chunk we're passed belongs to a group whose id is in
  // the list of root chunk groups.
  return Array.from(chunk.groupsIterable).some(({ id }) => rootChunkGroupsIds.has(id))
}

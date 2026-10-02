// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

const assert = require('node:assert/strict');
const test = require('node:test');
const schema = require('../../schemas/k0s.json');

// Editors use JavaScript regular expressions; the Go schema tests use RE2.
const versionPattern = new RegExp(schema.properties.spec.properties.images
  .properties.coredns.properties.version.pattern);

for (const [version, valid] of [
  ['v1.0', true],
  ['v1@sha256:0123456789abcdefABCDEF0123456789abcdefABCDEF0123456789abcdefABCDEF01', true],
  ['bad tag', false],
  ['v1@sha256:zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz', false],
  ['v1@sha256:abcd', false],
]) {
  test(`image version ${version}`, () => {
    assert.equal(versionPattern.test(version), valid);
  });
}

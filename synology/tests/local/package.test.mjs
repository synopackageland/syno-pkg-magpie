import assert from 'node:assert/strict';
import { existsSync } from 'node:fs';
import { test } from 'node:test';
import { cli } from '../support/cli.mjs';

// Explicit local build tier: check, build and inspect one architecture. No
// deployment, no NAS connection and no browser.
test('the App checks, builds and inspects as the documented wrapper', {timeout: 600_000}, () => {
  const checked = cli('check');
  assert.equal(checked.package, 'Magpie');

  const built = cli('build', '--arch', process.env.APP_TEST_ARCH || 'x86_64');
  assert.ok(built.outputs.length > 0);
  for (const spk of built.outputs) {
    assert.equal(existsSync(spk), true);
    const inspected = cli('inspect', spk);

    // The wrapper's contract: no SDK window, DSM administrators only, and the
    // gateway entry Package Center opens.
    assert.equal(inspected.spk, spk);
    assert.equal(inspected.package, 'Magpie');
    assert.equal(inspected.entry, 'admin-only');
    assert.equal(inspected.launch, 'new-page');
    assert.equal(inspected.signed, false);

    const carried = new Set(inspected.payload.map((file) => file.path));
    for (const path of [
      'daemon/payload/magpie',
      'daemon/payload/webtoken',
      'daemon/payload/magpie-launch',
      'daemon/payload/_sso/gateway',
      'sso/gateway.json',
      'sso/native-nginx.conf',
      'ui/sso/open.cgi',
      'ui/cgi/api.cgi',
    ]) {
      assert.ok(carried.has(path), `${spk} does not carry ${path}`);
    }

    // inspect findings are the build-contract errors; warnings are DSM's own
    // INFO notes, which the CLI generates for the nginx gateway entry.
    assert.deepEqual(inspected.findings, []);
    assert.deepEqual(inspected.dsm.errors, []);
  }
});

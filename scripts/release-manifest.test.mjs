import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { releaseMetadata, validateVersion, writeReleaseManifest } from './release-manifest.mjs';

test('publishing requires a matching VERSION tag; branch dispatch only builds', () => {
  assert.deepEqual(releaseMetadata('0.2.0', { refType: 'tag', refName: 'v0.2.0', eventName: 'push' }), {
    version: '0.2.0', tag: 'v0.2.0', prerelease: false, publish: true,
  });
  assert.equal(releaseMetadata('0.2.0', { refType: 'branch', refName: 'main', eventName: 'workflow_dispatch' }).publish, false);
  assert.throws(() => releaseMetadata('0.2.0', { refType: 'tag', refName: 'v0.3.0' }), /must match VERSION/);
  assert.throws(() => releaseMetadata('0.2.0', { refType: 'branch', refName: 'main', publish: 'true' }), /matching existing version tag/);
  assert.equal(releaseMetadata('0.2.0-rc.1+build.2', { refType: 'tag', refName: 'v0.2.0-rc.1+build.2', publish: 'true' }).prerelease, true);
});

test('versions reject malformed and ambiguous releases', () => {
  for (const value of ['v0.2.0', '0.2', '01.2.0', '0.2.0-01', '0.2.0-', '0.2.0+']) {
    assert.throws(() => validateVersion(value), /Invalid semantic version/);
  }
  assert.equal(validateVersion(' 0.2.0-alpha.1+build.3\n'), '0.2.0-alpha.1+build.3');
});

test('combined manifest binds both artifacts to their tag and hashes their actual bytes', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'modelswitch-release-'));
  try {
    const version = '0.2.0';
    const windows = `ModelSwitcher-${version}-windows-amd64.exe`;
    const macos = `ModelSwitcher-${version}-macos-universal.app.zip`;
    await writeFile(join(directory, windows), 'windows binary');
    await writeFile(join(directory, macos), 'mac bundle');
    const baseURL = 'https://github.com/owner/repo/releases/download/v0.2.0';
    const manifest = await writeReleaseManifest({ directory, version, baseURL, requireAll: true });
    const digest = (data) => createHash('sha256').update(data).digest('hex');
    assert.deepEqual(manifest.windows, { url: `${baseURL}/${windows}`, sha256: digest('windows binary') });
    assert.deepEqual(manifest.macos, { url: `${baseURL}/${macos}`, sha256: digest('mac bundle') });
    assert.ok(Number.isFinite(Date.parse(manifest.published_at)));
    const json = await readFile(join(directory, 'latest.json'));
    assert.deepEqual(JSON.parse(json.toString('utf8')), manifest);
    const checksums = await readFile(join(directory, 'SHA256SUMS.txt'), 'utf8');
    assert.equal(checksums, `${digest('windows binary')}  ${windows}\n${digest('mac bundle')}  ${macos}\n${digest(json)}  latest.json\n`);
  } finally { await rm(directory, { recursive: true, force: true }); }
});

test('a missing platform prevents publishing without overwriting a prior manifest', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'modelswitch-release-'));
  try {
    await writeFile(join(directory, 'ModelSwitcher-0.2.0-windows-amd64.exe'), 'windows');
    await writeFile(join(directory, 'latest.json'), 'prior manifest');
    await assert.rejects(writeReleaseManifest({ directory, version: '0.2.0', baseURL: 'https://github.com/owner/repo/releases/download/v0.2.0', requireAll: true }), /Missing release artifact/);
    assert.equal(await readFile(join(directory, 'latest.json'), 'utf8'), 'prior manifest');
    const local = await writeReleaseManifest({ directory, version: '0.2.0' });
    assert.equal(local.windows.url, '');
    assert.deepEqual(local.macos, { url: '', sha256: '' });
  } finally { await rm(directory, { recursive: true, force: true }); }
});

test('publishing rejects an empty artifact and unsafe download sources', async () => {
  const directory = await mkdtemp(join(tmpdir(), 'modelswitch-release-'));
  try {
    await writeFile(join(directory, 'ModelSwitcher-0.2.0-windows-amd64.exe'), '');
    await assert.rejects(writeReleaseManifest({ directory, version: '0.2.0' }), /Empty release artifact/);
    for (const baseURL of ['http://example.com', 'https://secret@example.com', 'https://example.com?token=secret']) {
      await assert.rejects(writeReleaseManifest({ directory, version: '0.2.0', baseURL }), /must be HTTPS/);
    }
  } finally { await rm(directory, { recursive: true, force: true }); }
});

import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { appendFile, mkdir, readFile, stat, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const versionPattern = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;

export function validateVersion(value) {
  const version = value.trim();
  if (!versionPattern.test(version)) throw new Error(`Invalid semantic version: ${version}`);
  return version;
}

export function releaseMetadata(version, { refType, refName, eventName, publish } = {}) {
  version = validateVersion(version);
  const tag = `v${version}`;
  if (refType === 'tag' && refName !== tag) {
    throw new Error(`Tag ${refName} must match VERSION (${tag})`);
  }
  const shouldPublish = eventName === 'push' || publish === 'true';
  if (shouldPublish && (refType !== 'tag' || refName !== tag)) {
    throw new Error('Publishing requires running on the matching existing version tag');
  }
  return { version, tag, prerelease: version.split('+')[0].includes('-'), publish: shouldPublish };
}

async function sha256(path) {
  const hash = createHash('sha256');
  for await (const chunk of createReadStream(path)) hash.update(chunk);
  return hash.digest('hex');
}

// The base URL is the directory containing versioned artifacts, including the tag.
// Local builds may omit it; only the final publishing job requires both platforms.
export async function writeReleaseManifest({ directory, version, baseURL = '', requireAll = false }) {
  version = validateVersion(version);
  if (baseURL) {
    const url = new URL(baseURL);
    if (url.protocol !== 'https:' || url.username || url.password || url.search || url.hash) {
      throw new Error('Release download base URL must be HTTPS without credentials or query parameters');
    }
  }
  await mkdir(directory, { recursive: true });
  const manifest = { version, published_at: new Date().toISOString() };
  const checksums = [];
  for (const [platform, suffix] of [['windows', 'windows-amd64.exe'], ['macos', 'macos-universal.app.zip']]) {
    const name = `ModelSwitcher-${version}-${suffix}`;
    const path = resolve(directory, name);
    let exists = false;
    try {
      exists = (await stat(path)).isFile();
    } catch (error) {
      if (error.code !== 'ENOENT') throw error;
    }
    if (!exists) {
      if (requireAll) throw new Error(`Missing release artifact: ${name}`);
      manifest[platform] = { url: '', sha256: '' };
      continue;
    }
    if ((await stat(path)).size === 0) throw new Error(`Empty release artifact: ${name}`);
    const digest = await sha256(path);
    manifest[platform] = { url: baseURL ? `${baseURL.replace(/\/$/, '')}/${encodeURIComponent(name)}` : '', sha256: digest };
    checksums.push(`${digest}  ${name}`);
  }
  if (!checksums.length) throw new Error('No release artifacts found');
  if (requireAll && !baseURL) throw new Error('Publishing requires a release download base URL');
  const json = `${JSON.stringify(manifest, null, 2)}\n`;
  const manifestPath = resolve(directory, 'latest.json');
  await writeFile(manifestPath, json, 'utf8');
  checksums.push(`${createHash('sha256').update(json).digest('hex')}  latest.json`);
  await writeFile(resolve(directory, 'SHA256SUMS.txt'), `${checksums.join('\n')}\n`, 'utf8');
  return manifest;
}

async function main() {
  const [command = 'manifest', ...args] = process.argv.slice(2);
  const version = validateVersion(args[0] || await readFile(new URL('../VERSION', import.meta.url), 'utf8'));
  if (command === 'prepare') {
    const metadata = releaseMetadata(version, {
      refType: process.env.GITHUB_REF_TYPE, refName: process.env.GITHUB_REF_NAME,
      eventName: process.env.GITHUB_EVENT_NAME, publish: process.env.RELEASE_PUBLISH,
    });
    const output = Object.entries(metadata).map(([key, value]) => `${key}=${value}`).join('\n') + '\n';
    if (process.env.GITHUB_OUTPUT) await appendFile(process.env.GITHUB_OUTPUT, output);
    process.stdout.write(output);
  } else if (command === 'manifest') {
    await writeReleaseManifest({
      version, directory: args[1] || resolve('release', version),
      baseURL: process.env.MODELSWITCHER_RELEASE_DOWNLOAD_BASE_URL || '',
      requireAll: args[2] === '--require-all',
    });
  } else {
    throw new Error(`Unknown release command: ${command}`);
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  main().catch((error) => { console.error(error.message); process.exitCode = 1; });
}

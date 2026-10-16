#!/usr/bin/env node
// Fills in the npm packages in npm/ for a release tag, from the release
// assets in <assets-dir>, and writes the seven package folders into <out>:
//
//   cli/                 @ahasend/cli, the wrapper: npm/cli/ with the version,
//                        and the six platform packages as optional
//                        dependencies of exactly that version
//   cli-<os>-<cpu>/      @ahasend/cli-<os>-<cpu>: npm/platform/package.json
//                        filled in, and the asset ahasend-<goos>-<goarch> as
//                        bin/ahasend (bin/ahasend.exe on Windows)
//
// Every package also gets the assets LICENSE and THIRD_PARTY_NOTICES. The
// binaries and the launcher get mode 755, which `npm pack` keeps when it runs
// on macOS or Linux. Usage: scripts/npm-packages.mjs <tag> <assets-dir> <out>
import { chmodSync, copyFileSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const platforms = [
  { os: 'darwin', cpu: 'arm64', asset: 'ahasend-darwin-arm64' },
  { os: 'darwin', cpu: 'x64', asset: 'ahasend-darwin-amd64' },
  { os: 'linux', cpu: 'arm64', asset: 'ahasend-linux-arm64' },
  { os: 'linux', cpu: 'x64', asset: 'ahasend-linux-amd64' },
  { os: 'win32', cpu: 'arm64', asset: 'ahasend-windows-arm64.exe' },
  { os: 'win32', cpu: 'x64', asset: 'ahasend-windows-amd64.exe' },
];
const licenceFiles = ['LICENSE', 'THIRD_PARTY_NOTICES'];

if (process.argv.length !== 5) {
  console.error('usage: scripts/npm-packages.mjs <tag> <assets-dir> <out>');
  process.exit(1);
}
const [tag, assets, out] = process.argv.slice(2);
if (!/^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$/.test(tag)) {
  console.error(`npm-packages: ${tag} is not a release tag`);
  process.exit(1);
}
const version = tag.slice(1);
const templates = fileURLToPath(new URL('../npm/', import.meta.url));

function readJSON(path) {
  return JSON.parse(readFileSync(path, 'utf8'));
}

// writePackage writes the package.json and the licence files of a package
function writePackage(dir, json) {
  mkdirSync(join(dir, 'bin'), { recursive: true });
  writeFileSync(join(dir, 'package.json'), `${JSON.stringify(json, null, 2)}\n`);
  for (const file of licenceFiles) {
    copyFileSync(join(assets, file), join(dir, file));
  }
}

const wrapper = readJSON(join(templates, 'cli', 'package.json'));
wrapper.version = version;
wrapper.optionalDependencies = {};

for (const { os, cpu, asset } of platforms) {
  const json = readJSON(join(templates, 'platform', 'package.json'));
  const binary = os === 'win32' ? 'bin/ahasend.exe' : 'bin/ahasend';
  json.name = `@ahasend/cli-${os}-${cpu}`;
  json.version = version;
  json.description = `The ahasend binary for ${os} ${cpu}; install @ahasend/cli instead`;
  json.os = [os];
  json.cpu = [cpu];
  json.files = [binary, ...licenceFiles];

  const dir = join(out, `cli-${os}-${cpu}`);
  writePackage(dir, json);
  copyFileSync(join(assets, asset), join(dir, binary));
  if (os !== 'win32') {
    chmodSync(join(dir, binary), 0o755);
  }
  wrapper.optionalDependencies[json.name] = version;
}

const dir = join(out, 'cli');
writePackage(dir, wrapper);
copyFileSync(join(templates, 'cli', 'bin', 'ahasend.js'), join(dir, 'bin', 'ahasend.js'));
chmodSync(join(dir, 'bin', 'ahasend.js'), 0o755);
console.log(`Wrote the seven npm packages of ${version} into ${out}`);

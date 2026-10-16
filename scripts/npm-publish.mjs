#!/usr/bin/env node
// Publishes the npm packages that scripts/npm-packages.mjs wrote into <dir>
// to the registry npm is set up for, each with `npm publish --access public`.
// It skips a package that already has its version, so a run can be repeated,
// and publishes the platform packages first and @ahasend/cli last, because
// the wrapper needs them. A version older than a package's latest dist-tag
// is published with --tag legacy, so that dist-tag does not move back.
// Provenance comes from trusted publishing. It runs on macOS and Linux.
// Usage: scripts/npm-publish.mjs <dir>
import { spawnSync } from 'node:child_process';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const wrapper = '@ahasend/cli';

if (process.argv.length !== 3) {
  console.error('usage: scripts/npm-publish.mjs <dir>');
  process.exit(1);
}
const dir = process.argv[2];

function fail(message) {
  console.error(`npm-publish: ${message}`);
  process.exit(1);
}

// view prints the field of `npm view <spec> <field>`, or null when the
// registry has no such package or version (E404); any other error fails
function view(spec, field) {
  const result = spawnSync('npm', ['view', spec, field, '--json', '--prefer-online'], {
    encoding: 'utf8',
  });
  if (result.error) {
    fail(`cannot run npm: ${result.error.message}`);
  }
  let output = null;
  try {
    output = JSON.parse(result.stdout);
  } catch {
    // Not JSON: handled below
  }
  if (result.status === 0) {
    return output;
  }
  if (output && output.error && output.error.code === 'E404') {
    return null;
  }
  process.stderr.write(result.stderr);
  fail(`npm view ${spec} ${field} failed`);
}

// parse returns the numbers of a version X.Y.Z
function parse(version) {
  const match = /^([0-9]+)\.([0-9]+)\.([0-9]+)$/.exec(version);
  if (!match) {
    fail(`${version} is not a stable version X.Y.Z`);
  }
  return match.slice(1).map(Number);
}

// older reports whether version a is lower than version b
function older(a, b) {
  const [x, y] = [parse(a), parse(b)];
  for (let i = 0; i < 3; i++) {
    if (x[i] !== y[i]) {
      return x[i] < y[i];
    }
  }
  return false;
}

const packages = readdirSync(dir, { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => {
    const path = join(dir, entry.name);
    const { name, version } = JSON.parse(readFileSync(join(path, 'package.json'), 'utf8'));
    return { path, name, version };
  })
  .sort((a, b) => (a.name === wrapper) - (b.name === wrapper) || a.name.localeCompare(b.name));
if (packages.length !== 7 || packages[6].name !== wrapper) {
  fail(`${dir} must hold the seven packages that scripts/npm-packages.mjs writes`);
}

for (const { path, name, version } of packages) {
  if (view(`${name}@${version}`, 'version') !== null) {
    console.log(`${name}@${version} is already published`);
    continue;
  }
  const args = ['publish', path, '--access', 'public'];
  const latest = view(name, 'dist-tags.latest');
  if (latest !== null && older(version, latest)) {
    console.log(`${name}@${version} is older than the latest dist-tag, ${latest}`);
    args.push('--tag', 'legacy');
  }
  console.log(`Publishing ${name}@${version}`);
  const result = spawnSync('npm', args, { stdio: 'inherit' });
  if (result.error || result.status !== 0) {
    fail(`npm publish of ${name}@${version} failed`);
  }
}

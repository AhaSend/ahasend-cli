// Tests the npm packages as users install them, from the seven tarballs that
// test/install/npm-pack.sh packs:
//
//   node test/install/npm.mjs <tarballs> <verdaccio>
//
// It publishes the tarballs to a local Verdaccio registry with no uplinks
// (test/install/verdaccio.yaml); <verdaccio> is Verdaccio's bin/verdaccio
// script, which runs with node. Then it installs @ahasend/cli from it with
// npm, npx, pnpm and bun and checks `ahasend --version`, the exit code of an
// unknown command, AHASEND_BINARY_PATH, the signal handling of the launcher
// (on macOS and Linux) and the message when the platform package is left
// out. Every command runs with a new HOME (and USERPROFILE and LOCALAPPDATA),
// npm config, cache and global prefix, so nothing reads or writes the user's
// own. It needs npm, pnpm and bun on PATH and runs on Node 18 and newer.
import { spawn, spawnSync } from 'node:child_process';
import { chmodSync, copyFileSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { delimiter, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

if (process.argv.length !== 4) {
  console.error('usage: node test/install/npm.mjs <tarballs> <verdaccio>');
  process.exit(1);
}
const tarballs = resolve(process.argv[2]);
const verdaccio = resolve(process.argv[3]);
const version = 'v99.0.0';
const windows = process.platform === 'win32';
const here = fileURLToPath(new URL('.', import.meta.url));
const port = 4873;
const registry = `http://127.0.0.1:${port}/`;

const tmp = mkdtempSync(join(tmpdir(), 'ahasend-npm-'));
const home = join(tmp, 'home');
mkdirSync(join(home, 'AppData', 'Local'), { recursive: true });
mkdirSync(join(home, 'AppData', 'Roaming'), { recursive: true });

// The environment of every command: the user's, without their npm, pnpm,
// bun and AhaSend settings and any token, with this test's home, npm config
// and registry
const env = {};
for (const [key, value] of Object.entries(process.env)) {
  if (!/^(npm_|pnpm_|bun_|ahasend_)|token/i.test(key)) {
    env[key] = value;
  }
}
Object.assign(env, {
  HOME: home,
  USERPROFILE: home,
  LOCALAPPDATA: join(home, 'AppData', 'Local'),
  APPDATA: join(home, 'AppData', 'Roaming'),
  XDG_CONFIG_HOME: join(home, '.config'),
  XDG_DATA_HOME: join(home, '.local', 'share'),
  XDG_CACHE_HOME: join(home, '.cache'),
  npm_config_userconfig: join(home, '.npmrc'),
  npm_config_cache: join(tmp, 'npm-cache'),
  npm_config_registry: registry,
  npm_config_update_notifier: 'false',
  npm_config_fund: 'false',
  npm_config_audit: 'false',
});
// Windows names it Path; PATH would be a second variable
const pathKey = Object.keys(env).find((key) => key.toUpperCase() === 'PATH') || 'PATH';

let failures = 0;
function check(name, ok, details) {
  if (ok) {
    console.log(`ok: ${name}`);
    return;
  }
  failures++;
  console.log(`FAIL: ${name}`);
  if (details) {
    console.log(details);
  }
}

// run runs a command with the test environment and returns its result. On
// Windows npm, npx, pnpm and the installed commands are .cmd files, which
// only a shell runs.
function run(command, args, extraEnv = {}, options = {}) {
  const quote = (arg) => (windows && /[\s"]/.test(arg) ? `"${arg}"` : arg);
  const result = spawnSync(windows ? quote(command) : command, windows ? args.map(quote) : args, {
    env: { ...env, ...extraEnv },
    encoding: 'utf8',
    shell: windows,
    ...options,
  });
  if (result.error) {
    throw result.error;
  }
  return result;
}

// must runs a command that must succeed
function must(command, args, extraEnv, options) {
  const result = run(command, args, extraEnv, options);
  if (result.status !== 0) {
    throw new Error(`${command} ${args.join(' ')} failed:\n${result.stdout}${result.stderr}`);
  }
  return result;
}

function output(result) {
  return `exit ${result.status}, signal ${result.signal}\n${result.stdout}${result.stderr}`;
}

// checkCommand checks an installed ahasend: --version, the exit code of an
// unknown command and AHASEND_BINARY_PATH
function checkCommand(how, ahasend) {
  const versionRun = run(ahasend, ['--version']);
  check(
    `${how}: ahasend --version prints ${version}`,
    versionRun.status === 0 && versionRun.stdout.split(/\r?\n/)[0] === `AhaSend CLI ${version}`,
    output(versionRun),
  );
  const unknown = run(ahasend, ['no-such-command']);
  check(`${how}: an unknown command exits 4, as the binary does`, unknown.status === 4, output(unknown));
  // node --version stands in for the binary
  const replaced = run(ahasend, ['--version'], { AHASEND_BINARY_PATH: process.execPath });
  check(
    `${how}: AHASEND_BINARY_PATH replaces the binary`,
    replaced.status === 0 && replaced.stdout.trim() === process.version,
    output(replaced),
  );
}

// bin returns the path of an installed command in the folder dir
function bin(dir, name) {
  return join(dir, windows ? `${name}.cmd` : name);
}

const sleep = (ms) => new Promise((done) => setTimeout(done, ms));

async function startVerdaccio() {
  const config = join(tmp, 'verdaccio.yaml');
  copyFileSync(join(here, 'verdaccio.yaml'), config);
  const server = spawn(process.execPath, [verdaccio, '--config', config, '--listen', `127.0.0.1:${port}`], {
    env,
    stdio: 'ignore',
  });
  for (let tries = 0; tries < 100; tries++) {
    try {
      const response = await fetch(`${registry}-/ping`);
      if (response.ok) {
        return server;
      }
    } catch {
      // Not listening yet
    }
    await sleep(200);
  }
  server.kill();
  throw new Error('Verdaccio did not start');
}

// publish adds a user who may publish to the registry and publishes the
// seven tarballs, the wrapper last
async function publish() {
  const response = await fetch(`${registry}-/user/org.couchdb.user:test`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: 'test', password: 'test-password' }),
  });
  const { token } = await response.json();
  writeFileSync(env.npm_config_userconfig, `//127.0.0.1:${port}/:_authToken=${token}\n`);
  const files = readdirSync(tarballs)
    .filter((file) => file.endsWith('.tgz'))
    .sort((a, b) => a.startsWith('ahasend-cli-9') - b.startsWith('ahasend-cli-9') || a.localeCompare(b));
  if (files.length !== 7) {
    throw new Error(`${tarballs} must hold the seven tarballs, not ${files.join(', ')}`);
  }
  for (const file of files) {
    must('npm', ['publish', join(tarballs, file), '--access', 'public']);
  }
  console.log('Published the seven packages to Verdaccio');
}

// signalCase starts the launcher in its own process group with the recorder
// as its binary, sends signal to the group or to the launcher alone, and
// returns how the launcher ended and the signals the recorder got
async function signalCase(ahasend, recorder, end, signal, to) {
  const log = join(tmp, `signals-${end || 'none'}-${signal}-${to}.log`);
  writeFileSync(log, '');
  const launcher = spawn(ahasend, [], {
    env: { ...env, AHASEND_BINARY_PATH: recorder, RECORDER_LOG: log, RECORDER_END: end || '' },
    detached: true,
    stdio: 'ignore',
  });
  const ended = new Promise((done) => launcher.on('exit', (code, endSignal) => done({ code, signal: endSignal })));
  for (let tries = 0; !readFileSync(log, 'utf8').includes('ready'); tries++) {
    if (tries > 100) {
      throw new Error('the recorder did not start');
    }
    await sleep(100);
  }
  process.kill(to === 'group' ? -launcher.pid : launcher.pid, signal);
  const result = await Promise.race([ended, sleep(15000).then(() => null)]);
  if (result === null) {
    process.kill(-launcher.pid, 'SIGKILL');
    throw new Error(`the launcher did not end after ${signal}`);
  }
  // The recorder may end just after the launcher
  await sleep(200);
  const signals = readFileSync(log, 'utf8').split('\n').filter((line) => line.startsWith('SIG'));
  return { ...result, signals };
}

async function checkSignals(ahasend) {
  const recorder = join(tmp, 'recorder.cjs');
  copyFileSync(join(here, 'npm-recorder.cjs'), recorder);
  // The recorder runs as the binary, so it must be executable
  chmodSync(recorder, 0o755);

  let result = await signalCase(ahasend, recorder, 'exit', 'SIGINT', 'group');
  check(
    'SIGINT to the process group reaches a binary that handles it once, and the launcher exits with its code',
    result.signals.join(' ') === 'SIGINT' && result.code === 7,
    JSON.stringify(result),
  );
  result = await signalCase(ahasend, recorder, null, 'SIGINT', 'group');
  check(
    'SIGINT to the process group ends a binary that does not handle it, and the launcher ends by SIGINT',
    result.signal === 'SIGINT',
    JSON.stringify(result),
  );
  for (const signal of ['SIGTERM', 'SIGHUP']) {
    result = await signalCase(ahasend, recorder, 'raise', signal, 'launcher');
    check(
      `${signal} to the launcher alone reaches the binary once, and the launcher ends by it too`,
      result.signals.join(' ') === signal && result.signal === signal,
      JSON.stringify(result),
    );
  }
  result = await signalCase(ahasend, recorder, 'exit', 'SIGTERM', 'group');
  check(
    'SIGTERM to the process group reaches the binary twice, the limit the docs state',
    result.signals.join(' ') === 'SIGTERM SIGTERM' && result.code === 7,
    JSON.stringify(result),
  );
  // Node ignores SIGPIPE, so the launcher cannot end by it, and exits 141
  // instead, as a shell reports it
  const piped = join(tmp, 'sigpipe.sh');
  writeFileSync(piped, '#!/bin/sh\nkill -PIPE $$\n');
  chmodSync(piped, 0o755);
  const pipeRun = run(ahasend, [], { AHASEND_BINARY_PATH: piped });
  check(
    'a binary that SIGPIPE ends makes the launcher fail with 141, not exit 0',
    pipeRun.status === 141,
    output(pipeRun),
  );
}

const server = await startVerdaccio();
try {
  await publish();

  console.log('Installing with npm install -g');
  const npmPrefix = join(tmp, 'npm-global');
  must('npm', ['install', '-g', '@ahasend/cli'], { npm_config_prefix: npmPrefix });
  const npmBin = windows ? npmPrefix : join(npmPrefix, 'bin');
  checkCommand('npm', bin(npmBin, 'ahasend'));
  if (!windows) {
    await checkSignals(join(npmBin, 'ahasend'));
  }

  // A global install installs optional dependencies whatever --omit says,
  // so this one is local
  console.log('Installing with npm install --omit=optional');
  const project = join(tmp, 'project');
  mkdirSync(project);
  must('npm', ['install', '@ahasend/cli', '--omit=optional'], {}, { cwd: project });
  const missing = run(bin(join(project, 'node_modules', '.bin'), 'ahasend'), ['--version']);
  check(
    'without optional dependencies, ahasend explains that the platform package is missing',
    missing.status === 1 &&
      missing.stderr.includes('@ahasend/cli-') &&
      missing.stderr.includes('is not installed') &&
      missing.stderr.includes('optional dependencies') &&
      missing.stderr.includes('https://ahasend.com/docs/cli/installation'),
    output(missing),
  );

  console.log('Running with npx');
  const npxDir = join(tmp, 'npx');
  mkdirSync(npxDir);
  const npx = run('npx', ['--yes', '@ahasend/cli', '--version'], {}, { cwd: npxDir });
  check(
    'npx @ahasend/cli --version works',
    npx.status === 0 && npx.stdout.split(/\r?\n/)[0] === `AhaSend CLI ${version}`,
    output(npx),
  );

  // pnpm 10 puts global commands in PNPM_HOME and pnpm 11 in PNPM_HOME/bin,
  // and either refuses to install unless that folder is on PATH
  console.log('Installing with pnpm add -g');
  const pnpmHome = join(tmp, 'pnpm-home');
  const pnpmPath = [join(pnpmHome, 'bin'), pnpmHome, env[pathKey]].join(delimiter);
  const pnpmEnv = { PNPM_HOME: pnpmHome, [pathKey]: pnpmPath };
  must('pnpm', ['add', '-g', '@ahasend/cli', '--registry', registry], pnpmEnv);
  const pnpmBin = must('pnpm', ['bin', '-g'], pnpmEnv).stdout.trim();
  checkCommand('pnpm', bin(pnpmBin, 'ahasend'));

  console.log('Installing with bun add -g');
  const bunInstall = join(tmp, 'bun');
  must('bun', ['add', '-g', '@ahasend/cli', '--registry', registry], { BUN_INSTALL: bunInstall });
  checkCommand('bun', join(bunInstall, 'bin', windows ? 'ahasend.exe' : 'ahasend'));
} finally {
  // Windows cannot remove the folder while Verdaccio still has it open
  if (server.exitCode === null && server.signalCode === null) {
    const exited = new Promise((done) => server.once('exit', done));
    server.kill();
    await exited;
  }
  rmSync(tmp, { recursive: true, force: true, maxRetries: 5 });
}

if (failures) {
  console.log(`${failures} check(s) failed`);
  process.exit(1);
}
console.log('The npm packages work');

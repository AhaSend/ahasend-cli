#!/usr/bin/env node
// Runs the ahasend binary. npm installs it in the platform package of this
// machine, one of the optional dependencies of @ahasend/cli, and leaves the
// others out. AHASEND_BINARY_PATH names another binary to run instead.
'use strict';

const { spawn } = require('node:child_process');
const { constants } = require('node:os');

const packages = {
  'darwin arm64': '@ahasend/cli-darwin-arm64',
  'darwin x64': '@ahasend/cli-darwin-x64',
  'linux arm64': '@ahasend/cli-linux-arm64',
  'linux x64': '@ahasend/cli-linux-x64',
  'win32 arm64': '@ahasend/cli-win32-arm64',
  'win32 x64': '@ahasend/cli-win32-x64',
};

const otherChannels =
  'You can also install ahasend with Homebrew, the install scripts, Scoop, a .deb\n' +
  'or .rpm package, or a binary download: https://ahasend.com/docs/cli/installation';

function fail(message) {
  process.stderr.write(`ahasend: ${message}\n`);
  process.exit(1);
}

function binaryPath() {
  if (process.env.AHASEND_BINARY_PATH) {
    return process.env.AHASEND_BINARY_PATH;
  }
  const platform = `${process.platform} ${process.arch}`;
  const name = packages[platform];
  if (!name) {
    fail(`@ahasend/cli has no binary for ${platform}.\n${otherChannels}`);
  }
  const binary = process.platform === 'win32' ? 'ahasend.exe' : 'ahasend';
  try {
    return require.resolve(`${name}/bin/${binary}`);
  } catch {
    fail(
      `the package ${name}, which holds the ahasend binary for ${platform}, is not installed.\n` +
        '@ahasend/cli installs it as an optional dependency, so it is missing when optional\n' +
        'dependencies are left out, for example with --omit=optional, or when a lockfile\n' +
        'made on another platform is installed. Install @ahasend/cli again with its\n' +
        'optional dependencies.\n' +
        otherChannels,
    );
  }
}

const binary = binaryPath();
const child = spawn(binary, process.argv.slice(2), { stdio: 'inherit' });

// A Ctrl+C in the terminal reaches the binary itself, which is in the same
// process group, so the launcher ignores SIGINT and does not forward it. A
// SIGTERM or SIGHUP may be sent to the launcher alone, so it forwards them.
// Windows has no such signals to forward: there the binary gets the console's
// own events, and sending it SIGHUP would throw.
process.on('SIGINT', () => {});
if (process.platform !== 'win32') {
  for (const signal of ['SIGTERM', 'SIGHUP']) {
    process.on(signal, () => {
      child.kill(signal);
    });
  }
}

child.on('error', (error) => {
  fail(`cannot run ${binary}: ${error.message}`);
});

// The launcher ends the way the binary did: with its exit code, or by the
// signal that ended it, raised again once the launcher's own handler is gone.
// Node ignores some signals, such as SIGPIPE, and handles others itself, such
// as SIGUSR1; the launcher then is still running, and exits with the code a
// shell gives a process that signal ended.
child.on('exit', (code, signal) => {
  if (signal) {
    process.removeAllListeners(signal);
    process.kill(process.pid, signal);
    process.exit(128 + (constants.signals[signal] || 0));
  }
  process.exit(code);
});

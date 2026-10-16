#!/usr/bin/env node
// Stands in for the ahasend binary in the signal tests of npm.mjs, which run
// it through the npm launcher with AHASEND_BINARY_PATH. It appends "ready",
// then the name of each signal it gets, to the file RECORDER_LOG names.
// RECORDER_END says how it ends one second after the first signal, so a
// second signal sent with the first is recorded too: "exit" exits with code
// 7, and "raise" ends by that signal. Without RECORDER_END it handles no
// signal, so the first one ends it.
'use strict';

const fs = require('node:fs');

const log = process.env.RECORDER_LOG;
const end = process.env.RECORDER_END;

if (end) {
  let first = null;
  for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
    process.on(signal, () => {
      fs.appendFileSync(log, `${signal}\n`);
      if (first) {
        return;
      }
      first = signal;
      setTimeout(() => {
        if (end === 'exit') {
          process.exit(7);
        }
        process.removeAllListeners(first);
        process.kill(process.pid, first);
      }, 1000);
    });
  }
}
fs.appendFileSync(log, 'ready\n');
// Ends by itself, so a test whose signal never arrives still ends
setTimeout(() => process.exit(3), 10000);

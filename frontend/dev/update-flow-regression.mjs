import assert from 'node:assert/strict';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { build } from 'esbuild';

const nativeSetTimeout = globalThis.setTimeout;
const nativeClearTimeout = globalThis.clearTimeout;
const testDir = await mkdtemp(path.join(tmpdir(), 'lannventory-update-flow-'));
const bundlePath = path.join(testDir, 'update-flow.mjs');

const buildResult = await build({
  entryPoints: [path.resolve('src/functions/updateFlow.ts')],
  bundle: true,
  format: 'esm',
  platform: 'browser',
  write: false,
});
await writeFile(bundlePath, buildResult.outputFiles[0].contents);

globalThis.window = {
  confirm: () => true,
  setTimeout: (callback) => nativeSetTimeout(callback, 0),
  clearTimeout: (timer) => nativeClearTimeout(timer),
};

const status = {
  currentVersion: '0.1.0-beta.13',
  channel: 'beta',
  latestVersion: '0.1.0-beta.14',
  available: true,
  publishedAt: '',
  releaseUrl: '',
  releaseSummary: '',
  installSupported: true,
  installReason: '',
  message: '',
  updating: false,
  automaticCheck: false,
  automatic: false,
  intervalHours: 24,
  lastChecked: '',
  snapshotBaseVersion: '',
};

const progress = (overrides = {}) => ({
  attemptId: 'attempt-new',
  status: 'running',
  stage: 'preparing',
  previousVersion: status.currentVersion,
  targetVersion: status.latestVersion,
  startedAt: new Date(Date.now() - 60_000).toISOString(),
  updatedAt: new Date().toISOString(),
  completedAt: '',
  backupPath: '/var/lib/lannventory/update-backups/test',
  backupCreated: false,
  failedStage: '',
  error: '',
  serviceRestored: false,
  systemdUnit: 'lannventory-update-test',
  ...overrides,
});

const jsonResponse = (value, responseStatus = 200) => new Response(JSON.stringify(value), {
  status: responseStatus,
  headers: { 'content-type': 'application/json' },
});

const waitFor = async (predicate, label, timeoutMs = 500) => {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await new Promise((resolve) => nativeSetTimeout(resolve, 2));
  }
  throw new Error(`Timed out waiting for ${label}`);
};

const loadFlow = async (name) => import(`${pathToFileURL(bundlePath).href}?scenario=${name}-${Date.now()}-${Math.random()}`);

function installMock({ baseline, progressSequence, versionSequence = [status.latestVersion], applyError, delayProgress = false }) {
  let progressCalls = 0;
  let applyCalls = 0;
  let activeProgressRequests = 0;
  let maxActiveProgressRequests = 0;
  const sequence = [...progressSequence];
  const versions = [...versionSequence];
  let lastProgress = sequence.at(-1) ?? baseline;

  globalThis.fetch = async (input, init = {}) => {
    const url = String(input);
    if (url.startsWith('/api/update/progress')) {
      activeProgressRequests++;
      maxActiveProgressRequests = Math.max(maxActiveProgressRequests, activeProgressRequests);
      try {
        if (delayProgress) await new Promise((resolve) => nativeSetTimeout(resolve, 3));
        const next = progressCalls++ === 0 ? baseline : (sequence.shift() ?? lastProgress);
        if (next instanceof Error) throw next;
        lastProgress = next;
        return jsonResponse(next);
      } finally {
        activeProgressRequests--;
      }
    }
    if (url === '/api/update/apply' && init.method === 'POST') {
      applyCalls++;
      if (applyError) throw applyError;
      return jsonResponse({
        version: status.latestVersion,
        scheduled: true,
        message: 'scheduled',
        backupPath: '/var/lib/lannventory/update-backups/test',
        attemptId: 'attempt-new',
      }, 202);
    }
    if (url === '/api/version') {
      const next = versions.length > 1 ? versions.shift() : versions[0];
      if (next instanceof Error) throw next;
      return jsonResponse(next);
    }
    if (url.startsWith('/api/update/status')) return jsonResponse({ ...status, available: false, currentVersion: status.latestVersion });
    throw new Error(`Unexpected request: ${init.method ?? 'GET'} ${url}`);
  };

  return {
    applyCalls: () => applyCalls,
    progressCalls: () => progressCalls,
    maxActiveProgressRequests: () => maxActiveProgressRequests,
  };
}

async function testNormalClockSkewedUpdate() {
  const api = installMock({
    baseline: progress({ attemptId: 'attempt-old', status: 'complete', stage: 'complete' }),
    progressSequence: [
      progress({ stage: 'downloading' }),
      progress({ stage: 'restarting', backupCreated: true }),
      progress({ status: 'complete', stage: 'complete', backupCreated: true, serviceRestored: true }),
    ],
    delayProgress: true,
  });
  const flow = await loadFlow('normal');
  const stages = [];
  let completed;
  const handle = await flow.startUpdateFlow(status, {
    onProgress: (next) => stages.push(next.stage),
    onComplete: (next) => { completed = next; },
  });
  await waitFor(() => completed, 'clock-skewed update completion');
  assert.equal(completed.status, 'complete');
  assert.ok(stages.includes('downloading'));
  assert.ok(stages.includes('restarting'));
  assert.equal(api.applyCalls(), 1);
  assert.equal(api.maxActiveProgressRequests(), 1, 'polls must not overlap');
  const callsAtCompletion = api.progressCalls();
  await new Promise((resolve) => nativeSetTimeout(resolve, 15));
  assert.equal(api.progressCalls(), callsAtCompletion, 'terminal state must stop polling');
  handle?.cancel();
}

async function testRestartAndRepeatedUnavailability() {
  installMock({
    baseline: progress({ attemptId: '', status: 'idle', stage: 'idle', targetVersion: '', startedAt: '' }),
    progressSequence: [
      progress({ stage: 'preparing' }),
      new TypeError('connection reset'),
      new TypeError('connection refused'),
      new TypeError('connection refused'),
      progress({ stage: 'health-check', backupCreated: true }),
      progress({ status: 'complete', stage: 'complete', backupCreated: true, serviceRestored: true }),
      progress({ status: 'complete', stage: 'complete', backupCreated: true, serviceRestored: true }),
    ],
    versionSequence: [new TypeError('backend restarting'), status.latestVersion],
  });
  const flow = await loadFlow('restart');
  const messages = [];
  let completed = false;
  await flow.startUpdateFlow(status, {
    onMessage: (message) => messages.push(message),
    onComplete: () => { completed = true; },
  });
  await waitFor(() => completed, 'reconnection after backend restart');
  assert.ok(messages.some((message) => message.includes('temporarily unreachable')));
  assert.ok(messages.some((message) => message.includes('waiting for the restarted API')));
}

async function testLostScheduleResponseRecovers() {
  const api = installMock({
    baseline: progress({ attemptId: 'attempt-old', status: 'complete', stage: 'complete' }),
    progressSequence: [
      progress({ stage: 'restarting', backupCreated: true }),
      new TypeError('service restart'),
      progress({ status: 'complete', stage: 'complete', backupCreated: true, serviceRestored: true }),
    ],
    applyError: new TypeError('connection closed before response'),
  });
  const flow = await loadFlow('lost-response');
  let completed = false;
  const handle = await flow.startUpdateFlow(status, { onComplete: () => { completed = true; } });
  assert.ok(handle, 'network loss after scheduling must keep the watcher active');
  await waitFor(() => completed, 'completion after lost scheduling response');
  assert.equal(api.applyCalls(), 1);
}

async function testTerminalFailure() {
  installMock({
    baseline: progress({ attemptId: '', status: 'idle', stage: 'idle', targetVersion: '', startedAt: '' }),
    progressSequence: [progress({
      status: 'failed',
      stage: 'installing',
      failedStage: 'installing',
      error: 'Package installation failed.',
      backupCreated: true,
      serviceRestored: true,
    })],
  });
  const flow = await loadFlow('failed');
  let error = '';
  let installing = true;
  await flow.startUpdateFlow(status, {
    onError: (message) => { if (message) error = message; },
    onInstalling: (value) => { installing = value; },
  });
  await waitFor(() => Boolean(error), 'terminal update failure');
  assert.equal(error, 'Package installation failed.');
  assert.equal(installing, false);
}

async function testCancelAndResumePersistedAttempt() {
  const api = installMock({
    baseline: progress({ status: 'running', stage: 'restarting', backupCreated: true }),
    progressSequence: [
      progress({ status: 'running', stage: 'restarting', backupCreated: true }),
      progress({ status: 'complete', stage: 'complete', backupCreated: true, serviceRestored: true }),
    ],
  });
  const flow = await loadFlow('resume');
  const first = flow.resumeUpdateFlow(progress({ stage: 'restarting', backupCreated: true }));
  await waitFor(() => api.progressCalls() >= 1, 'initial resumed poll');
  first?.cancel();
  const callsAfterClose = api.progressCalls();
  await new Promise((resolve) => nativeSetTimeout(resolve, 15));
  assert.equal(api.progressCalls(), callsAfterClose, 'closing must clean up its poller');

  let completed = false;
  flow.resumeUpdateFlow(progress({ stage: 'restarting', backupCreated: true }), {
    onComplete: () => { completed = true; },
  });
  await waitFor(() => completed, 'reopened dialog completion');
}

async function testDuplicateSubmissionPrevention() {
  const api = installMock({
    baseline: progress({ attemptId: '', status: 'idle', stage: 'idle', targetVersion: '', startedAt: '' }),
    progressSequence: [progress({ status: 'complete', stage: 'complete', backupCreated: true, serviceRestored: true })],
  });
  const flow = await loadFlow('duplicate');
  const first = flow.startUpdateFlow(status);
  const second = flow.startUpdateFlow(status);
  await Promise.all([first, second]);
  await waitFor(() => api.progressCalls() >= 2, 'duplicate flow terminal poll');
  assert.equal(api.applyCalls(), 1, 'only one update submission is allowed');
}

try {
  await testNormalClockSkewedUpdate();
  await testRestartAndRepeatedUnavailability();
  await testLostScheduleResponseRecovers();
  await testTerminalFailure();
  await testCancelAndResumePersistedAttempt();
  await testDuplicateSubmissionPrevention();
  console.log('Update flow regression checks passed.');
} finally {
  await rm(testDir, { recursive: true, force: true });
}

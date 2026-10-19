/**
 * In-memory mock of the HBC API (all endpoints of SPEC §4.1). Used by the Nitro proxy
 * when `mockApi` is on, and by unit tests. State lives in the process memory.
 */
import type { MockRequest, MockResponse } from './types';
import { MockError, internal } from './errors';
import { buildFixtureState } from './fixtures';
import type { MockState } from './state';
import type { Context } from './routes';
import { RawResponse, authenticate, resolve } from './routes';

export type { MockRequest, MockResponse } from './types';
export { DEMO_PASSWORD } from './fixtures';

let state: MockState = buildFixtureState();

/** Rebuilds the state from fixtures (fresh UUIDs). */
export function resetMockState(): void {
  state = buildFixtureState();
}

export function handleMock(req: MockRequest): MockResponse {
  try {
    const { route, params } = resolve(req.method, req.path);
    const viewer = route.auth ? authenticate(state, req.token) : undefined;
    const ctx = { state, req, params, viewer } as Context;
    const result = route.handler(ctx);
    return result instanceof RawResponse ? result.response : { status: 200, body: result };
  } catch (err) {
    const error = err instanceof MockError ? err : internal();
    if (!(err instanceof MockError)) {
      console.error('[mock-api]', err);
    }
    return { status: error.status, body: error.toBody() };
  }
}

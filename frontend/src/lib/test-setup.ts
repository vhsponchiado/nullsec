import { afterEach } from 'vitest'
import { cleanup } from '@testing-library/react'
import '@testing-library/jest-dom/vitest'

// vitest doesn't expose `afterEach` as a global (test.globals isn't set, to
// keep every test file's imports explicit) — so @testing-library/react's own
// auto-cleanup, which only registers itself when it detects a global
// `afterEach`, never kicks in unless this does it by hand. Without this,
// each test's rendered DOM piles up on top of the previous one, tests
// bleed into each other, and any query that matches more than one instance
// (e.g. two "Test" buttons) starts throwing.
afterEach(() => {
  cleanup()
})

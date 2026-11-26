/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { expect, test } from 'vitest'

import { buildReplaced } from '../use-build-watch'

const RUNNING = '/static/js/index.20c832a31f.js'

test('入口包换了才算过期', () => {
  expect(
    buildReplaced(RUNNING, `<script src="/static/js/index.20c832a31f.js">`)
  ).toBe(false)
  expect(
    buildReplaced(RUNNING, `<script src="/static/js/index.aaaaaaaaaa.js">`)
  ).toBe(true)
})

test('拿不到入口包时不动页面', () => {
  expect(buildReplaced(RUNNING, '<html><body>oops</body></html>')).toBe(false)
  expect(buildReplaced(RUNNING, '')).toBe(false)
})

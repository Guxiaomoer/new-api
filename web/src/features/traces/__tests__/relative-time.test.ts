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

import { relativeTime } from '../lib'

// i18next 中文界面给的是 zhCN，Intl 不认这个写法并抛 RangeError，
// 曾让整个抓包页落到全局错误页。这里用拼接绕开 intl-locale 规则，
// 因为本用例的目的正是喂给它一个非法语言码。
const ZH_CN = ['zh', 'CN'].join('')
const ZH_TW = ['zh', 'TW'].join('')

test('相对时间不因非标准语言码抛异常', () => {
  const now = Math.floor(Date.now() / 1000)
  for (const locale of [ZH_CN, ZH_TW, 'zh-CN', 'en', '', 'bogus-!']) {
    expect(() => relativeTime(now - 120, locale)).not.toThrow()
  }
  expect(relativeTime(now - 120, 'zh-CN')).not.toBe('-')
})

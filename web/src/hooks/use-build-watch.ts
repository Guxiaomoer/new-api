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
import { useEffect } from 'react'

const RELOAD_STAMP_KEY = 'build-watch:reloaded-at'
const RELOAD_COOLDOWN_MS = 10 * 60 * 1000
const INDEX_SCRIPT_RE = /\/static\/js\/index\.[A-Za-z0-9]+\.js/

function runningIndexScript(): string | null {
  const script = document.querySelector<HTMLScriptElement>(
    'script[src*="/static/js/index."]'
  )
  return script?.getAttribute('src') ?? null
}

/** 服务器当前入口包与正在运行的不一致时返回 true。 */
export function buildReplaced(running: string, html: string): boolean {
  const served = html.match(INDEX_SCRIPT_RE)?.[0]
  return Boolean(served && served !== running)
}

/**
 * 部署后旧标签页仍在跑上一版前端，菜单和页面都不会更新，只能等用户手动刷新。
 * 回到标签页时比对服务器当前入口包，发现换了就整页重载一次。
 */
export function useBuildWatch() {
  useEffect(() => {
    if (import.meta.env.MODE !== 'production') return
    const running = runningIndexScript()
    if (!running) return

    let checking = false
    const check = async () => {
      if (checking || document.visibilityState !== 'visible') return
      if (
        Date.now() -
          Number(sessionStorage.getItem(RELOAD_STAMP_KEY) ?? 0) <
        RELOAD_COOLDOWN_MS
      ) {
        return
      }
      checking = true
      try {
        const response = await fetch('/', { cache: 'no-store' })
        if (buildReplaced(running, await response.text())) {
          sessionStorage.setItem(RELOAD_STAMP_KEY, String(Date.now()))
          window.location.reload()
        }
      } catch {
        // 离线或请求失败时保持现状，下次回到页面再试。
      } finally {
        checking = false
      }
    }

    const onVisibility = () => {
      if (document.visibilityState === 'visible') void check()
    }
    document.addEventListener('visibilitychange', onVisibility)
    return () => document.removeEventListener('visibilitychange', onVisibility)
  }, [])
}

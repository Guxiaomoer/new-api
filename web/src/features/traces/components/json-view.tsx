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
import { useState } from 'react'
import { ChevronRight } from 'lucide-react'

import { CopyButton } from '@/components/copy-button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { cn } from '@/lib/utils'

const LONG_STRING = 80

type JsonViewProps = {
  value: unknown
  expandAll?: boolean
  maxHeight?: string
}

export function JsonView({ value, expandAll, maxHeight }: JsonViewProps) {
  if (value === undefined) return null
  return (
    <div
      className={cn(
        'bg-background/60 min-w-0 overflow-auto rounded-md border p-2 font-mono text-[11px] leading-5',
        maxHeight ?? 'max-h-[60vh]'
      )}
    >
      <JsonNode
        key={expandAll ? 'all' : 'part'}
        value={value}
        name={null}
        depth={0}
        expandAll={Boolean(expandAll)}
      />
    </div>
  )
}

type JsonNodeProps = {
  value: unknown
  name: string | null
  depth: number
  expandAll: boolean
  isLast?: boolean
}

function JsonNode({ value, name, depth, expandAll, isLast }: JsonNodeProps) {
  const isContainer = value !== null && typeof value === 'object'
  const [open, setOpen] = useState(expandAll || depth < 2)

  if (!isContainer) {
    return (
      <div className='flex min-w-0 gap-1 break-words whitespace-pre-wrap'>
        {name !== null && <span className='text-sky-600 dark:text-sky-400'>{name}:</span>}
        <PrimitiveValue value={value} />
        {!isLast && <span className='text-muted-foreground'>,</span>}
      </div>
    )
  }

  const entries = Array.isArray(value)
    ? value.map((item, index) => [String(index), item] as const)
    : Object.entries(value as Record<string, unknown>)
  const bracket = Array.isArray(value) ? ['[', ']'] : ['{', '}']
  const count = entries.length

  if (count === 0) {
    return (
      <div className='flex min-w-0 gap-1'>
        {name !== null && <span className='text-sky-600 dark:text-sky-400'>{name}:</span>}
        <span className='text-muted-foreground'>
          {bracket[0]}
          {bracket[1]}
        </span>
        {!isLast && <span className='text-muted-foreground'>,</span>}
      </div>
    )
  }

  return (
    <div className='min-w-0'>
      <button
        type='button'
        onClick={() => setOpen((prev) => !prev)}
        className='hover:bg-muted flex min-w-0 items-center gap-0.5 rounded px-0.5 text-left'
      >
        <ChevronRight
          className={cn(
            'text-muted-foreground size-3 shrink-0 transition-transform',
            open && 'rotate-90'
          )}
        />
        {name !== null && <span className='text-sky-600 dark:text-sky-400'>{name}:</span>}
        <span className='text-muted-foreground'>
          {bracket[0]}
          {open ? '' : `…${count}${bracket[1]}`}
        </span>
      </button>
      {open && (
        <div className='border-border/60 ml-1.5 border-l pl-2'>
          {entries.map(([key, item], index) => (
            <JsonNode
              key={key}
              value={item}
              name={key}
              depth={depth + 1}
              expandAll={expandAll}
              isLast={index === count - 1}
            />
          ))}
          <div className='text-muted-foreground'>
            {bracket[1]}
            {!isLast && ','}
          </div>
        </div>
      )}
    </div>
  )
}

function PrimitiveValue({ value }: { value: unknown }) {
  if (typeof value === 'string') {
    if (value.length > LONG_STRING) {
      return <LongString value={value} />
    }
    return (
      <span className='break-words whitespace-pre-wrap text-emerald-700 dark:text-emerald-400'>
        "{value}"
      </span>
    )
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return <span className='text-amber-700 dark:text-amber-400'>{String(value)}</span>
  }
  return <span className='text-muted-foreground'>null</span>
}

function LongString({ value }: { value: string }) {
  return (
    <Popover>
      <PopoverTrigger
        render={
          <button
            type='button'
            className='max-w-full truncate text-left text-emerald-700 underline decoration-dotted dark:text-emerald-400'
            title={value}
          />
        }
      >
        "{value.slice(0, LONG_STRING)}…" ({value.length})
      </PopoverTrigger>
      <PopoverContent
        align='start'
        sideOffset={6}
        className='w-[min(40rem,calc(100vw-2rem))] p-0'
      >
        <div className='flex items-center justify-between gap-2 border-b px-2 py-1'>
          <span className='text-muted-foreground text-[10px]'>
            {value.length} chars
          </span>
          <CopyButton value={value} size='icon' className='size-6' />
        </div>
        <pre className='max-h-[50vh] overflow-auto p-2 font-mono text-[11px] break-words whitespace-pre-wrap'>
          {value}
        </pre>
      </PopoverContent>
    </Popover>
  )
}

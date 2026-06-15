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
import { type Table } from '@tanstack/react-table'
import { ShieldOff, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { type User } from '../types'
import { batchManageUsersApiRestriction } from '../api'
import { useUsers } from './users-provider'

interface DataTableBulkActionsProps {
  table: Table<User>
}

export function DataTableBulkActions({ table }: DataTableBulkActionsProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useUsers()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [restrictDialogOpen, setRestrictDialogOpen] = useState(false)
  const [restrictMessage, setRestrictMessage] = useState(
    t('Your API access has been restricted. Please contact an administrator.')
  )

  const selectedIds = table
    .getFilteredSelectedRowModel()
    .rows.map((row) => row.original.id)

  const handleBatchApiRestriction = async (restricted: boolean) => {
    if (selectedIds.length === 0) {
      toast.error(t('No user selected'))
      return
    }

    setIsSubmitting(true)
    try {
      const result = await batchManageUsersApiRestriction({
        ids: selectedIds,
        action: restricted ? 'restrict_api' : 'unrestrict_api',
        message: restricted ? restrictMessage : '',
      })

      if (result.success) {
        toast.success(
          restricted
            ? t('API access restricted successfully')
            : t('API access restored successfully')
        )
        table.resetRowSelection()
        triggerRefresh()
        setRestrictDialogOpen(false)
      } else {
        toast.error(result.message || t('Operation failed'))
      }
    } catch (error) {
      toast.error(t('Operation failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName='user'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={isSubmitting}
          onClick={() => setRestrictDialogOpen(true)}
        >
          <ShieldOff className='mr-2 h-4 w-4' />
          {t('Restrict API')}
        </Button>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={isSubmitting}
          onClick={() => handleBatchApiRestriction(false)}
        >
          <ShieldCheck className='mr-2 h-4 w-4' />
          {t('Unrestrict API')}
        </Button>
      </BulkActionsToolbar>

      <Dialog open={restrictDialogOpen} onOpenChange={setRestrictDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Restrict API Access')}</DialogTitle>
            <DialogDescription>
              {t('Restrict API access for')} {selectedIds.length}{' '}
              {t('user(s)')}. {t('They will not be able to use API tokens.')}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4 py-4'>
            <div className='space-y-2'>
              <Label htmlFor='restrict-message'>
                {t('Restriction Message')}
              </Label>
              <Input
                id='restrict-message'
                value={restrictMessage}
                onChange={(e) => setRestrictMessage(e.target.value)}
                placeholder={t(
                  'Your API access has been restricted. Please contact an administrator.'
                )}
              />
              <p className='text-muted-foreground text-sm'>
                {t(
                  'This message will be returned to users when they try to use the API.'
                )}
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setRestrictDialogOpen(false)}
              disabled={isSubmitting}
            >
              {t('Cancel')}
            </Button>
            <Button
              variant='destructive'
              onClick={() => handleBatchApiRestriction(true)}
              disabled={isSubmitting}
            >
              {isSubmitting ? t('Processing...') : t('Confirm Restrict')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

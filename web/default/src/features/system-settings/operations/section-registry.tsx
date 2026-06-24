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
import { CommunityMonitor } from '@/features/community-monitor'
import { SystemBehaviorSection } from '../general/system-behavior-section'
import { EmailSettingsSection } from '../integrations/email-settings-section'
import { MonitoringSettingsSection } from '../integrations/monitoring-settings-section'
import { WorkerSettingsSection } from '../integrations/worker-settings-section'
import { LogSettingsSection } from '../maintenance/log-settings-section'
import { PerformanceSection } from '../maintenance/performance-section'
import { UpdateCheckerSection } from '../maintenance/update-checker-section'
import { CommunityCheckinBotSection } from './community-checkin-bot-section'
import { CommunitySyncSection } from './community-sync-section'
import { MaintenanceResponseSection } from './maintenance-response-section'
import { ResponseFilterSection } from './response-filter-section'
import type { OperationsSettings } from '../types'
import { createSectionRegistry } from '../utils/section-registry'

const OPERATIONS_SECTIONS = [
  {
    id: 'behavior',
    titleKey: 'System Behavior',
    build: (settings: OperationsSettings) => (
      <SystemBehaviorSection
        defaultValues={{
          DefaultCollapseSidebar: settings.DefaultCollapseSidebar,
          DemoSiteEnabled: settings.DemoSiteEnabled,
          SelfUseModeEnabled: settings.SelfUseModeEnabled,
        }}
      />
    ),
  },
  {
    id: 'alerts',
    titleKey: 'Monitoring & Alerts',
    build: (settings: OperationsSettings) => (
      <MonitoringSettingsSection
        defaultValues={{
          QuotaRemindThreshold: settings.QuotaRemindThreshold,
          'perf_metrics_setting.enabled':
            settings['perf_metrics_setting.enabled'] ?? true,
          'perf_metrics_setting.flush_interval':
            settings['perf_metrics_setting.flush_interval'] ?? 5,
          'perf_metrics_setting.bucket_time':
            settings['perf_metrics_setting.bucket_time'] ?? 'hour',
          'perf_metrics_setting.retention_days':
            settings['perf_metrics_setting.retention_days'] ?? 0,
        }}
      />
    ),
  },
  {
    id: 'response-filter',
    titleKey: 'Upstream Response Filter',
    build: (settings: OperationsSettings) => (
      <ResponseFilterSection
        defaultValues={{
          'general_setting.upstream_pollution_keywords':
            settings['general_setting.upstream_pollution_keywords'] ?? '',
          'general_setting.upstream_pollution_disable_channel':
            settings['general_setting.upstream_pollution_disable_channel'] ?? true,
          'general_setting.upstream_custom_response_http_200_enabled':
            settings['general_setting.upstream_custom_response_http_200_enabled'] ??
            true,
          'general_setting.upstream_pollution_message':
            settings['general_setting.upstream_pollution_message'] ?? '',
          'general_setting.upstream_failure_message':
            settings['general_setting.upstream_failure_message'] ?? '',
          'general_setting.upstream_intercept_audit_enabled':
            settings['general_setting.upstream_intercept_audit_enabled'] ?? true,
          'general_setting.upstream_intercept_audit_retention_days':
            settings['general_setting.upstream_intercept_audit_retention_days'] ?? 30,
        }}
      />
    ),
  },
  {
    id: 'maintenance-response',
    titleKey: 'Maintenance Response',
    build: (settings: OperationsSettings) => (
      <MaintenanceResponseSection
        defaultValues={{
          'general_setting.global_maintenance_enabled':
            settings['general_setting.global_maintenance_enabled'] ?? false,
          'general_setting.global_maintenance_message':
            settings['general_setting.global_maintenance_message'] ?? '',
        }}
      />
    ),
  },
  {
    id: 'community-sync',
    titleKey: 'Community Member Sync',
    build: (settings: OperationsSettings) => (
      <CommunitySyncSection
        defaultValues={{
          'community_sync.enabled': settings['community_sync.enabled'] ?? false,
          'community_sync.endpoint':
            settings['community_sync.endpoint'] ??
            'https://dc.hhhl.cc/api/chat/rooms/members',
          'community_sync.room_id': settings['community_sync.room_id'] ?? 'ani5zrxyl7',
          'community_sync.authorization':
            settings['community_sync.authorization'] ?? '',
          'community_sync.fingerprint': settings['community_sync.fingerprint'] ?? '',
          'community_sync.interval_minutes':
            settings['community_sync.interval_minutes'] ?? 5,
          'community_sync.protected_users':
            settings['community_sync.protected_users'] ??
            '1456671048@qq.com\nlufeng2820@163.com',
        }}
      />
    ),
  },
  {
    id: 'community-checkin-bot',
    titleKey: '社区签到机器人维护',
    build: (settings: OperationsSettings) => (
      <CommunityCheckinBotSection
        defaultValues={{
          'community_checkin_bot.enabled':
            settings['community_checkin_bot.enabled'] ?? false,
          'community_checkin_bot.bot_user_id':
            settings['community_checkin_bot.bot_user_id'] ?? 'amlarbic93',
          'community_checkin_bot.bot_name':
            settings['community_checkin_bot.bot_name'] ?? 'Guxiaomo',
          'community_checkin_bot.interval_seconds':
            settings['community_checkin_bot.interval_seconds'] ?? 30,
          'community_checkin_bot.min_usd':
            settings['community_checkin_bot.min_usd'] ?? 2,
          'community_checkin_bot.max_usd':
            settings['community_checkin_bot.max_usd'] ?? 5,
          'community_checkin_bot.last_message_id':
            settings['community_checkin_bot.last_message_id'] ?? '',
        }}
      />
    ),
  },
  {
    id: 'community-monitor',
    titleKey: 'Community Monitor',
    build: () => <CommunityMonitor embedded />,
  },
  {
    id: 'email',
    titleKey: 'SMTP Email',
    build: (settings: OperationsSettings) => (
      <EmailSettingsSection
        defaultValues={{
          SMTPServer: settings.SMTPServer,
          SMTPPort: settings.SMTPPort,
          SMTPAccount: settings.SMTPAccount,
          SMTPFrom: settings.SMTPFrom,
          SMTPToken: settings.SMTPToken,
          SMTPSSLEnabled: settings.SMTPSSLEnabled,
          SMTPForceAuthLogin: settings.SMTPForceAuthLogin,
        }}
      />
    ),
  },
  {
    id: 'worker',
    titleKey: 'Worker Proxy',
    build: (settings: OperationsSettings) => (
      <WorkerSettingsSection
        defaultValues={{
          WorkerUrl: settings.WorkerUrl,
          WorkerValidKey: settings.WorkerValidKey,
          WorkerAllowHttpImageRequestEnabled:
            settings.WorkerAllowHttpImageRequestEnabled,
        }}
      />
    ),
  },
  {
    id: 'logs',
    titleKey: 'Log Maintenance',
    build: (settings: OperationsSettings) => (
      <LogSettingsSection
        defaultEnabled={Boolean(settings.LogConsumeEnabled)}
      />
    ),
  },
  {
    id: 'performance',
    titleKey: 'Performance',
    build: (settings: OperationsSettings) => (
      <PerformanceSection
        defaultValues={{
          'performance_setting.disk_cache_enabled':
            settings['performance_setting.disk_cache_enabled'] ?? false,
          'performance_setting.disk_cache_threshold_mb':
            settings['performance_setting.disk_cache_threshold_mb'] ?? 10,
          'performance_setting.disk_cache_max_size_mb':
            settings['performance_setting.disk_cache_max_size_mb'] ?? 1024,
          'performance_setting.disk_cache_path':
            settings['performance_setting.disk_cache_path'] ?? '',
          'performance_setting.monitor_enabled':
            settings['performance_setting.monitor_enabled'] ?? false,
          'performance_setting.monitor_cpu_threshold':
            settings['performance_setting.monitor_cpu_threshold'] ?? 90,
          'performance_setting.monitor_memory_threshold':
            settings['performance_setting.monitor_memory_threshold'] ?? 90,
          'performance_setting.monitor_disk_threshold':
            settings['performance_setting.monitor_disk_threshold'] ?? 95,
        }}
      />
    ),
  },
  {
    id: 'update-checker',
    titleKey: 'System maintenance',
    build: (
      _settings: OperationsSettings,
      currentVersion?: string | null,
      startTime?: number | null
    ) => (
      <UpdateCheckerSection
        currentVersion={currentVersion}
        startTime={startTime}
      />
    ),
  },
] as const

export type OperationsSectionId = (typeof OPERATIONS_SECTIONS)[number]['id']

const operationsRegistry = createSectionRegistry<
  OperationsSectionId,
  OperationsSettings,
  [string | null | undefined, number | null | undefined]
>({
  sections: OPERATIONS_SECTIONS,
  defaultSection: 'behavior',
  basePath: '/system-settings/operations',
  urlStyle: 'path',
})

export const OPERATIONS_SECTION_IDS = operationsRegistry.sectionIds
export const OPERATIONS_DEFAULT_SECTION = operationsRegistry.defaultSection
export const getOperationsSectionNavItems =
  operationsRegistry.getSectionNavItems
export const getOperationsSectionContent = operationsRegistry.getSectionContent
export const getOperationsSectionMeta = operationsRegistry.getSectionMeta

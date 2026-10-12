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
/**
 * Application-wide constants
 */

// System Configuration Defaults
export const DEFAULT_SYSTEM_NAME = 'New API'
export const DEFAULT_LOGO = '/logo.png'

// Agent Development (Dify) console port.
// Keep in sync with AGENT_DEV_PORT in the root .env: start.sh renders it into
// docker/dify/.env and publishes the dify nginx service on this host port.
export const AGENT_DEV_PORT = 8081

/**
 * URL of the external Dify "Agent Development" console.
 *
 * The dify stack is deployed beside the main site and exposed on a dedicated
 * plain-HTTP port (see AGENT_DEV_PORT), so the URL is derived from the host
 * the user is currently browsing. Returns an empty string outside the browser.
 */
export function getAgentDevUrl(): string {
  if (typeof window === 'undefined') return ''
  return `http://${window.location.hostname}:${AGENT_DEV_PORT}`
}

// LocalStorage Keys
export const STORAGE_KEYS = {
  SYSTEM_NAME: 'system_name',
  LOGO: 'logo',
  FOOTER_HTML: 'footer_html',
} as const

// Type definitions for date handling
type ISODateString = string

// Interface for date range objects
export interface IDateRange {
  startDate: ISODateString
  endDate: ISODateString
}

/**
 * Copy text to clipboard using modern API with fallback support
 * @param str - The text string to copy to clipboard
 */
export function directCopy(str: string): void {
  // Use modern clipboard API if available and in secure context
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(str).then(() => {
      console.warn('Text copied to clipboard successfully')
    }).catch((err) => {
      console.error('Failed to copy text using modern API: ', err)
      // Fallback to legacy method
      fallbackCopy(str)
    })
  }
  else {
    // Use fallback method for older browsers or non-secure contexts
    fallbackCopy(str)
  }
}

/**
 * Fallback clipboard copy method for older browsers
 * @param str - The text string to copy
 */
function fallbackCopy(str: string): void {
  // Create temporary textarea element
  const textArea = document.createElement('textarea')
  textArea.value = str
  // Position off-screen to avoid visual flash
  textArea.style.position = 'fixed'
  textArea.style.left = '-999999px'
  textArea.style.top = '-999999px'

  // Add to DOM, select and copy
  document.body.appendChild(textArea)
  textArea.focus()
  textArea.select()

  try {
    // Use legacy execCommand for copying
    document.execCommand('copy')
    console.warn('Text copied to clipboard using fallback method')
  }
  catch (err) {
    console.error('Fallback copy method failed: ', err)
  }

  // Clean up temporary element
  document.body.removeChild(textArea)
}

/**
 * Sanitize object property names by replacing hyphens with underscores
 * @param data - Object to sanitize
 * @param transform - Optional transformation function to apply to values
 */
export function sanitizePropertyNames(data: any, transform: (value: any) => any = (value: any) => value): void {
  Object.keys(data).forEach((prop) => {
    // Replace hyphens with underscores in property names
    data[prop.replace(/-/g, '_')] = transform(data[prop])
  })
}

/**
 * Get date range for the last 30 days
 * @returns IDateRange object with start and end dates (YYYY-MM-DD)
 */
export function getlast30DayRange(): IDateRange {
  const range: IDateRange = {} as IDateRange
  const today = new Date()
  // Calculate 30 days ago (2592000000 ms = 30 days)
  range.startDate = toDateOnly(new Date(today.valueOf() - 2592000000))
  range.endDate = toDateOnly(today)
  return range
}

/**
 * Convert a date to YYYY-MM-DD in UTC
 * @param date - Date object to clamp
 * @returns ISO date-only string (YYYY-MM-DD)
 */
export function toDateOnly(date: Date): ISODateString {
  const normalized = new Date(date.valueOf())
  normalized.setUTCHours(0)
  normalized.setUTCMinutes(0)
  normalized.setUTCSeconds(0)
  normalized.setUTCMilliseconds(0)
  return normalized.toISOString().split('T')[0]
}

/**
 * Format number with locale-specific formatting
 * @param num - Number to format (can be null or undefined)
 * @returns Formatted number string or '0' if invalid
 */
export function formatNumber(num: number | null | undefined): string {
  if (num == null || Number.isNaN(num))
    return '0'
  return new Intl.NumberFormat().format(num)
}

/**
 * Convert empty/null values to dash (-) for display purposes
 * @param value - Value to check and convert
 * @returns original value or dash if empty/null
 */
export function dashWhenEmptyString(value: string | string[] | undefined): string {
  if (Array.isArray(value)) {
    return value.length === 0 ? '-' : value.join('\n')
  }
  return value ? String(value) : '-'
}

/**
 * Safely get item from session storage
 * @param key - Storage key to retrieve
 * @returns Stored value or null if not found/error
 */
export function getSessionStorage(key: string): string | null {
  try {
    return window.sessionStorage.getItem(key)
  }
  catch (e) {
    console.warn('Session storage not available:', e)
    return null
  }
}

/**
 * Safely set item in session storage
 * @param key - Storage key
 * @param value - Value to store
 */
export function setSessionStorage(key: string, value: string): void {
  try {
    window.sessionStorage.setItem(key, value)
  }
  catch (e) {
    console.warn('Failed to set session storage:', e)
  }
}

/**
 * Safely remove item from session storage
 * @param key - Storage key to remove
 */
export function removeSessionStorage(key: string): void {
  try {
    window.sessionStorage.removeItem(key)
  }
  catch (e) {
    console.warn('Failed to remove from session storage:', e)
  }
}

/**
 * Calculate DMARC alignment percentage from record counts
 * @param item - Object containing record counts
 * @param item.total_count - Total message count
 * @param item.pass_count - Passing message count
 * @returns Percentage as number (0-100)
 */
export function calculatePassingPercentage(item: { total_count: number, pass_count: number }): number {
  if (item.total_count === 0)
    return 0
  return (item.pass_count / item.total_count) * 100
}

/**
 * Get color for percentage display based on value
 * @param percentage - Percentage number (e.g., 85.2)
 * @returns Color string for Vuetify chip
 */
export function getPercentageColor(percentage: number): string {
  if (percentage >= 99)
    return 'success'
  if (percentage >= 90)
    return 'warning'
  return 'error'
}

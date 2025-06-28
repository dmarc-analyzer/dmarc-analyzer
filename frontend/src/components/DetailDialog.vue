<script setup lang="ts">
import type { DataTableHeader } from 'vuetify/framework'
import type { DetailReportRow } from '@/services/dmarcService'
import { computed, ref, watch } from 'vue'
import { dashWhenEmptyString, directCopy, formatNumber } from '@/utils/utilities'

interface Props {
  modelValue?: boolean
  domain?: string
  source?: string
  startDate?: string
  endDate?: string
  detailData?: DetailReportRow[]
  loading?: boolean
  error?: string
}

const props = withDefaults(defineProps<Props>(), {
  modelValue: false,
  domain: '',
  source: '',
  startDate: '',
  endDate: '',
  detailData: () => [],
  loading: false,
  error: '',
})

// Component emit events for v-model and dialog lifecycle
const emit = defineEmits<{
  'update:modelValue': [value: boolean] // For v-model synchronization
  'close': [] // Emitted when dialog is closed
}>()

// Internal reactive state
const localDialog = ref(props.modelValue) // Local dialog visibility state for v-model
const searchQuery = ref('') // Search query for filtering table data

// Table headers configuration with consistent typing
const tableHeaders: DataTableHeader[] = [
  { title: 'Source IP', key: 'source_ip', sortable: false, width: '150px', align: 'center' },
  { title: 'Count', key: 'message_count', sortable: false, width: '100px', align: 'center' },
  { title: 'Report Org Name', key: 'report_org_name', sortable: false, width: '200px', align: 'center' },
  { title: 'Country', key: 'country', sortable: false, width: '120px', align: 'center' },
  { title: 'Source Host', key: 'source_host', sortable: false, width: '250px', align: 'center' },
  { title: 'Disposition', key: 'disposition', sortable: false, width: '120px', align: 'center' },
  { title: 'SPF', key: 'eval_spf', sortable: false, width: '100px', align: 'center' },
  { title: 'SPF Results', key: 'auth_spf_result', sortable: false, width: '150px', align: 'center' },
  { title: 'SPF Domain', key: 'auth_spf_domain', sortable: false, width: '200px', align: 'center' },
  { title: 'DKIM', key: 'eval_dkim', sortable: false, width: '100px', align: 'center' },
  { title: 'DKIM Results', key: 'auth_dkim_result', sortable: false, width: '150px', align: 'center' },
  { title: 'DKIM Selector', key: 'auth_dkim_selector', sortable: false, width: '250px', align: 'center' },
  { title: 'DKIM Domain', key: 'auth_dkim_domain', sortable: false, width: '200px', align: 'center' },
  { title: 'Policy Override', key: 'po_comment', width: '180px', align: 'center' },
  { title: 'Override Reason', key: 'po_reason', sortable: false, width: '180px', align: 'center' },
]

// Sync local dialog state with external v-model prop
watch(
  () => props.modelValue,
  (newValue) => {
    localDialog.value = newValue
  },
  { immediate: true },
)

// Emit updates when local dialog state changes
watch(localDialog, (newValue) => {
  emit('update:modelValue', newValue)
  if (!newValue) {
    emit('close')
  }
})

/**
 * Computed property that validates and returns detail data
 * Ensures data is always a valid array for table consumption
 */
const validatedDetailData = computed(() => {
  return Array.isArray(props.detailData) ? props.detailData : []
})

/**
 * Determine appropriate color based on SPF/DKIM authentication results
 * @param result - Authentication result string (e.g., 'pass', 'fail', 'neutral')
 * @returns Vuetify color name for consistent UI theming
 */
function getResultColor(result: string | undefined): string {
  if (!result) {
    return 'grey'
  }

  const normalizedResult = result.toLowerCase().trim()

  // Define color mapping for authentication results
  if (normalizedResult.includes('pass'))
    return 'success'
  if (normalizedResult.includes('fail'))
    return 'error'
  if (normalizedResult.includes('warn') || normalizedResult.includes('neutral'))
    return 'warning'

  return 'info' // Default for unknown results
}

/**
 * Convert country code to flag emoji using Unicode regional indicators
 * @param countryCode - Two-letter country code (ISO 3166-1 alpha-2)
 * @returns Flag emoji string or empty string if invalid
 */
function getCountryFlagEmoji(countryCode: string): string {
  if (!countryCode || !/^[A-Z]{2}$/i.test(countryCode)) {
    return ''
  }

  try {
    // Convert country code to flag emoji using regional indicator symbols
    // Formula: 127397 + charCode converts A-Z to regional indicator symbols
    const codePoints = countryCode
      .toUpperCase()
      .split('')
      .map(char => 127397 + char.charCodeAt(0))

    return String.fromCodePoint(...codePoints)
  }
  catch {
    // Fallback for any Unicode conversion errors
    return ''
  }
}

/**
 * Copy text to clipboard with console feedback
 * Uses the directCopy utility function to handle clipboard operations
 * @param text - Text to copy to clipboard
 */
function copyToClipboard(text: string) {
  directCopy(text)
  // TODO: Replace console.warn with proper toast notification for better UX
  console.warn(`Copied to clipboard: ${text}`)
}

/**
 * Close dialog and reset component state
 * Triggers close event emission via localDialog watcher
 */
function closeDialog() {
  localDialog.value = false
  searchQuery.value = '' // Clear search filter for next opening
}
</script>

<template>
  <!-- Detail dialog using native Vuetify dialog component -->
  <v-dialog
    v-model="localDialog"
    max-width="98vw"
    width="98vw"
    scrollable
    persistent
  >
    <v-card>
      <!-- Dialog header with title and close button -->
      <v-card-title class="d-flex align-center justify-space-between">
        <div class="dialog-title">
          <v-icon class="mr-2">
            mdi-information-outline
          </v-icon>
          Details for {{ source }}
        </div>
        <v-btn
          icon="mdi-close"
          variant="text"
          aria-label="Close dialog"
          @click="closeDialog"
        />
      </v-card-title>

      <v-divider />

      <!-- Dialog content area -->
      <v-card-text class="pa-0">
        <!-- Loading state using native Vuetify progress -->
        <div v-if="loading" class="loading-container">
          <v-progress-circular
            indeterminate
            color="primary"
            size="40"
            width="2"
          />
          <div class="mt-4 text-body-2">
            Loading detailed records...
          </div>
        </div>

        <!-- Error state using native Vuetify alert -->
        <v-alert
          v-if="error"
          type="error"
          variant="tonal"
          class="ma-4"
        >
          {{ error }}
        </v-alert>

        <!-- Empty state when no data is available -->
        <div
          v-if="!loading && !error && validatedDetailData.length === 0"
          class="no-data-container"
        >
          <v-icon size="64" color="grey-lighten-1" class="mb-4">
            mdi-database-off
          </v-icon>
          <div class="text-h6 mb-2">
            No Detail Records Found
          </div>
          <div class="text-body-2 text-medium-emphasis">
            No detailed records available for source "{{ source }}" in the selected time range.
          </div>
        </div>

        <!-- Data table for displaying DMARC detail records -->
        <v-data-table
          v-if="!loading && !error && validatedDetailData.length > 0"
          :headers="tableHeaders"
          :items="validatedDetailData"
          :items-per-page="100"
          :items-per-page-options="[100, 200, 500]"
          class="detail-table"
          density="compact"
          :search="searchQuery"
        >
          <!-- Search filter input for table records -->
          <template #top>
            <div class="pa-4">
              <v-text-field
                v-model="searchQuery"
                label="Search records (IP, host, domain, etc.)"
                prepend-inner-icon="mdi-magnify"
                variant="outlined"
                density="comfortable"
                clearable
                hide-details
                placeholder="Filter table data..."
                style="max-width: 400px;"
              />
            </div>
          </template>

          <!-- Clickable IP addresses with copy functionality -->
          <template #[`item.source_ip`]="{ item }">
            <v-btn
              variant="text"
              size="small"
              class="text-decoration-underline"
              :title="`Click to copy IP address: ${item.source_ip}`"
              @click="copyToClipboard(item.source_ip)"
            >
              {{ item.source_ip }}
            </v-btn>
          </template>

          <!-- Format number columns with thousand separators -->
          <template #[`item.message_count`]="{ item }">
            {{ formatNumber(item.message_count) }}
          </template>

          <!-- Report org name -->
          <template #[`item.report_org_name`]="{ item }">
            {{ dashWhenEmptyString(item.report_org_name) }}
          </template>

          <!-- Country with flag emoji (if valid country code) -->
          <template #[`item.country`]="{ item }">
            <span v-if="getCountryFlagEmoji(item.country)" class="country-flag">{{ getCountryFlagEmoji(item.country) }}</span>
            {{ dashWhenEmptyString(item.country) }}
          </template>

          <!-- Source host -->
          <template #[`item.source_host`]="{ item }">
            {{ dashWhenEmptyString(item.source_host) }}
          </template>

          <!-- Disposition result -->
          <template #[`item.disposition`]="{ item }">
            {{ dashWhenEmptyString(item.disposition) }}
          </template>

          <!-- SPF evaluation result with color coding -->
          <template #[`item.eval_spf`]="{ item }">
            <v-chip
              :color="getResultColor(item.eval_spf)"
              size="small"
            >
              {{ item.eval_spf }}
            </v-chip>
          </template>

          <!-- SPF auth results -->
          <template #[`item.auth_spf_result`]="{ item }">
            {{ dashWhenEmptyString(item.auth_spf_result) }}
          </template>

          <!-- SPF domains -->
          <template #[`item.auth_spf_domain`]="{ item }">
            {{ dashWhenEmptyString(item.auth_spf_domain) }}
          </template>

          <!-- DKIM evaluation result with color coding -->
          <template #[`item.eval_dkim`]="{ item }">
            <v-chip
              :color="getResultColor(item.eval_dkim)"
              size="small"
            >
              {{ item.eval_dkim }}
            </v-chip>
          </template>

          <!-- DKIM auth results -->
          <template #[`item.auth_dkim_result`]="{ item }">
            {{ dashWhenEmptyString(item.auth_dkim_result) }}
          </template>

          <!-- DKIM selectors -->
          <template #[`item.auth_dkim_selector`]="{ item }">
            {{ dashWhenEmptyString(item.auth_dkim_selector) }}
          </template>

          <!-- DKIM domains -->
          <template #[`item.auth_dkim_domain`]="{ item }">
            {{ dashWhenEmptyString(item.auth_dkim_domain) }}
          </template>

          <!-- Policy override indicator -->
          <template #[`item.po_comment`]="{ item }">
            {{ dashWhenEmptyString(item.po_comment) }}
          </template>

          <!-- Override reason -->
          <template #[`item.po_reason`]="{ item }">
            {{ dashWhenEmptyString(item.po_reason) }}
          </template>
        </v-data-table>
      </v-card-text>

      <!-- Dialog actions -->
      <v-card-actions class="px-4 py-3">
        <v-spacer />
        <v-btn
          color="primary"
          variant="text"
          @click="closeDialog"
        >
          Close
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style lang="scss" scoped>
// Dialog title styling
.dialog-title {
  font-size: 1.25rem;
  font-weight: 500;
  color: rgba(0, 0, 0, 0.87);
}

// Loading container centered layout
.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 200px;
  padding: 32px;
}

// No data state styling
.no-data-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 300px;
  padding: 32px;
  text-align: center;
}

// Detail table customizations
.detail-table {
  // Enable horizontal scrolling for very wide tables
  overflow-x: auto;

  // Responsive table behavior
  @media (max-width: 960px) {
    font-size: 0.875rem;
  }

  // Table cell content - preserve line breaks (\n) in text while allowing word wrapping
  // white-space: pre-line allows newlines to be displayed as line breaks
  :deep(td) {
    white-space: pre-line;
    overflow: visible;
    text-overflow: unset;
    min-width: fit-content;
    justify-content: center;
    text-align: center;
  }

  // Table headers - prevent text wrapping to keep headers compact
  :deep(th) {
    white-space: nowrap;
    min-width: fit-content;
    justify-content: center;
    text-align: center;
  }

  // Make IP addresses more prominent
  :deep(.v-btn) {
    text-transform: none;
    font-family: monospace;
    font-size: 0.875rem;
  }

  // Ensure table takes full width
  :deep(.v-table) {
    width: 100%;
    min-width: fit-content;
  }
}

// Country flag styling
.country-flag {
  font-size: 1.2em;
  line-height: 1;
}

// Responsive dialog sizing for mobile devices
@media (max-width: 768px) {
  :deep(.v-dialog) {
    margin: 8px;
    max-width: calc(100% - 16px) !important;
    width: calc(100% - 16px) !important;
  }
}

// Enable horizontal scrolling for dialog content when table is too wide
.v-card-text {
  overflow-x: auto;
}
</style>

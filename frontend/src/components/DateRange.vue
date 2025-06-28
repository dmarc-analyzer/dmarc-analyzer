<script setup lang="ts">
import { endOfMonth, format, parseISO, startOfMonth, subDays } from 'date-fns'
import { computed, ref, watch } from 'vue'

// Component props with TypeScript support
interface Props {
  initialStartDate?: string | null
  initialEndDate?: string | null
}

const props = withDefaults(defineProps<Props>(), {
  initialStartDate: null,
  initialEndDate: null,
})

// Component emits
const emit = defineEmits<{
  'update:date-range': [value: { startDate: string, endDate: string }]
}>()

// Reactive state for menu visibility
const startMenu = ref(false)
const endMenu = ref(false)

// Initialize with props or default to last 30 days
const today = new Date()
const thirtyDaysAgo = subDays(today, 30)

const localStartDate = ref(props.initialStartDate || format(thirtyDaysAgo, 'yyyy-MM-dd'))
const localEndDate = ref(props.initialEndDate || format(today, 'yyyy-MM-dd'))

// Computed properties for formatted dates display
const formattedStartDate = computed(() => {
  const date = localStartDate.value
  const dateObj = typeof date === 'string' ? parseISO(date) : date
  return format(dateObj, 'MMM dd, yyyy')
})

const formattedEndDate = computed(() => {
  const date = localEndDate.value
  const dateObj = typeof date === 'string' ? parseISO(date) : date
  return format(dateObj, 'MMM dd, yyyy')
})

// Date range validation
const isValidDateRange = computed(() => {
  if (!localStartDate.value || !localEndDate.value)
    return false

  const start = new Date(localStartDate.value)
  const end = new Date(localEndDate.value)

  return start <= end
})

// Predefined date range presets for quick selection
const presets = [
  {
    label: 'Last 7 days',
    startDate: format(subDays(today, 7), 'yyyy-MM-dd'),
    endDate: format(today, 'yyyy-MM-dd'),
  },
  {
    label: 'Last 30 days',
    startDate: format(subDays(today, 30), 'yyyy-MM-dd'),
    endDate: format(today, 'yyyy-MM-dd'),
  },
  {
    label: 'This month',
    startDate: format(startOfMonth(today), 'yyyy-MM-dd'),
    endDate: format(today, 'yyyy-MM-dd'),
  },
  {
    label: 'Last month',
    startDate: format(startOfMonth(subDays(startOfMonth(today), 1)), 'yyyy-MM-dd'),
    endDate: format(endOfMonth(subDays(startOfMonth(today), 1)), 'yyyy-MM-dd'),
  },
]

// Methods

/**
 * Apply current date range and emit to parent component
 */
function applyDateRange() {
  if (isValidDateRange.value) {
    emit('update:date-range', {
      startDate: localStartDate.value,
      endDate: localEndDate.value,
    })
  }
}

/**
 * Apply a predefined date range preset
 * @param preset - Preset configuration with label and date range
 * @param preset.label - Display label for the preset
 * @param preset.startDate - Start date in YYYY-MM-DD format
 * @param preset.endDate - End date in YYYY-MM-DD format
 */
function applyPreset(preset: { label: string, startDate: string, endDate: string }) {
  localStartDate.value = preset.startDate
  localEndDate.value = preset.endDate
  applyDateRange()
}

// Watchers

/**
 * Watch for date picker changes to ensure proper string format
 * Date pickers may return Date objects, convert to ISO string format
 */
watch(localStartDate, (newValue: any) => {
  if (newValue && typeof newValue === 'object' && newValue instanceof Date) {
    localStartDate.value = format(newValue, 'yyyy-MM-dd')
  }
})

watch(localEndDate, (newValue: any) => {
  if (newValue && typeof newValue === 'object' && newValue instanceof Date) {
    localEndDate.value = format(newValue, 'yyyy-MM-dd')
  }
})

/**
 * Watch for prop changes to update internal state
 */
watch(
  () => [props.initialStartDate, props.initialEndDate],
  ([newStartDate, newEndDate]) => {
    if (newStartDate && newStartDate !== localStartDate.value) {
      localStartDate.value = newStartDate
    }

    if (newEndDate && newEndDate !== localEndDate.value) {
      localEndDate.value = newEndDate
    }
  },
)
</script>

<template>
  <div class="date-range">
    <v-row>
      <v-col cols="12" sm="4">
        <v-menu
          v-model="startMenu"
          :close-on-content-click="false"
          transition="scale-transition"
          min-width="auto"
        >
          <template #activator="{ props: activatorProps }">
            <v-text-field
              v-model="formattedStartDate"
              label="Start Date"
              prepend-inner-icon="mdi-calendar"
              readonly
              v-bind="activatorProps"
              variant="outlined"
              density="compact"
              hide-details
            />
          </template>
          <v-date-picker
            v-model="localStartDate"
            @update:model-value="startMenu = false"
          />
        </v-menu>
      </v-col>

      <v-col cols="12" sm="4">
        <v-menu
          v-model="endMenu"
          :close-on-content-click="false"
          transition="scale-transition"
          min-width="auto"
        >
          <template #activator="{ props: activatorProps }">
            <v-text-field
              v-model="formattedEndDate"
              label="End Date"
              prepend-inner-icon="mdi-calendar"
              readonly
              v-bind="activatorProps"
              variant="outlined"
              density="compact"
              hide-details
            />
          </template>
          <v-date-picker
            v-model="localEndDate"
            @update:model-value="endMenu = false"
          />
        </v-menu>
      </v-col>

      <v-col cols="12" sm="4" class="d-flex align-center">
        <v-btn
          color="primary"
          :disabled="!isValidDateRange"
          @click="applyDateRange"
        >
          Apply
        </v-btn>

        <v-menu>
          <template #activator="{ props: activatorProps }">
            <v-btn
              class="ml-2"
              variant="outlined"
              v-bind="activatorProps"
            >
              Presets
            </v-btn>
          </template>
          <v-list>
            <v-list-item
              v-for="(preset, index) in presets"
              :key="index"
              @click="applyPreset(preset)"
            >
              <v-list-item-title>{{ preset.label }}</v-list-item-title>
            </v-list-item>
          </v-list>
        </v-menu>
      </v-col>
    </v-row>
  </div>
</template>

<style lang="scss" scoped>
.date-range {
  // Custom styles if needed
}
</style>

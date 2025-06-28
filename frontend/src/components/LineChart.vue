<script setup lang="ts">
import type {
  ChartData,
} from 'chart.js'
import {
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  TimeScale,
  Title,
  Tooltip,
} from 'chart.js'
import { Line } from 'vue-chartjs'

// Chart.js data type for line chart
type LineChartData = ChartData<'line', number[], string>

// Component props with TypeScript support
interface Props {
  chartData: LineChartData | null // Chart.js data object with labels and datasets
}

defineProps<Props>()

// Register ChartJS components for line chart functionality
ChartJS.register(
  Title,
  Tooltip,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  CategoryScale,
  TimeScale,
)

// Chart configuration options for responsive line chart
const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  scales: {
    x: {
      grid: {
        display: true,
      },
      ticks: {
        autoSkip: true,
        maxTicksLimit: 10,
      },
    },
    y: {
      beginAtZero: true,
      grid: {
        display: true,
      },
    },
  },
  plugins: {
    legend: {
      display: false,
      position: 'top' as const,
    },
    tooltip: {
      mode: 'index' as const,
      intersect: false,
    },
  },
  interaction: {
    mode: 'nearest' as const,
    axis: 'x' as const,
    intersect: false,
  },
}
</script>

<template>
  <div class="line-chart-container">
    <Line
      v-if="chartData"
      :data="chartData"
      :options="chartOptions"
    />
  </div>
</template>

<style scoped>
.line-chart-container {
  position: relative;
  width: 100%;
  height: 100%;
}
</style>
